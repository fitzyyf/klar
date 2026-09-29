function starNote(s) {
  const { scopeName, writtenPaths, pathOf } = window.K;
  const lines = [];
  const scope = scopeName();
  if (s.kind === "if" && (s.label === "for" || s.label === "while")) lines.push("这是循环。条件写在离开它的箭头上，成立才继续转。");
  else if (s.kind === "if") lines.push("这是判定。成立、否则，或某个 case，才走到后面的调用。");
  if (s.kind === "entry") lines.push("全链从这里开始：它带入口注解，或者往上找不到本仓库里调用它的方法。");
  if (s.kind === "leaf") lines.push("接口方法（Mapper、Repository 一类），没有函数体，全链在这里结束。");
  if (s.mark === "add" && s.kind === "if" && (s.label === "for" || s.label === "while")) lines.push("这次新写的循环。");
  else if (s.mark === "add" && s.kind === "if") lines.push("这次新写的判定。");
  else if (s.mark === "add") lines.push(`${scope}新出现的方法。`);
  if (s.mark === "edit") lines.push("函数体变了。去掉注释和空白后，前后哈希不同。");
  if (!s.mark && s.kind === "fn") lines.push(`${scope}没改这个方法。留在线上，是为了把全链画完。`);
  if (!writtenPaths().has(pathOf(s.id))) lines.push("这个文件不在这个会话的编辑里，按参照提交读入，别的会话还没提交的改动不会接进来。");
  if (s.unknownCalls) lines.push("函数体里有认不出的调用（下标、apply、call 或拼出来的名字），没有建边。");
  return lines.join("");
}

function branchOf(call) {
  const raw = (call.guard || "").trim();
  if (!raw || raw.includes("\u0001") || raw.includes("\u001e")) return "";
  return raw.startsWith("[") ? raw : `[${raw}]`;
}

function callNote(call) {
  const { viewingNet, writtenPaths, pathOf } = window.K;
  const net = viewingNet();
  const lines = [{
    add: net ? "会话做到最后，这次调用是新加上的。" : "这一回合加上的调用。",
    del: net ? "会话开始时还有这次调用，做到最后拆掉了。" : "这一回合拆掉的调用。回合开始时的图上还有它，所以画成虚线。",
    edit: "被调方没变，参数文本变了。",
    none: "这一跳没动。"
  }[call.change]];
  if (!writtenPaths().has(pathOf(call.from))) lines.push("调用方文件不在这个会话的编辑里，按参照提交接上，亮度低一档。");
  if (call.uncertain) lines.push("按方法名和调用对象的名字接上的，同名的不止一个，或者走的是接口，指向不一定准。");
  return lines.join("");
}

window.starNote = starNote;
window.callNote = callNote;

function callDocs(call) {
  if (call.before && (call.after || call.change === "del")) return { before: call.before, after: call.after || "" };
  if (call.after && !call.before) return { before: "", after: call.after };
  const text = call.text || call.after || call.before || "";
  return { before: text, after: text };
}

function lineDiff(before, after) {
  const a = before ? String(before).replace(/\n$/, "").split("\n") : [];
  const b = after ? String(after).replace(/\n$/, "").split("\n") : [];
  if (before === after) return b.map((line) => ({ t: " ", line }));
  if (!a.length || a.length * b.length > 250000) return b.map((line) => ({ t: "+", line }));
  const dp = Array.from({ length: a.length + 1 }, () => new Uint16Array(b.length + 1));
  for (let i = a.length - 1; i >= 0; i -= 1) {
    for (let j = b.length - 1; j >= 0; j -= 1) {
      dp[i][j] = a[i] === b[j] ? dp[i + 1][j + 1] + 1 : Math.max(dp[i + 1][j], dp[i][j + 1]);
    }
  }
  const out = [];
  let i = 0;
  let j = 0;
  while (i < a.length && j < b.length) {
    if (a[i] === b[j]) { out.push({ t: " ", line: a[i] }); i += 1; j += 1; }
    else if (dp[i + 1][j] >= dp[i][j + 1]) { out.push({ t: "-", line: a[i] }); i += 1; }
    else { out.push({ t: "+", line: b[j] }); j += 1; }
  }
  while (i < a.length) { out.push({ t: "-", line: a[i] }); i += 1; }
  while (j < b.length) { out.push({ t: "+", line: b[j] }); j += 1; }
  return out;
}

function foldDiff(rows) {
  const out = [];
  let i = 0;
  while (i < rows.length) {
    if (rows[i].t !== " ") { out.push(rows[i]); i += 1; continue; }
    let j = i;
    while (j < rows.length && rows[j].t === " ") j += 1;
    const n = j - i;
    if (n > 8) {
      out.push(rows[i], rows[i + 1], { t: "skip", line: `… ${n - 4} 行没改` }, rows[j - 2], rows[j - 1]);
    } else {
      for (let k = i; k < j; k += 1) out.push(rows[k]);
    }
    i = j;
  }
  return out;
}

function paintDiff(before, after) {
  const host = document.querySelector("#detail .diff");
  if (!host) return;
  const { esc } = window.K;
  host.innerHTML = foldDiff(lineDiff(before || "", after || "")).map((row) => {
    if (row.t === "skip") return `<p class="skip">${esc(row.line)}</p>`;
    const kind = row.t === "+" ? "add" : row.t === "-" ? "del" : "same";
    return `<div class="ln ${kind}"><span>${row.t === " " ? "" : esc(row.t)}</span><code>${esc(row.line) || " "}</code></div>`;
  }).join("");
}

function relatedChains(ids) {
  const { esc, turn, chainTitle } = window.K;
  if (!ids.length) return "";
  return `
    <h3>所在全链</h3>
    ${ids.map((cid) => {
      const chain = turn().chains[Number(cid.slice(1))];
      return `<button type="button" class="rel" data-focus-chain="${cid}">${esc(chainTitle(chain))}</button>`;
    }).join("")}
  `;
}

// 文件正文。两个地方用：右栏那个小窗，和全屏那层。编辑器起不来就退回纯文本。
// 复原不出来就说清是复原不出来；磁盘上那份一直在，「用浏览器打开」不受影响。
function openFile(host, spec) {
  host.replaceChildren();
  if (!spec || (!spec.before && !spec.after)) {
    host.innerHTML = '<p class="file-open-note note">这个文件对不上，复原不了。</p>';
    return false;
  }
  if (!window.CodeView) {
    host.innerHTML = `<pre class="src">${esc(spec.after || spec.before)}</pre>`;
    return true;
  }
  try {
    window.CodeView.open(host, spec);
    return true;
  } catch (error) {
    host.innerHTML = `<pre class="src">${esc(spec.after || spec.before)}</pre>`;
    console.error("文件正文画不出来", error);
    return true;
  }
}

// 交给系统用浏览器打开。路径由 Go 那边把关：只开仓库里的相对路径。
async function openOnDisk(path) {
  const host = document.querySelector("#file-full[hidden]") ? document.querySelector("#edits .file-open") : document.querySelector("#file-full-body");
  if (!path) return;
  try {
    const reply = await ask("open_file", { repo: result.repo, path });
    if (host) {
      const note = document.createElement("p");
      note.className = "note";
      note.textContent = reply.note;
      host.appendChild(note);
    }
  } catch (error) {
    console.error("打开文件失败", error);
  }
}

// 全屏那层。Esc 和「关闭」都走这里。
function showFile(path) {
  const layer = document.querySelector("#file-full");
  const body = document.querySelector("#file-full-body");
  const where = document.querySelector("#file-full-path");
  if (!layer || !body || !path) return;
  const file = fileOf(path);
  where.textContent = path;
  openFile(body, { path, before: file ? (file.before || "") : "", after: file ? (file.after || "") : "" });
  document.querySelector("#file-full-disk").onclick = (event) => { event.stopPropagation(); openOnDisk(path); };
  layer.hidden = false;
  body.scrollTop = 0;
}

function hideFile() {
  const layer = document.querySelector("#file-full");
  const body = document.querySelector("#file-full-body");
  if (!layer || layer.hidden) return;
  layer.hidden = true;
  if (window.CodeView) window.CodeView.close(body);
  else if (body) body.replaceChildren();
}

function fileLayerOpen() {
  const layer = document.querySelector("#file-full");
  return Boolean(layer && !layer.hidden);
}

function fileOf(path) {
  const current = session();
  const pools = [turn(), current && current.net, ...(current && current.turns || [])];
  for (const item of pools) {
    const found = item && (item.files || []).find((entry) => entry.path === path);
    if (found && (found.after || found.before)) return found;
  }
  return null;
}

function renderDetail() {
  const root = document.querySelector("#detail");
  if (window.CodeView) window.CodeView.clear();
  const { esc, turn, hasChart, starById, callByKey, labelOf, chainTitle, chainsOfFocus, chainTone, starNote, callNote, scopeName, focus, pathOf, ready } = window.K;
  const item = turn();
  const where = focus();
  if (item && ready && !ready()) {
    root.innerHTML = `<p class="eyebrow">${esc(scopeName())}</p><h2>正在复原</h2><p class="note">正在按会话记录复原改过的文件，并组装调用链。</p>`;
    return;
  }
  if (item && where.type === "file") {
    root.innerHTML = "";
    const host = document.querySelector("#edits .file-open");
    const file = fileOf(where.id);
    if (!host) return;
    // 前后两份都传，编辑器才知道哪几行是加的、哪几行是删的。传同一份就等于没 diff。
    const spec = { path: where.id, before: file ? (file.before || "") : "", after: file ? (file.after || "") : "" };
    openFile(host, spec);
    const bar = document.createElement("div");
    bar.className = "file-open-bar";
    // 复原和打开是两件事：复原是从会话记录重建那个时间点的正文，打开是磁盘上现在这份。
    // 所以不管复原成没成，这一条都在。
    for (const [label, run] of [["放大", () => showFile(where.id)], ["用浏览器打开", () => openOnDisk(where.id)]]) {
      const button = document.createElement("button");
      button.type = "button";
      button.className = "text";
      button.textContent = label;
      button.addEventListener("click", (event) => { event.stopPropagation(); run(); });
      bar.appendChild(button);
    }
    host.before(bar);
    host.scrollIntoView({ block: "nearest" });
    return;
  }
  if (!hasChart() || where.type === "none") {
    root.innerHTML = item
      ? `<p class="eyebrow">${esc(scopeName())}</p><h2>${esc(item.prompt || "没有能画成全链的改动")}</h2><p class="note">${esc(scopeName())}没有能画成全链的改动。下面列着写过的文件。</p>`
      : "";
    return;
  }

  if (where.type === "chain") {
    const chain = item.chains[Number(where.id.slice(1))];
    const hops = chain.stars.slice(0, -1).map((id, i) => {
      const call = item.calls.find((c) => c.from === id && c.to === chain.stars[i + 1]);
      if (!call) return "";
      const badge = { add: "加上", del: "拆掉", edit: "参数改了", none: "没改" }[call.change];
      return `
        <button type="button" class="hop" data-focus-edge="${esc(call.key)}" data-change="${call.change}">
          ${esc(labelOf(call.from))} → ${esc(labelOf(call.to))} · ${badge}
        </button>
      `;
    }).join("");
    root.innerHTML = `
      <p class="eyebrow">全链</p>
      <h2>${esc(chainTitle(chain))}</h2>
      <p class="badge ${chain.dashed ? "del" : "none"}">${chain.dashed ? "虚线 · 拆掉的" : chain.broken ? "实线 · 断开" : "实线 · 入口到叶子"}</p>
      <p class="note">${esc(chainTone(chain))}</p>
      ${hops ? `<h3>每一跳</h3>${hops}` : ""}
    `;
    return;
  }

  if (where.type === "edge") {
    const call = callByKey(where.id);
    if (!call) return;
    const badge = { add: "加上", del: "拆掉", edit: "参数改了", none: "没改" }[call.change];
    root.innerHTML = `
      <p class="eyebrow">调用点</p>
      <h2>${esc(labelOf(call.from))} → ${esc(labelOf(call.to))}</h2>
      <p class="badge ${call.change}">${badge}${call.uncertain ? " · 指向不确定" : ""}</p>
      ${branchOf(call) ? `<p class="note">分支 ${esc(branchOf(call))}</p>` : ""}
      <p class="key">${esc(pathOf(call.from))}</p>
      <p class="note">${esc(callNote(call))}</p>
      ${(call.before || call.after || call.text) ? `<div class="diff"></div>` : ""}
      ${relatedChains(chainsOfFocus())}
    `;
    const docs = callDocs(call);
    paintDiff(docs.before, docs.after);
    return;
  }

  const s = starById(where.id);
  if (!s) return;
  const loop = s.kind === "if" && (s.label === "for" || s.label === "while");
  const eyebrow = loop ? "循环" : { entry: "入口", leaf: "叶子", fn: "方法", if: "判定" }[s.kind] || "方法";
  const badge = s.kind === "if"
    ? (s.mark === "add" ? (loop ? "新循环" : "新判定") : s.mark === "edit" ? (loop ? "循环改了" : "判定改了") : (loop ? "循环" : "判定"))
    : s.mark === "add" ? "新方法" : s.mark === "edit" ? "方法改了" : "没改";
  const own = item.calls.filter((c) => c.from === s.id);
  const callLines = own.map((c) => `
    <button type="button" class="hop" data-focus-edge="${esc(c.key)}" data-change="${c.change}">
      ${esc(branchOf(c) ? `${branchOf(c)} → ${labelOf(c.to)}` : (c.after || c.text || c.before || labelOf(c.to)))} · ${{ add: "加上", del: "拆掉", edit: "参数改了", none: "还在" }[c.change]}
    </button>
  `).join("");
  root.innerHTML = `
    <p class="eyebrow">${eyebrow}</p>
    <h2>${esc(s.label)}</h2>
    <p class="badge ${s.mark || "none"}">${badge}</p>
    <p class="key">${esc(pathOf(s.id))}</p>
    <p class="note">${esc(starNote(s))}</p>
    ${s.body ? `<div class="diff"></div>` : ""}
    ${own.length ? `<h3>它在全链上调用的</h3>${callLines}` : ""}
    ${relatedChains(chainsOfFocus())}
  `;
  if (s.body) paintDiff(s.body.before, s.body.after);
}

window.renderDetail = renderDetail;
// 文件全屏这一层由 nebula.js 接线：关闭按钮和 Esc 都调它。
window.FileLayer = { show: showFile, hide: hideFile, isOpen: fileLayerOpen };

const W = 880;
const TOP = 72;
const ROW = 104;
const GAP = 190;
const EDGE = 150;
const SOLID = ["#8faf86", "#7fafbf", "#b49ac8", "#c9b27a"];
const CUT = "#d27a68";

const state = {
  sessionIdx: 0,
  turnId: null,
  focus: null,
  motion: "enter",
  pass: {}
};

function wantsMotion() {
  return !window.matchMedia("(prefers-reduced-motion: reduce)").matches;
}

function session() {
  return result.sessions[state.sessionIdx];
}

function turn() {
  return session().turns.find((item) => item.id === state.turnId);
}

function starById(id) {
  return turn().stars.find((item) => item.id === id);
}

function callByKey(key) {
  return turn().calls.find((item) => item.key === key);
}

function callBetween(from, to) {
  return turn().calls.find((item) => item.from === from && item.to === to);
}

function pathOf(id) {
  return id.split("#")[0];
}

function writtenPaths() {
  const paths = new Set();
  session().turns.forEach((item) => item.edits.forEach((edit) => { if (edit.ok) paths.add(edit.path); }));
  return paths;
}

function chainId(index) {
  return `c${index}`;
}

function chainColor(chain, index) {
  return chain.dashed ? CUT : SOLID[index % SOLID.length];
}

function chainTitle(chain) {
  return chain.stars.map((id) => starById(id).label).join(" → ");
}

function defaultFocus(item) {
  const call = item.calls.find((c) => c.change !== "none" && item.chains.some((ch) => onChain(ch, c)));
  if (call) return { type: "edge", id: call.key };
  const star = item.stars.find((s) => s.mark);
  if (star) return { type: "node", id: star.id };
  return { type: "chain", id: chainId(0) };
}

function onChain(chain, call) {
  const i = chain.stars.indexOf(call.from);
  return i >= 0 && chain.stars[i + 1] === call.to;
}

function chainsOfFocus() {
  const focus = state.focus;
  const chains = turn().chains;
  if (focus.type === "chain") return [focus.id];
  if (focus.type === "edge") {
    const call = callByKey(focus.id);
    return chains.map((ch, i) => (onChain(ch, call) ? chainId(i) : null)).filter(Boolean);
  }
  return chains.map((ch, i) => (ch.stars.includes(focus.id) ? chainId(i) : null)).filter(Boolean);
}

function esc(value) {
  return String(value).replace(/[&<>"]/g, (char) => ({
    "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;"
  }[char]));
}

// 分层：深度取它在各条全链里的最远位置，同层按上一层的平均横坐标排。
function layout(item) {
  const depth = new Map();
  item.chains.forEach((ch) => ch.stars.forEach((id, i) => depth.set(id, Math.max(depth.get(id) ?? 0, i))));
  for (let round = 0; round < 24; round += 1) {
    let moved = false;
    item.chains.forEach((ch) => {
      for (let i = 1; i < ch.stars.length; i += 1) {
        const need = depth.get(ch.stars[i - 1]) + 1;
        if (depth.get(ch.stars[i]) < need) { depth.set(ch.stars[i], need); moved = true; }
      }
    });
    if (!moved) break;
  }

  const preds = new Map();
  const near = new Map();
  const link = (map, a, b) => { if (!map.has(a)) map.set(a, new Set()); map.get(a).add(b); };
  item.chains.forEach((ch) => {
    for (let i = 1; i < ch.stars.length; i += 1) {
      link(preds, ch.stars[i], ch.stars[i - 1]);
      link(near, ch.stars[i], ch.stars[i - 1]);
      link(near, ch.stars[i - 1], ch.stars[i]);
    }
  });

  const layers = [];
  item.chains.forEach((ch) => ch.stars.forEach((id) => {
    const d = depth.get(id);
    layers[d] = layers[d] || [];
    if (!layers[d].includes(id)) layers[d].push(id);
  }));

  const x = new Map();
  layers.forEach((ids, d) => {
    if (d === 0) {
      ids.forEach((id, i) => x.set(id, (W * (i + 1)) / (ids.length + 1)));
      return;
    }
    const want = ids.map((id) => {
      const from = [...(preds.get(id) || [])].filter((p) => x.has(p));
      return { id, at: from.length ? from.reduce((s, p) => s + x.get(p), 0) / from.length : W / 2 };
    }).sort((a, b) => a.at - b.at);
    const mean = want.reduce((s, w) => s + w.at, 0) / want.length;
    const placed = [];
    want.forEach((w, i) => placed.push(i ? Math.max(w.at, placed[i - 1] + GAP) : w.at));
    const shift = mean - placed.reduce((s, v) => s + v, 0) / placed.length;
    want.forEach((w, i) => x.set(w.id, Math.min(W - EDGE, Math.max(EDGE, placed[i] + shift))));
  });

  const pos = new Map();
  item.stars.forEach((s) => {
    if (!depth.has(s.id)) return;
    const px = x.get(s.id);
    const around = [...(near.get(s.id) || [])].map((id) => x.get(id));
    const avg = around.length ? around.reduce((a, b) => a + b, 0) / around.length : px;
    let label = avg > px + 1 ? "left" : "right";
    if (s.kind === "entry") label = "above";
    if (s.kind === "leaf") label = "below";
    pos.set(s.id, { x: px, y: TOP + depth.get(s.id) * ROW, label });
  });
  const height = TOP + (layers.length - 1) * ROW + 70;
  return { pos, height };
}

// 同一跳被几条全链共用时，各自往两边弯一点，线不叠在一起。
function bendsOf(item) {
  const users = new Map();
  item.chains.forEach((ch, ci) => {
    for (let i = 0; i < ch.stars.length - 1; i += 1) {
      const key = `${ch.stars[i]}>${ch.stars[i + 1]}`;
      if (!users.has(key)) users.set(key, []);
      users.get(key).push(ci);
    }
  });
  return item.chains.map((ch, ci) => ch.stars.slice(0, -1).map((id, i) => {
    const list = users.get(`${id}>${ch.stars[i + 1]}`);
    return (list.indexOf(ci) - (list.length - 1) / 2) * 18;
  }));
}

function retreat(from, toward, radius) {
  const dx = toward.x - from.x;
  const dy = toward.y - from.y;
  const len = Math.hypot(dx, dy) || 1;
  return { x: from.x + (dx / len) * radius, y: from.y + (dy / len) * radius };
}

function control(prev, next, bend) {
  const dx = next.x - prev.x;
  const dy = next.y - prev.y;
  const len = Math.hypot(dx, dy) || 1;
  return { x: (prev.x + next.x) / 2 + (-dy / len) * bend, y: (prev.y + next.y) / 2 + (dx / len) * bend };
}

function filamentD(raw, bends) {
  const pts = raw.map((p) => ({ ...p }));
  if (pts.length >= 2) {
    pts[0] = retreat(raw[0], raw[1], 14);
    pts[pts.length - 1] = retreat(raw[raw.length - 1], raw[raw.length - 2], 14);
  }
  let d = `M ${pts[0].x} ${pts[0].y}`;
  for (let i = 1; i < pts.length; i += 1) {
    const c = control(raw[i - 1], raw[i], bends[i - 1] || 0);
    d += ` Q ${c.x} ${c.y} ${pts[i].x} ${pts[i].y}`;
  }
  return d;
}

function segmentMid(prev, next, bend) {
  const c = control(prev, next, bend);
  return { x: 0.25 * prev.x + 0.5 * c.x + 0.25 * next.x, y: 0.25 * prev.y + 0.5 * c.y + 0.25 * next.y };
}

function renderSession() {
  const root = document.querySelector("#sessions");
  root.innerHTML = result.sessions.map((s, i) => `
    <button type="button" data-session="${i}" aria-current="${i === state.sessionIdx}">
      ${esc(s.agent)} · ${esc(s.title)}
    </button>
  `).join("");
  const s = session();
  document.querySelector("#session-line").innerHTML =
    `参照 <code>${esc(s.baseline)}</code>　${s.bound === "late" ? "绑晚了，从记录末尾倒放" : "从会话开头绑上"}`;
}

function renderHistory() {
  const root = document.querySelector("#history");
  root.innerHTML = session().turns.map((item) => {
    const files = new Set(item.edits.filter((e) => e.ok).map((e) => e.path)).size;
    const solid = item.chains.filter((c) => !c.dashed).length;
    const dashed = item.chains.length - solid;
    const parts = [`写了 ${files} 个文件`, `实线 ${solid}`];
    if (dashed) parts.push(`虚线 ${dashed}`);
    return `
      <button type="button" data-turn="${item.id}" aria-current="${item.id === state.turnId}">
        <span class="when">${esc(item.at)}</span>
        <span class="title">${esc(item.prompt)}</span>
        <span class="meta">${esc(parts.join(" · "))}${item.errors.length ? ` · <em>${item.errors.length} 处对不上</em>` : ""}</span>
      </button>
    `;
  }).join("");

  const item = turn();
  document.querySelector("#edits").innerHTML = `
    <p class="col-name">这一回合的编辑</p>
    <p class="source" title="${esc(session().source)}">${esc(session().source)}</p>
    ${item.edits.map((e) => `
      <p class="edit${e.ok ? "" : " skipped"}">
        <span class="tool">${esc(e.tool)}</span><code>${esc(e.path)}</code>
        ${e.ok ? "" : `<span class="why">${esc(e.reason)}</span>`}
      </p>
    `).join("")}
    ${item.errors.map((e) => `
      <p class="error"><code>${esc(e.path)}</code>${esc(e.reason)}</p>
    `).join("")}
  `;
}

function renderCaption() {
  const item = turn();
  const count = (list, pred) => list.filter(pred).length;
  const edited = count(item.stars, (s) => s.mark === "edit");
  const added = count(item.stars, (s) => s.mark === "add");
  const onAny = (c) => item.chains.some((ch) => onChain(ch, c));
  const addCalls = count(item.calls, (c) => c.change === "add" && onAny(c));
  const delCalls = count(item.calls, (c) => c.change === "del" && onAny(c));
  const editCalls = count(item.calls, (c) => c.change === "edit" && onAny(c));
  const solid = count(item.chains, (c) => !c.dashed);
  const dashed = item.chains.length - solid;

  const methods = [edited && `改了 ${edited} 个方法`, added && `新出现 ${added} 个方法`].filter(Boolean).join("，");
  const calls = [addCalls && `加上 ${addCalls} 跳`, delCalls && `拆掉 ${delCalls} 跳`, editCalls && `${editCalls} 跳参数改了`].filter(Boolean);
  const text = [
    methods ? `这一回合${methods}。` : "",
    calls.length ? `调用：${calls.join("，")}。` : "调用没变，标在星上。",
    `经过它们的全链：实线 ${solid} 条${dashed ? `，虚线 ${dashed} 条（回合开始时还在，这回合拆掉）` : ""}。`,
    item.omitted ? `还有 ${item.omitted} 条没画出。` : "",
    item.errors.length ? `${item.errors.length} 个文件对不上，链在那里断开。` : ""
  ].join("");
  document.querySelector("#caption").textContent = text;
  document.querySelector("#said").textContent = item.prompt;
  document.querySelector("#reply").textContent = item.reply || "这一回合没有文字回复。";
}

function renderChart() {
  const svg = document.querySelector("#chart");
  const item = turn();
  const { pos, height } = layout(item);
  const bends = bendsOf(item);
  const lit = new Set(chainsOfFocus());
  const motion = wantsMotion() ? state.motion : "";
  const parts = [];
  svg.setAttribute("viewBox", `0 0 ${W} ${height}`);
  svg.setAttribute("class", motion);

  item.chains.forEach((chain, ci) => {
    const pts = chain.stars.map((id) => pos.get(id));
    const d = filamentD(pts, bends[ci]);
    const hot = lit.has(chainId(ci)) ? " hot" : "";
    const cut = chain.dashed ? " cut" : "";
    const measurable = hot && !chain.dashed && motion === "enter" ? ' pathLength="1"' : "";
    parts.push(`<path class="filament${hot}${cut}" d="${d}" stroke="${chainColor(chain, ci)}"${measurable}></path>`);
    parts.push(`<path class="hit" data-chain="${chainId(ci)}" d="${d}"></path>`);
  });

  const pins = new Map();
  item.chains.forEach((chain, ci) => {
    for (let i = 0; i < chain.stars.length - 1; i += 1) {
      const call = callBetween(chain.stars[i], chain.stars[i + 1]);
      if (!call) continue;
      const a = pos.get(call.from);
      const b = pos.get(call.to);
      const d = filamentD([a, b], [bends[ci][i]]);
      if (call.change !== "none") {
        const color = call.change === "del" ? CUT : "#e0b15a";
        parts.push(`<path class="change-seg" d="${d}" stroke="${color}" pathLength="1"></path>`);
      }
      parts.push(`<path class="hit" data-edge="${esc(call.key)}" d="${d}"></path>`);
      if (call.change !== "none" || call.uncertain) {
        if (!pins.has(call.key)) pins.set(call.key, { call, mids: [] });
        pins.get(call.key).mids.push(segmentMid(a, b, bends[ci][i]));
      }
    }
    if (chain.broken) {
      const end = pos.get(chain.stars[chain.stars.length - 1]);
      parts.push(`<g class="broken" transform="translate(${end.x} ${end.y + 30})"><line x1="-8" y1="-6" x2="8" y2="6"></line><line x1="-8" y1="2" x2="8" y2="14"></line><text x="14" y="8">断在这里</text></g>`);
    }
  });

  item.stars.forEach((s) => {
    const p = pos.get(s.id);
    if (!p) return;
    const marked = s.mark ? " is-marked" : "";
    const focused = state.focus.type === "node" && state.focus.id === s.id;
    const pulse = s.mark && (motion === "enter" || (motion === "move" && focused)) ? " pulse" : "";
    const pin = s.mark === "add" ? "＋" : s.mark === "edit" ? "◆" : "";
    const pinAt = s.kind === "entry" ? 'x="14" y="-10" text-anchor="start"' : 'x="0" y="-16" text-anchor="middle"';
    parts.push(`
      <g class="star is-${s.kind}${marked}${pulse}" data-node="${esc(s.id)}" transform="translate(${p.x} ${p.y})">
        <circle class="halo" r="16"></circle>
        ${s.kind === "leaf"
          ? '<rect class="core" x="-5" y="-5" width="10" height="10"></rect>'
          : '<circle class="core" r="5"></circle>'}
        ${pin ? `<text class="pin-node" ${pinAt}>${pin}</text>` : ""}
        <text ${labelAttrs(p.label)}>${esc(s.label)}</text>
      </g>
    `);
  });

  pins.forEach(({ call, mids }) => {
    const x = mids.reduce((sum, m) => sum + m.x, 0) / mids.length;
    const y = mids.reduce((sum, m) => sum + m.y, 0) / mids.length;
    const hot = state.focus.type === "edge" && state.focus.id === call.key;
    const pulse = call.change !== "none" && (motion === "enter" || (motion === "move" && hot)) ? " pulse" : "";
    const glyph = { add: "＋", del: "✕", edit: "±", none: "?" }[call.change];
    const tone = call.change === "none" ? "unsure" : call.change;
    parts.push(`
      <g class="edge-pin is-${tone}${hot ? " is-hot" : ""}${pulse}" data-edge="${esc(call.key)}" transform="translate(${x} ${y})">
        <circle r="${hot ? 11 : call.change === "none" ? 7 : 9}"></circle>
        <text>${glyph}</text>
      </g>
    `);
  });

  svg.innerHTML = parts.join("");
}

function labelAttrs(pos) {
  if (pos === "above") return 'x="0" y="-22" text-anchor="middle"';
  if (pos === "below") return 'x="0" y="28" text-anchor="middle"';
  if (pos === "left") return 'x="-16" y="4" text-anchor="end"';
  return 'x="16" y="4" text-anchor="start"';
}

function chainTone(chain) {
  if (chain.broken) return `到 ${chain.broken} 对不上，断在这里，没走到叶子。`;
  if (chain.dashed) return "回合开始时还在。这一回合拆掉了其中一跳。";
  return "经过这一回合的改动，从入口走到叶子。";
}

function renderChains() {
  const root = document.querySelector("#chains");
  const lit = new Set(chainsOfFocus());
  root.innerHTML = turn().chains.map((chain, ci) => `
    <button type="button" data-chain="${chainId(ci)}" aria-current="${lit.has(chainId(ci))}" style="border-left-color:${chainColor(chain, ci)}">
      <span class="short">${esc(chainTitle(chain))}</span>
      <span class="tone">${esc(chainTone(chain))}</span>
    </button>
  `).join("");
}

function starNote(s) {
  const lines = [];
  const outside = s.kind !== "leaf" && !writtenPaths().has(pathOf(s.id));
  if (s.kind === "entry") lines.push("全链从这里开始。往上找不到本仓库里调用它的方法。");
  if (s.kind === "leaf") lines.push("调用落到仓库外面，全链在这里结束。");
  if (s.mark === "add") lines.push("这一回合新出现的方法。");
  if (s.mark === "edit") lines.push("函数体变了。去掉注释和空白后，前后哈希不同。");
  if (!s.mark && s.kind === "fn") lines.push("这一回合没改。留在线上，是为了把全链画完。");
  if (outside) lines.push("这个文件不在这个会话的编辑里，按当前源码读入，只用来接上调用，不高亮。");
  if (s.unknownCalls) lines.push("函数体里有认不出的调用（下标、apply、call 或拼出来的名字），没有建边。");
  return lines.join("");
}

function callNote(call) {
  const lines = [{
    add: "这一回合加上的调用。",
    del: "这一回合拆掉的调用。回合开始时的图上还有它，所以画成虚线。",
    edit: "被调方没变，参数文本变了。",
    none: "这一跳没动。"
  }[call.change]];
  if (!writtenPaths().has(pathOf(call.from)) && starById(call.from).kind !== "leaf") {
    lines.push("调用方文件不在这个会话的编辑里，是读当前源码接上的，亮度低一档。");
  }
  if (call.uncertain) lines.push("被调方的类型来自参数或构造参数，指向不一定唯一。");
  return lines.join("");
}

function block(name, items, tone) {
  if (!items || !items.length) return "";
  return `<h3>${name}</h3>${items.map((text) => `<pre class="${tone}">${esc(text)}</pre>`).join("")}`;
}

function relatedChains(ids) {
  return `
    <h3>所在全链</h3>
    ${ids.map((cid) => {
      const chain = turn().chains[Number(cid.slice(1))];
      return `<button type="button" class="rel" data-focus-chain="${cid}">${esc(chainTitle(chain))}</button>`;
    }).join("")}
  `;
}

function renderDetail() {
  const root = document.querySelector("#detail");
  const focus = state.focus;
  const item = turn();

  if (focus.type === "chain") {
    const chain = item.chains[Number(focus.id.slice(1))];
    const hops = chain.stars.slice(0, -1).map((id, i) => {
      const call = callBetween(id, chain.stars[i + 1]);
      const badge = { add: "加上", del: "拆掉", edit: "参数改了", none: "没改" }[call.change];
      return `
        <button type="button" class="hop" data-focus-edge="${esc(call.key)}" data-change="${call.change}">
          ${esc(starById(call.from).label)} → ${esc(starById(call.to).label)} · ${badge}
        </button>
      `;
    }).join("");
    root.innerHTML = `
      <p class="eyebrow">全链</p>
      <h2>${esc(chainTitle(chain))}</h2>
      <p class="badge ${chain.dashed ? "del" : "none"}">${chain.dashed ? "虚线 · 这回合拆掉" : chain.broken ? "实线 · 断开" : "实线 · 入口到叶子"}</p>
      <p class="note">${esc(chainTone(chain))}</p>
      <h3>每一跳</h3>
      ${hops}
    `;
    return;
  }

  if (focus.type === "edge") {
    const call = callByKey(focus.id);
    const badge = { add: "这回合加上", del: "这回合拆掉", edit: "参数改了", none: "没改" }[call.change];
    root.innerHTML = `
      <p class="eyebrow">调用点</p>
      <h2>${esc(starById(call.from).label)} → ${esc(starById(call.to).label)}</h2>
      <p class="badge ${call.change}">${badge}${call.uncertain ? " · 指向不确定" : ""}</p>
      <p class="key">${esc(call.key)}</p>
      <p class="note">${esc(callNote(call))}</p>
      ${block("删掉", call.before && [call.before], "del")}
      ${block("加上", call.after && [call.after], "add")}
      ${block("还在", call.text && [call.text], "keep")}
      ${relatedChains(chainsOfFocus())}
    `;
    return;
  }

  const s = starById(focus.id);
  const eyebrow = { entry: "入口", leaf: "叶子", fn: "方法" }[s.kind];
  const badge = s.mark === "add" ? "新方法" : s.mark === "edit" ? "方法改了" : "没改";
  const own = item.calls.filter((c) => c.from === s.id);
  const callLines = own.map((c) => `
    <button type="button" class="hop" data-focus-edge="${esc(c.key)}" data-change="${c.change}">
      ${esc(c.after || c.text || c.before)} · ${{ add: "加上", del: "拆掉", edit: "参数改了", none: "还在" }[c.change]}
    </button>
  `).join("");
  root.innerHTML = `
    <p class="eyebrow">${eyebrow}</p>
    <h2>${esc(s.label)}</h2>
    <p class="badge ${s.mark || "none"}">${badge}</p>
    <p class="key">${esc(s.id)}</p>
    <p class="note">${esc(starNote(s))}</p>
    ${block("函数体里加上", s.body && s.body.added, "add")}
    ${block("函数体里删掉", s.body && s.body.removed, "del")}
    ${own.length ? `<h3>它调用的</h3>${callLines}` : ""}
    ${relatedChains(chainsOfFocus())}
  `;
}

function handoffText(verdict, opinion) {
  const s = session();
  const end = /[。！？.!?]$/.test(opinion) ? "" : "。";
  if (verdict === "back") return `评审没通过：${opinion}${end}参照 ${s.baseline}。改完后停下，等下一次评审。`;
  return `评审通过。请在这个会话里提交。参照 ${s.baseline}。提交说明：${s.title}。只提交这个会话写过的文件。`;
}

function passOf() {
  const s = session();
  if (!state.pass[s.sid]) state.pass[s.sid] = { done: null, tries: 0, note: "", handoff: "" };
  return state.pass[s.sid];
}

function renderPass() {
  const s = session();
  const pass = passOf();
  const button = document.querySelector("#pass");
  const back = document.querySelector("#back");
  document.querySelector("#pass-scope").textContent =
    `评审整个会话：${s.turns.length} 个回合，写过 ${writtenPaths().size} 个文件。话由你粘给 agent。`;
  button.disabled = Boolean(pass.done);
  back.disabled = Boolean(pass.done);
  document.querySelector("#back-text").disabled = Boolean(pass.done);
  button.textContent = pass.done === "pass" ? "已记下通过" : "记下通过";
  const box = document.querySelector("#handoff");
  box.hidden = !pass.handoff;
  document.querySelector("#handoff-text").value = pass.handoff;
  document.querySelector("#pass-note").textContent = pass.note;
}

function send(verdict) {
  const pass = passOf();
  if (pass.done) return;
  const opinion = document.querySelector("#back-text").value.trim();
  if (verdict === "back" && !opinion) {
    pass.note = "先写一句退回意见。";
  } else {
    pass.done = verdict;
    pass.handoff = handoffText(verdict, opinion);
    pass.note = "结论记在本机了。复制下面这句话，粘给 agent。";
  }
  renderPass();
}

function selectTurn(id) {
  state.turnId = id;
  state.focus = defaultFocus(turn());
  state.motion = "enter";
}

function render() {
  renderSession();
  renderHistory();
  renderCaption();
  renderChart();
  renderChains();
  renderDetail();
  renderPass();
}

document.querySelector("#pass").addEventListener("click", (event) => {
  event.stopPropagation();
  send("pass");
});

document.querySelector("#back").addEventListener("click", (event) => {
  event.stopPropagation();
  send("back");
});

document.querySelector("#handoff-copy").addEventListener("click", (event) => {
  event.stopPropagation();
  const pass = passOf();
  if (!pass.handoff) return;
  navigator.clipboard.writeText(pass.handoff);
  document.querySelector("#pass-note").textContent = "话已经复制，粘给这个会话的 agent。";
});

document.querySelector("#sessions").addEventListener("click", (event) => {
  const button = event.target.closest("[data-session]");
  if (!button) return;
  state.sessionIdx = Number(button.dataset.session);
  selectTurn(session().turns[0].id);
  render();
});

document.querySelector(".stage").addEventListener("click", (event) => {
  const turnButton = event.target.closest("[data-turn]");
  if (turnButton) {
    selectTurn(turnButton.dataset.turn);
    render();
    return;
  }
  const pick = [
    ["[data-focus-edge]", "edge", "focusEdge"],
    ["[data-focus-chain]", "chain", "focusChain"],
    ["[data-edge]", "edge", "edge"],
    ["[data-node]", "node", "node"],
    ["[data-chain]", "chain", "chain"]
  ];
  for (const [selector, type, key] of pick) {
    const hit = event.target.closest(selector);
    if (!hit) continue;
    state.focus = { type, id: hit.dataset[key] };
    state.motion = "move";
    render();
    return;
  }
});

selectTurn(session().turns[0].id);
render();

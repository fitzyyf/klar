const SOLID = ["var(--c0)", "var(--c1)", "var(--c2)", "var(--c3)"];
const CUT = "var(--cut)";
const AGENT_STATE = { idle: "空闲", busy: "在跑", waiting: "等权限", exited: "已退出" };
const REVIEW = { pass: "已记通过", back: "已记退回" };
async function ask(name, body) {
  const path = { load_repo: "/api/load", open_session: "/api/session" }[name] || "/api/verdict";
  const res = await fetch(path, {
    method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify(body || {})
  });
  if (!res.ok) throw new Error((await res.text()) || res.status);
  return res.json();
}
let result = { repo: "", repos: [], sessions: [], notes: [] };
const state = {
  sessionIdx: 0,
  turnId: "net",
  focus: { type: "none" },
  pass: {},
  loading: false,
  opening: "",
  openError: "",
  workspace: "chain"
};
function session() {
  return result.sessions[state.sessionIdx];
}
function viewingNet() {
  return state.turnId === "net";
}

function scopeName() {
  return viewingNet() ? "这个会话" : "这一回合";
}

function turn() {
  const s = session();
  if (!s) return null;
  if (viewingNet()) return s.net || null;
  return s.turns.find((item) => item.id === state.turnId) || null;
}

function hasChart() {
  const item = turn();
  return Boolean(item && item.chains.length);
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
  const s = session();
  if (!s) return paths;
  s.turns.forEach((item) => item.edits.forEach((edit) => { if (edit.ok) paths.add(edit.path); }));
  return paths;
}

function chainId(index) {
  return `c${index}`;
}

function chainColor(chain, index) {
  return chain.dashed ? CUT : SOLID[index % SOLID.length];
}

function labelOf(id) {
  const s = starById(id);
  return s ? s.label : id;
}

function chainTitle(chain) {
  return chain.stars.map(labelOf).join(" → ");
}

function defaultFocus(item) {
  const call = item.calls.find((c) => c.change !== "none" && item.chains.some((ch) => onChain(ch, c)));
  if (call) return { type: "edge", id: call.key };
  const star = item.stars.find((s) => s.mark);
  if (star) return { type: "node", id: star.id };
  if (item.chains.length) return { type: "chain", id: chainId(0) };
  return { type: "none" };
}

function onChain(chain, call) {
  const i = chain.stars.indexOf(call.from);
  return i >= 0 && chain.stars[i + 1] === call.to;
}

function chainsOfFocus() {
  const focus = state.focus;
  if (!hasChart() || focus.type === "none") return [];
  const chains = turn().chains;
  if (focus.type === "chain") return [focus.id];
  if (focus.type === "edge") {
    const call = callByKey(focus.id);
    if (!call) return [];
    return chains.map((ch, i) => (onChain(ch, call) ? chainId(i) : null)).filter(Boolean);
  }
  return chains.map((ch, i) => (ch.stars.includes(focus.id) ? chainId(i) : null)).filter(Boolean);
}

function esc(value) {
  return String(value).replace(/[&<>"]/g, (char) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" }[char]));
}

function reviewOf(s) {
  const done = (state.pass[s.sid] && state.pass[s.sid].done) || (s.review && s.review.current && s.review.verdict);
  return { text: REVIEW[done] || "未评审", cls: done ? `is-${done}` : "is-none" };
}

function sessionSum(s) {
  const files = filesOf(s);
  if (files.length) return files.join("、");
  const reply = s.turns[0] && s.turns[0].reply;
  return reply || "没有改过文件";
}

function sessionBlurb(s) {
  const first = s.turns[s.turns.length - 1];
  return s.title || (first && first.prompt) || "没有标题";
}
const filesOf = (s) => [...new Set(s.turns.flatMap((item) => item.edits.filter((edit) => edit.ok && edit.path).map((edit) => edit.path)))];
function metaOf(s) {
  const when = (s.turns[0] && s.turns[0].at) || "";
  return [s.agent, when].filter(Boolean).join(" · ");
}
function renderSession() {
  const where = document.querySelector("#project-path");
  if (where) { where.textContent = result.repo ? result.repo.replace(/^\/Users\/[^/]+/, "~") : "还没选项目"; where.title = result.repo || ""; }
  const banner = document.querySelector("#banner");
  const notes = [...result.notes];
  if (!result.sessions.length && result.repo) notes.push("这个仓库里没有 Claude、Codex 或 Grok 的会话。");
  banner.hidden = !notes.length && !state.loading;
  banner.textContent = state.loading ? "正在列出会话……" : notes.join(" ");

  const root = document.querySelector("#sessions");
  root.innerHTML = result.sessions.length ? result.sessions.map((s, i) => {
    const review = reviewOf(s);
    return `
      <div class="session" data-session="${i}" aria-current="${i === state.sessionIdx}">
        <button type="button">
          <span class="title">${esc(sessionBlurb(s))}</span>
          <span class="review ${review.cls}">${esc(review.text)}</span>
          <span class="meta">${esc(metaOf(s))}</span>
          <span class="sum">${esc(sessionSum(s))}</span>
        </button>
      </div>
    `;
  }).join("") : '<p class="empty">没有会话。</p>';

  const s = session();
  document.querySelector("#session-title").textContent = s ? `${s.agent} · ${sessionBlurb(s)}` : "";
  const review = document.querySelector("#session-review");
  const mark = s ? reviewOf(s) : { text: "", cls: "" };
  review.textContent = mark.text;
  review.className = mark.cls;
  document.querySelector("#session-line").innerHTML = s
    ? `参照 <code>${esc(s.baseline)}</code>　${s.bound === "late" ? "改前正文从磁盘倒放" : "改前正文取自会话记录"}`
    : "";
}

function renderEdits() {
  const edits = document.querySelector("#edits");
  const s = session();
  const item = turn();
  if (!s || !item) {
    edits.innerHTML = "";
    return;
  }
  if (!s.turns.length && viewingNet()) {
    edits.innerHTML = `<p class="col-name">会话记录</p><p class="source">${esc(s.source)}</p><p class="empty">这个会话还没有写过仓库里的文件。</p>`;
    return;
  }
  edits.innerHTML = `
    <p class="col-name">${esc(scopeName())}写过的文件</p>
    <p class="source" title="${esc(s.source)}">${esc(s.source)}</p>
    ${(() => { const opened = new Set(); return item.edits.map((e) => { const open = e.ok && state.focus.type === "file" && state.focus.id === e.path && !opened.has(e.path); if (open) opened.add(e.path); return `<button type="button" class="edit${e.ok ? "" : " skipped"}" ${e.ok ? `data-file="${esc(e.path)}"` : "disabled"} aria-current="${open}"><span class="tool">${esc(e.tool)}</span><code>${esc(e.path)}</code>${e.ok ? "" : `<span class="why">${esc(e.reason || "")}</span>`}</button>${open ? `<div class="diff file-open"></div>` : ""}`; }).join(""); })()}
    ${item.errors.map((e) => `<p class="error"><code>${esc(e.path)}</code>${esc(e.reason)}</p>`).join("")}
  `;
}

function renderCaption() {
  const item = turn();
  const said = document.querySelector("#said");
  const reply = document.querySelector("#reply");
  const caption = document.querySelector("#caption");
  const s = session();
  if (!item || !s) {
    said.textContent = "";
    reply.textContent = "";
    caption.textContent = state.loading ? "" : "选左边一个会话。";
    return;
  }
  document.querySelector("#said-label").textContent = viewingNet() ? "第一句" : "你说的";
  document.querySelector("#reply-label").textContent = viewingNet() ? "最近回复" : "它说的";
  if (viewingNet()) {
    const first = s.turns[s.turns.length - 1];
    const last = s.turns[0];
    said.textContent = (first && first.prompt) || s.title || "";
    reply.textContent = (last && last.reply) || "没有收成一句回复。点上面的回合看它当时怎么说。";
  } else {
    said.textContent = item.prompt;
    reply.textContent = item.reply || "这一回合没有文字回复。";
  }
  if (!s.assembled) {
    caption.textContent = state.openError || "正在按会话记录复原改过的文件，并组装调用链。";
    return;
  }
  if (!item.chains.length) {
    caption.textContent = item.errors.length
      ? `${item.errors.length} 个文件对不上，没有编链。`
      : `${scopeName()}没有改到方法，只动了模板、样式、配置或文档。`;
    return;
  }
  const n = (list, pred) => list.filter(pred).length;
  const onAny = (c) => item.chains.some((ch) => onChain(ch, c));
  const edited = n(item.stars, (star) => star.mark === "edit");
  const added = n(item.stars, (star) => star.mark === "add");
  const hop = (change) => n(item.calls, (c) => c.change === change && onAny(c));
  const solid = n(item.chains, (c) => !c.dashed);
  const dashed = item.chains.length - solid;
  const methods = [edited && `改了 ${edited} 个方法`, added && `新出现 ${added} 个方法`].filter(Boolean).join("，");
  const calls = [hop("add") && `加上 ${hop("add")} 跳`, hop("del") && `拆掉 ${hop("del")} 跳`, hop("edit") && `${hop("edit")} 跳参数改了`].filter(Boolean);
  caption.textContent = [
    viewingNet() ? "按每个文件第一次改之前、最后一次改成功之后比较。" : "",
    methods ? `${scopeName()}${methods}。` : "",
    calls.length ? `调用：${calls.join("，")}。` : "调用没变。",
    `经过它们的全链：实线 ${solid} 条${dashed ? `，虚线 ${dashed} 条。` : "。"}`,
    item.omitted ? `还有 ${item.omitted} 条没画出。` : "",
    item.errors.length ? `${item.errors.length} 个文件对不上，链在那里断开。` : ""
  ].join("");
}
function chainTone(chain) {
  if (chain.broken) return `到 ${chain.broken} 对不上，断在这里，没走到叶子。`;
  if (!chain.dashed) return `经过${scopeName()}的改动，从入口走到叶子。`;
  return viewingNet() ? "会话开始时还在。做到最后，其中一跳被拆掉了。" : "回合开始时还在。这一回合拆掉了其中一跳。";
}

function renderChains() {
  const root = document.querySelector("#chains");
  const item = turn();
  const s = session();
  if (!s || !item || !item.chains.length) {
    root.innerHTML = s && item ? `<p class="omitted">${s.assembled ? "这次没有编出全链。" : "正在组装。"}</p>` : "";
    return;
  }
  const lit = new Set(chainsOfFocus());
  root.innerHTML = turn().chains.map((chain, ci) => `
    <button type="button" data-chain="${chainId(ci)}" aria-current="${lit.has(chainId(ci))}" style="border-left-color:${chainColor(chain, ci)}">
      <span class="short">${esc(chainTitle(chain))}</span>
      <span class="tone">${esc(chainTone(chain))}</span>
    </button>
  `).join("") + (turn().omitted ? `<p class="omitted">还有 ${turn().omitted} 条全链没画出（每个入口最多 20 条，每次最多 24 条）。</p>` : "");
}

function litSets() {
  const nodes = new Set();
  const edges = new Set();
  if (!hasChart() || state.focus.type === "none") return { nodes, edges };
  const lit = new Set(chainsOfFocus());
  turn().chains.forEach((chain, index) => {
    if (!lit.has(chainId(index))) return;
    chain.stars.forEach((id) => nodes.add(id));
    for (let i = 0; i < chain.stars.length - 1; i += 1) {
      const call = callBetween(chain.stars[i], chain.stars[i + 1]);
      if (call) edges.add(call.key);
    }
  });
  return { nodes, edges };
}

function renderChart() {
  const host = document.querySelector("#chart");
  const empty = document.querySelector("#sky-empty");
  const item = turn();
  const show = hasChart();
  host.classList.toggle("is-off", !show);
  empty.hidden = show;
  if (!show) {
    window.Chain.clear();
    empty.textContent = !item ? "" : !session().assembled
      ? (state.openError || "正在组装调用链和改过的文件……")
      : `${scopeName()}没有经过改动的全链。`;
    return;
  }
  const { nodes, edges } = litSets();
  try {
    const ok = window.Chain.render(host, item, {
      focus: state.focus,
      nodes,
      edges,
      onPick(type, id) {
        state.focus = { type, id };
        render();
      }
    });
    if (!ok) { host.classList.add("is-off"); empty.hidden = false; empty.textContent = "调用链组件没载入。"; }
  } catch (error) {
    window.Chain.clear();
    host.classList.add("is-off"); empty.hidden = false; empty.textContent = "这张调用链排不开。";
    console.error("调用链布局失败", error);
  }
}

function passOf() {
  const s = session();
  if (!state.pass[s.sid]) state.pass[s.sid] = { done: null, note: "", busy: false, handoff: "" };
  return state.pass[s.sid];
}

function renderHandoff(pass) {
  const box = document.querySelector("#handoff");
  const area = document.querySelector("#handoff-text");
  if (!box || !area) return;
  const show = Boolean(pass && pass.handoff);
  box.hidden = !show;
  if (show && area.value !== pass.handoff) area.value = pass.handoff;
}

function renderPass() {
  const s = session();
  const button = document.querySelector("#pass");
  const back = document.querySelector("#back");
  const text = document.querySelector("#back-text");
  if (!s) {
    [button, back, text].forEach((el) => { el.disabled = true; });
    document.querySelector("#pass-scope").textContent = "";
    document.querySelector("#pass-note").textContent = "";
    renderHandoff(null);
    return;
  }
  const pass = passOf();
  const confirmed = pass.done === "pass" || (s.review && s.review.current && s.review.verdict === "pass");
  document.querySelector("#pass-scope").textContent =
    `这个会话：${s.turns.length} 个回合，写过 ${writtenPaths().size} 个文件。话由你粘给 agent。`;
  const locked = Boolean(pass.done) || pass.busy || !s.turns.length;
  back.disabled = locked;
  text.disabled = locked;
  button.disabled = locked || confirmed;
  button.textContent = pass.busy ? "正在记下……" : (confirmed ? "已记下通过" : "记下通过");
  document.querySelector("#pass-note").textContent = pass.note;
  renderHandoff(pass);
}

async function copyHandoff() {
  const pass = passOf();
  const area = document.querySelector("#handoff-text");
  const note = document.querySelector("#pass-note");
  if (!pass.handoff) return;
  area.value = pass.handoff;
  area.select();
  try {
    if (navigator.clipboard && navigator.clipboard.writeText) {
      await navigator.clipboard.writeText(pass.handoff);
    } else {
      document.execCommand("copy");
    }
    note.textContent = "话已经复制，粘给这个会话的 agent。";
  } catch (error) {
    note.textContent = "复制不了，话已经选中了，手动按 ⌘C。";
  }
}

async function send(verdict) {
  const s = session();
  if (!s) return;
  const pass = passOf();
  if (pass.done || pass.busy || (verdict === "pass" && s.review && s.review.current && s.review.verdict === "pass")) return;
  const opinion = document.querySelector("#back-text").value.trim();
  if (verdict === "back" && !opinion) {
    pass.note = "先写下退回意见。";
    renderPass();
    return;
  }
  pass.busy = true;
  renderPass();
  try {
    const tip = (s.turns[0] && s.turns[0].id) || "";
    const reply = await ask("send_verdict", { repo: result.repo, sid: s.sid, verdict, opinion, title: s.title, turnId: tip });
    pass.note = reply.note;
    pass.handoff = reply.text || "";
    if (reply.ok) pass.done = verdict;
    else s.review = Object.assign(s.review || {}, { verdict, current: true, text: reply.note });
  } catch (error) {
    pass.note = `没记下：${error}。可以再点一次。`;
  }
  pass.busy = false;
  render();
}

function selectTurn(id) {
  state.turnId = id || "net";
  const item = turn();
  state.focus = item ? defaultFocus(item) : { type: "none" };
}

function turnShort(text) {
  const chars = [...String(text || "")];
  return chars.length > 18 ? `${chars.slice(0, 17).join("")}…` : chars.join("");
}

function renderTurnBar() {
  const bar = document.querySelector("#turn-bar");
  if (!bar) return;
  const s = session();
  if (!s || !s.turns.length) {
    bar.innerHTML = "";
    bar.hidden = true;
    return;
  }
  bar.hidden = false;
  const pills = [`<button type="button" class="turn-pill" data-turn-id="net" aria-current="${viewingNet()}">净改</button>`];
  s.turns.forEach((item) => {
    pills.push(`
      <button type="button" class="turn-pill" data-turn-id="${esc(item.id)}" title="${esc(item.prompt || "")}" aria-current="${state.turnId === item.id}">
        <span class="t-when">${esc(item.at || "")}</span>${esc(turnShort(item.prompt))}
      </button>
    `);
  });
  bar.innerHTML = pills.join("");
}

function render() {
  window.K = {
    esc, turn, hasChart, starById, callByKey, labelOf, chainTitle, chainsOfFocus, chainTone, scopeName, viewingNet, pathOf, writtenPaths,
    starNote: window.starNote, callNote: window.callNote,
    focus() { return state.focus; },
    ready() { const s = session(); return Boolean(s && s.assembled); },
    session, workspace() { return state.workspace; },
    setWorkspace(id) { state.workspace = id; render(); },
    openFile(_turnId, path) { state.focus = { type: "file", id: path }; render(); }
  };
  document.querySelector("#pane-sky").dataset.workspace = state.workspace;
  if (window.renderTalk) window.renderTalk();
  renderSession();
  renderTurnBar();
  renderEdits();
  renderCaption();
  renderChart();
  renderChains();
  window.renderDetail();
  renderPass();
  if (window.Guide) window.Guide.render(result.projects || [], result.repo);
}

async function load(repo) {
  const keepSid = session() && session().sid;
  const keepTurn = state.turnId;
  state.loading = true;
  renderSession();
  try {
    result = await ask("load_repo", { repo: repo || null });
  } catch (error) {
    result = { repo: repo || "", repos: result.repos || [], projects: result.projects || [], sessions: [], notes: [`读不出来：${error}`] };
  }
  state.loading = false;
  const idx = result.sessions.findIndex((s) => s.sid === keepSid);
  const sameSession = idx >= 0;
  state.sessionIdx = sameSession ? idx : 0;
  const turns = session() ? session().turns : [];
  const turnStill = keepTurn === "net" || (sameSession && turns.some((t) => t.id === keepTurn));
  const focus = state.focus;
  if (!turnStill || !session()) selectTurn("net");
  else {
    state.turnId = keepTurn || "net";
    const alive = (focus.type === "edge" && callByKey(focus.id)) || (focus.type === "node" && starById(focus.id)) || (focus.type === "chain" && turn().chains[Number(focus.id.slice(1))]);
    state.focus = alive ? focus : defaultFocus(turn());
  }
  applySaved();
  render();
  if (repo === undefined && window.Guide && !window.Guide.remembered()) window.Guide.open();
  openSession();
}

let openTicket = 0;
async function openSession() {
  const s = session();
  if (!s || s.assembled) return;
  const ticket = ++openTicket; const sid = s.sid;
  state.opening = sid; state.openError = "";
  render();
  try {
    const full = await ask("open_session", { repo: result.repo, sid, agent: s.agent });
    if (ticket !== openTicket) return;
    const idx = result.sessions.findIndex((item) => item.sid === sid);
    if (idx >= 0) result.sessions[idx] = full;
    if (session() && session().sid === sid) selectTurn(state.turnId);
  } catch (error) {
    if (ticket !== openTicket) return;
    state.openError = `组装失败：${error.message || error}`;
  }
  state.opening = "";
  render();
}
function applySaved() {
  for (const s of result.sessions || []) {
    if (state.pass[s.sid] && state.pass[s.sid].busy) continue;
    const rev = s.review;
    if (!rev) continue;
    const note = rev.current ? rev.text : (rev.verdict === "pass" ? "上次记了通过，之后又有新回合。" : "上次记了退回，之后又有新回合。");
    state.pass[s.sid] = { done: rev.current ? rev.verdict : null, note, busy: false, handoff: rev.current ? rev.text : "" };
  }
}

// 点哪儿都把两边对上：画布滚到那儿，右栏的全链清单滚到含它的那一条。
function followFocus() {
  const chain = window.Chain;
  if (!chain || !state.focus || state.focus.type === "none" || state.focus.type === "file") return;
  const item = turn();
  if (!item || !item.chains.length) return;
  let ids = [];
  let index = -1;
  if (state.focus.type === "chain") {
    index = Number(String(state.focus.id).replace(/^c/, ""));
    const hit = item.chains[index];
    if (!hit) return;
    ids = hit.stars;
  } else if (state.focus.type === "node") {
    ids = [state.focus.id];
  } else {
    const call = item.calls.find((one) => one.key === state.focus.id);
    if (!call) return;
    ids = [call.from, call.to];
  }
  chain.reveal(ids);
  if (index < 0) {
    index = item.chains.findIndex((one) => ids.every((id) => one.stars.includes(id)));
  }
  if (index < 0) return;
  const row = document.querySelector(`#chains [data-chain="${chainId(index)}"]`);
  if (row) row.scrollIntoView({ block: "nearest" });
}

document.querySelector("#pass").addEventListener("click", (event) => { event.stopPropagation(); send("pass"); });
document.querySelector("#back").addEventListener("click", (event) => { event.stopPropagation(); send("back"); });
document.querySelector("#handoff-copy").addEventListener("click", (event) => { event.stopPropagation(); copyHandoff(); });
document.querySelector("#refresh").addEventListener("click", () => load(result.repo));
document.querySelector("#switch-project").addEventListener("click", () => window.Guide && window.Guide.open());
window.loadProject = (path) => { state.sessionIdx = 0; state.turnId = "net"; load(path); };
window.addEventListener("focus", () => { if (!state.loading && result.repo) load(result.repo); });
if (location.protocol.startsWith("http")) new EventSource("/api/watch").onmessage = () => { if (!state.loading && result.repo) load(result.repo); };
document.querySelector("#turn-bar").addEventListener("click", (event) => {
  const pill = event.target.closest("[data-turn-id]");
  if (!pill) return;
  selectTurn(pill.dataset.turnId);
  render();
});
document.querySelector("#sessions").addEventListener("click", (event) => {
  const file = event.target.closest("[data-file]");
  const button = event.target.closest("[data-session]");
  if (!button) return;
  state.sessionIdx = Number(button.dataset.session);
  if (!(session() && session().assembled)) { openSession(); return; }
  if (file) window.K.openFile("net", file.dataset.file);
  else { selectTurn("net"); render(); }
});
document.querySelector(".stage").addEventListener("click", (event) => {
  if (window.Chain && window.Chain.swallowClick()) return;
  if (!turn()) return;
  const pick = [["[data-file]", "file", "file"], ["[data-focus-edge]", "edge", "focusEdge"], ["[data-focus-chain]", "chain", "focusChain"], ["[data-edge]", "edge", "edge"], ["[data-node]", "node", "node"], ["[data-chain]", "chain", "chain"]];
  for (const [selector, type, key] of pick) {
    const hit = event.target.closest(selector);
    if (!hit) continue;
    state.focus = type === "file" && state.focus.type === "file" && state.focus.id === hit.dataset[key] ? { type: "none" } : { type, id: hit.dataset[key] };
    render();
    followFocus();
    return;
  }
});
load(window.Guide && window.Guide.remembered() || undefined);

// 活动图：ELK 负责分层和正交走线。判断是一颗菱形，分支写在离开菱形的箭头上，格式是 UML 的 [条件]。
// 画布自己管方向（默认纵向）、疏密、缩放和视口。视口记的是「视口中心那颗星的 id」，
// 所以换方向、换疏密、点节点、拖分隔条、改窗口，视线都不跳；只有换回合、换会话才回到顶部重新适应。

const NS = "http://www.w3.org/2000/svg";

// 排版的形状参数。方向和疏密由人调，不写在这里。
const BASE_LAYOUT = {
  "elk.algorithm": "layered",
  "elk.edgeRouting": "ORTHOGONAL",
  "elk.spacing.edgeNode": "14",
  "elk.spacing.edgeEdge": "12",
  "elk.spacing.edgeLabel": "6",
  "elk.layered.spacing.edgeNodeBetweenLayers": "12",
  "elk.layered.nodePlacement.strategy": "BRANDES_KOEPF",
  "elk.padding": "[top=20,left=32,bottom=20,right=32]",
  "elk.separateConnectedComponents": "true",
  "elk.spacing.componentComponent": "48",
  "elk.edgeLabels.placement": "CENTER"
};

const DENSITY = {
  tight: { "elk.spacing.nodeNode": "24", "elk.layered.spacing.nodeNodeBetweenLayers": "52" },
  loose: { "elk.spacing.nodeNode": "36", "elk.layered.spacing.nodeNodeBetweenLayers": "76" }
};

const DIR_KEY = "klar-chain-direction";
const DENSITY_KEY = "klar-chain-density";
const SCALE_MIN = 0.06;
const SCALE_MAX = 6;
// 适应时最多放到 1.12，再大就只剩一两个方框了。
const FIT_MAX = 1.12;
// 刚打开时不能缩到看不清。24 条链一起铺开只有 0.37，所以起手用这个下限，剩下的让人拖。
const START_MIN = 0.7;
const PAN_STEP = 120;
const PAN_STEP_BIG = 420;

let elk = null;
let ticket = 0;
let shown = "";
let cachedKey = "";
let cached = null;
let direction = "DOWN";
let density = "tight";
let lastPaint = null;
let repaintTicket = 0;
let swallow = false;
let hostEl = null;
let barEl = null;
let svg = null;
let placed = new Map();
let rawW = 1;
let rawH = 1;
let fitScale = 1;
let userScale = 1;
let atFit = true;

function readPref(key, fallback) {
  try {
    return localStorage.getItem(key) || fallback;
  } catch (error) {
    return fallback;
  }
}

function writePref(key, value) {
  try {
    localStorage.setItem(key, value);
  } catch (error) {
    // 存不住就用这一次的，不拦着人操作。
  }
}

direction = readPref(DIR_KEY, "DOWN") === "RIGHT" ? "RIGHT" : "DOWN";
density = readPref(DENSITY_KEY, "tight") === "loose" ? "loose" : "tight";

function engine() {
  if (!window.ELK) return null;
  if (!elk) elk = new window.ELK();
  return elk;
}

function layoutOptions() {
  return Object.assign({}, BASE_LAYOUT, DENSITY[density], { "elk.direction": direction });
}

function textWidth(text, size) {
  let width = 0;
  for (const ch of String(text)) width += ch.charCodeAt(0) > 255 ? size : size * 0.62;
  return Math.ceil(width);
}

function guardText(call) {
  const raw = (call.guard || "").trim();
  if (!raw || raw.includes("\u0001") || raw.includes("\u001e")) return "";
  const text = raw.startsWith("[") ? raw : `[${raw}]`;
  const chars = [...text];
  return chars.length > 48 ? `${chars.slice(0, 47).join("")}…` : text;
}

function isLoop(star) {
  return star.kind === "if" && (star.label === "for" || star.label === "while");
}

function boxOf(star) {
  if (isLoop(star)) return { w: 84, h: 48 };
  if (star.kind === "if") return { w: 78, h: 78 };
  const width = Math.max(138, Math.min(280, textWidth(star.label || star.id, 13) + 36));
  return { w: width, h: 52 };
}

function graphOf(item) {
  const stars = item.stars || [];
  const ids = new Set(stars.map((star) => star.id));
  const calls = (item.calls || []).filter((call) => ids.has(call.from) && ids.has(call.to) && call.from !== call.to);
  const children = stars.map((star) => {
    const box = boxOf(star);
    return { id: star.id, width: box.w, height: box.h };
  });
  const edges = calls.map((call) => {
    const label = guardText(call);
    const edge = { id: call.key, sources: [call.from], targets: [call.to] };
    if (label) edge.labels = [{ text: label, width: textWidth(label, 12) + 18, height: 22 }];
    return edge;
  });
  // layoutOptions 每次现给一份，ELK 会就地改它，共用会串味。
  return { id: "root", layoutOptions: layoutOptions(), children, edges };
}

function el(name, attrs) {
  const node = document.createElementNS(NS, name);
  Object.entries(attrs || {}).forEach(([key, value]) => node.setAttribute(key, String(value)));
  return node;
}

function segHit(a, b, c, d) {
  const rx = b.x - a.x;
  const ry = b.y - a.y;
  const sx = d.x - c.x;
  const sy = d.y - c.y;
  const den = rx * sy - ry * sx;
  if (Math.abs(den) < 1e-6) return null;
  const t = ((c.x - a.x) * sy - (c.y - a.y) * sx) / den;
  const u = ((c.x - a.x) * ry - (c.y - a.y) * rx) / den;
  if (t < -0.02 || t > 1.02 || u < -0.02 || u > 1.02) return null;
  return { x: a.x + t * rx, y: a.y + t * ry };
}

function diamondVerts(box) {
  const cx = box.x + box.width / 2;
  const cy = box.y + box.height / 2;
  return [
    { x: cx, y: box.y },
    { x: box.x + box.width, y: cy },
    { x: cx, y: box.y + box.height },
    { x: box.x, y: cy }
  ];
}

// 布局把线停在菱形的外接矩形上。顺着这条线再往里收到菱形边上，避免插进角里。
function dockDiamond(box, points, atEnd) {
  const index = atEnd ? points.length - 1 : 0;
  const from = points[atEnd ? points.length - 2 : 1];
  const border = points[index];
  if (!from || !border) return;
  const dx = border.x - from.x;
  const dy = border.y - from.y;
  const len = Math.hypot(dx, dy) || 1;
  const far = {
    x: border.x + (dx / len) * Math.max(box.width, box.height),
    y: border.y + (dy / len) * Math.max(box.width, box.height)
  };
  const verts = diamondVerts(box);
  let best = null;
  let bestD = Infinity;
  for (let i = 0; i < 4; i += 1) {
    const hit = segHit(from, far, verts[i], verts[(i + 1) % 4]);
    if (!hit) continue;
    const dist = (hit.x - border.x) ** 2 + (hit.y - border.y) ** 2;
    if (dist < bestD) {
      bestD = dist;
      best = hit;
    }
  }
  if (best) points[index] = best;
}

function route(edge, nodes, diamonds) {
  const section = (edge.sections && edge.sections[0]) || {};
  const points = [];
  if (section.startPoint) points.push({ ...section.startPoint });
  (section.bendPoints || []).forEach((point) => points.push({ ...point }));
  if (section.endPoint) points.push({ ...section.endPoint });
  if (points.length < 2) return points;
  const source = nodes.get(edge.sources[0]);
  const target = nodes.get(edge.targets[0]);
  if (source && diamonds.has(source.id)) dockDiamond(source, points, false);
  if (target && diamonds.has(target.id)) dockDiamond(target, points, true);
  if (source && diamonds.has(source.id)) return alongDecision(points);
  const slim = [];
  points.forEach((point, index) => {
    const prev = slim[slim.length - 1];
    if (prev && Math.abs(prev.x - point.x) < 0.8 && Math.abs(prev.y - point.y) < 0.8) return;
    if (index > 0 && index < points.length - 1) {
      const next = points[index + 1];
      const sameX = Math.abs(prev.x - point.x) < 0.8 && Math.abs(point.x - next.x) < 0.8;
      const sameY = Math.abs(prev.y - point.y) < 0.8 && Math.abs(point.y - next.y) < 0.8;
      if (prev && (sameX || sameY)) return;
    }
    slim.push(point);
  });
  return slim;
}

// 判定出去的线先往下走，条件标在菱形下面，再横着拐进目标。
function alongDecision(points) {
  if (points.length < 2) return points;
  const start = points[0];
  const end = points[points.length - 1];
  if (Math.abs(start.x - end.x) < 8) return [start, { x: start.x, y: end.y }];
  const channel = end.y - 26;
  if (channel < start.y + 36) return points;
  return [start, { x: start.x, y: channel }, { x: end.x, y: channel }, end];
}

function rounded(points) {
  if (points.length < 3) {
    return `M ${points[0].x} ${points[0].y} L ${points[points.length - 1].x} ${points[points.length - 1].y}`;
  }
  const parts = [`M ${points[0].x} ${points[0].y}`];
  for (let i = 1; i < points.length - 1; i += 1) {
    const prev = points[i - 1];
    const cur = points[i];
    const next = points[i + 1];
    const d1 = Math.hypot(cur.x - prev.x, cur.y - prev.y) || 1;
    const d2 = Math.hypot(next.x - cur.x, next.y - cur.y) || 1;
    const radius = Math.min(12, d1 / 2, d2 / 2);
    const a = { x: cur.x - ((cur.x - prev.x) / d1) * radius, y: cur.y - ((cur.y - prev.y) / d1) * radius };
    const b = { x: cur.x + ((next.x - cur.x) / d2) * radius, y: cur.y + ((next.y - cur.y) / d2) * radius };
    parts.push(`L ${a.x} ${a.y} Q ${cur.x} ${cur.y} ${b.x} ${b.y}`);
  }
  const last = points[points.length - 1];
  parts.push(`L ${last.x} ${last.y}`);
  return parts.join(" ");
}

function longestMid(points) {
  let best = points[0];
  let span = 0;
  for (let i = 1; i < points.length; i += 1) {
    const len = Math.hypot(points[i].x - points[i - 1].x, points[i].y - points[i - 1].y);
    if (len > span) {
      span = len;
      best = { x: (points[i - 1].x + points[i].x) / 2, y: (points[i - 1].y + points[i].y) / 2 };
    }
  }
  return best;
}

// ---- 视口和缩放。视口记布局坐标，不是像素，换布局以后还能对上。----

function scaleUnit() {
  if (!svg) return 1;
  return (svg.getBoundingClientRect().width || rawW) / rawW;
}

function pointOf(clientX, clientY) {
  if (!svg) return null;
  const r = svg.getBoundingClientRect();
  const k = scaleUnit();
  return { x: (clientX - r.left) / k, y: (clientY - r.top) / k };
}

// 视口正中此刻落在哪个布局坐标、挨着哪颗星。
function centerPoint() {
  if (!svg || !hostEl) return null;
  const r = svg.getBoundingClientRect();
  const hr = hostEl.getBoundingClientRect();
  const k = scaleUnit();
  const x = (hr.left + hostEl.clientWidth / 2 - r.left) / k;
  const y = (hr.top + hostEl.clientHeight / 2 - r.top) / k;
  return { x, y, id: nearestNode(x, y) };
}

// 把某个布局坐标放回顾口正中。改 scroll 之后 rect 会跟着动，所以先量后调，一步到位。
function centerOn(point) {
  if (!svg || !hostEl || !point) return;
  const r = svg.getBoundingClientRect();
  const hr = hostEl.getBoundingClientRect();
  const k = scaleUnit();
  hostEl.scrollLeft += r.left - (hr.left + hostEl.clientWidth / 2 - point.x * k);
  hostEl.scrollTop += r.top - (hr.top + hostEl.clientHeight / 2 - point.y * k);
}

function nearestNode(x, y) {
  let best = null;
  let bestD = Infinity;
  for (const node of placed.values()) {
    const dx = node.x + node.width / 2 - x;
    const dy = node.y + node.height / 2 - y;
    const d = dx * dx + dy * dy;
    if (d < bestD) {
      bestD = d;
      best = node.id;
    }
  }
  return best;
}

function bboxOf(ids) {
  const list = (ids || []).map((id) => placed.get(id)).filter(Boolean);
  if (!list.length) return null;
  let minX = Infinity;
  let minY = Infinity;
  let maxX = -Infinity;
  let maxY = -Infinity;
  for (const node of list) {
    minX = Math.min(minX, node.x);
    minY = Math.min(minY, node.y);
    maxX = Math.max(maxX, node.x + node.width);
    maxY = Math.max(maxY, node.y + node.height);
  }
  return { x: (minX + maxX) / 2, y: (minY + maxY) / 2, w: maxX - minX, h: maxY - minY };
}

// 把这些星挪到画布中间；中间放不下就先缩到放得下。外部点全链、点跳都走这里。
// 布局还没排完就先记着，排完再兑现，不然第一次打开会话时点一下就没了。
function frame(ids) {
  const box = bboxOf(ids);
  if (!box || !hostEl) return;
  const need = Math.min((hostEl.clientWidth - 56) / Math.max(box.w, 1), (hostEl.clientHeight - 28) / Math.max(box.h, 1));
  if (need < userScale) {
    userScale = Math.max(need, fitScale);
    atFit = false;
    applyScale();
  }
  centerOn({ x: box.x, y: box.y });
}

let pendingReveal = null;

function reveal(ids) {
  const list = (ids || []).slice();
  if (!placed.size) {
    pendingReveal = list;
    return;
  }
  pendingReveal = null;
  frame(list);
}

function refit() {
  if (!hostEl) return;
  const roomW = Math.max(hostEl.clientWidth - 64, 1);
  const roomH = Math.max(hostEl.clientHeight - 28, 1);
  fitScale = Math.min(FIT_MAX, Math.max(SCALE_MIN, Math.min(roomW / rawW, roomH / rawH)));
}

function applyScale() {
  if (!svg) return;
  const scale = Math.min(SCALE_MAX, Math.max(SCALE_MIN, userScale));
  userScale = scale;
  svg.setAttribute("width", rawW * scale);
  svg.setAttribute("height", rawH * scale);
  if (barEl) {
    const out = barEl.querySelector("[data-act=pct]");
    if (out) out.textContent = `${Math.round(scale * 100)}%`;
  }
}

function fitView() {
  refit();
  userScale = fitScale;
  atFit = true;
  applyScale();
  centerOn({ x: rawW / 2, y: rawH / 2 });
}

// 换回合、换会话之后重新起头：缩到看得清，再落在这次改动上。
function resetView(item) {
  refit();
  userScale = Math.max(fitScale, START_MIN);
  atFit = false;
  applyScale();
  const seeds = (item.stars || []).filter((star) => star.mark).map((star) => star.id);
  if (seeds.length) {
    frame(seeds);
    return;
  }
  const first = (item.chains || [])[0];
  if (first) frame(first.stars);
  else centerOn({ x: rawW / 2, y: rawH / 2 });
}

function oneToOne() {
  userScale = 1;
  atFit = false;
  applyScale();
}

// 光标底下那一块图不动。
function zoomBy(factor, clientX, clientY) {
  if (!svg || !hostEl) return;
  const before = clientX == null ? null : pointOf(clientX, clientY);
  userScale = userScale * factor;
  atFit = false;
  applyScale();
  if (before && clientX != null) {
    const r = svg.getBoundingClientRect();
    const k = scaleUnit();
    hostEl.scrollLeft += r.left + before.x * k - clientX;
    hostEl.scrollTop += r.top + before.y * k - clientY;
  }
}

function pan(dx, dy) {
  if (!hostEl) return;
  hostEl.scrollLeft += dx;
  hostEl.scrollTop += dy;
}

// ---- 画布上的控件条。放在画布外层，不跟着滚动跑。----

function ensureBar(host) {
  const pane = host.parentElement;
  if (barEl && barEl.isConnected) return barEl;
  barEl = document.createElement("div");
  barEl.className = "chart-bar";
  barEl.innerHTML = `
    <button type="button" data-act="dir" title="横向 / 纵向">纵向</button>
    <button type="button" data-act="dense" title="紧凑 / 宽松">紧凑</button>
    <span class="chart-bar-gap"></span>
    <button type="button" data-act="out" aria-label="缩小">−</button>
    <output data-act="pct">100%</output>
    <button type="button" data-act="in" aria-label="放大">＋</button>
    <button type="button" data-act="fit">适应</button>
    <button type="button" data-act="one">1:1</button>
  `;
  pane.appendChild(barEl);
  barEl.addEventListener("click", (event) => {
    const button = event.target.closest("[data-act]");
    if (!button) return;
    event.stopPropagation();
    onBarAction(button.dataset.act);
  });
  syncBar();
  return barEl;
}

function syncBar() {
  if (!barEl) return;
  const dir = barEl.querySelector("[data-act=dir]");
  const dense = barEl.querySelector("[data-act=dense]");
  if (dir) dir.textContent = direction === "DOWN" ? "纵向" : "横向";
  if (dense) dense.textContent = density === "tight" ? "紧凑" : "宽松";
}

// 换方向、换疏密要重新排版。排完用刚才视口中心那颗星找回来。
function relayout() {
  if (!lastPaint || !lastPaint.host.isConnected) return;
  Chain.render(lastPaint.host, lastPaint.item, lastPaint.highlight);
}

function onBarAction(act) {
  if (act === "in") zoomBy(1.25);
  else if (act === "out") zoomBy(1 / 1.25);
  else if (act === "fit") fitView();
  else if (act === "one") oneToOne();
  else if (act === "dir") {
    direction = direction === "DOWN" ? "RIGHT" : "DOWN";
    writePref(DIR_KEY, direction);
    syncBar();
    relayout();
  } else if (act === "dense") {
    density = density === "tight" ? "loose" : "tight";
    writePref(DENSITY_KEY, density);
    syncBar();
    relayout();
  }
}

function paint(host, item, laid, highlight) {
  hostEl = host;
  const stars = new Map((item.stars || []).map((star) => [star.id, star]));
  const calls = new Map((item.calls || []).map((call) => [call.key, call]));
  placed = new Map((laid.children || []).map((node) => [node.id, node]));
  rawW = Math.max(laid.width || 0, 1);
  rawH = Math.max(laid.height || 0, 1);
  const diamonds = new Set((item.stars || []).filter((star) => star.kind === "if" && !isLoop(star)).map((star) => star.id));
  const litNodes = highlight && highlight.nodes;
  const litEdges = highlight && highlight.edges;
  const focus = (highlight && highlight.focus) || {};
  const dimOn = focus.type === "chain" && litNodes && litNodes.size > 0;
  const picture = el("svg", { viewBox: `0 0 ${rawW} ${rawH}` });
  const defs = el("defs");
  const marker = el("marker", { id: "arrow", viewBox: "0 0 8 8", refX: "7.2", refY: "4", markerWidth: "7", markerHeight: "7", orient: "auto" });
  marker.appendChild(el("path", { d: "M0.4 0.7 L7.2 4 L0.4 7.3 Z", fill: "context-stroke" }));
  defs.appendChild(marker);
  picture.appendChild(defs);

  // 药丸统一最后画：节点会盖住先画的标签。
  const pills = [];
  (laid.edges || []).forEach((edge) => {
    const call = calls.get(edge.id);
    if (!call) return;
    const points = route(edge, placed, diamonds);
    if (points.length < 2) return;
    const dim = dimOn && !(litEdges && litEdges.has(call.key));
    const group = el("g", {
      class: dim ? "wire-wrap is-dim" : "wire-wrap",
      "data-edge": call.key
    });
    const path = rounded(points);
    const kind = ["wire", `chg-${call.change || "none"}`];
    if (call.uncertain && call.change !== "del") kind.push("is-unsure");
    if (focus.type === "edge" && focus.id === call.key) kind.push("is-on");
    group.appendChild(el("path", { class: "hit", d: path }));
    group.appendChild(el("path", { class: kind.join(" "), d: path, "marker-end": "url(#arrow)" }));
    const label = guardText(call);
    if (label) {
      const mid = longestMid(points);
      const width = textWidth(label, 12) + 16;
      const pill = el("g", { class: "pill", transform: `translate(${mid.x - width / 2}, ${mid.y - 10})` });
      const tip = el("title");
      tip.textContent = (call.guard || label).trim();
      pill.appendChild(tip);
      pill.appendChild(el("rect", { class: "pill-bg", width, height: 20 }));
      const text = el("text", { class: "plabel", x: width / 2, y: 14 });
      text.textContent = label;
      pill.appendChild(text);
      pills.push(pill);
    }
    picture.appendChild(group);
  });

  (laid.children || []).forEach((node) => {
    const star = stars.get(node.id);
    if (!star) return;
    const dim = dimOn && !(litNodes && litNodes.has(star.id));
    const group = el("g", {
      class: `${dim ? "node is-dim" : "node"}${focus.type === "node" && focus.id === star.id ? " is-on" : ""}`,
      transform: `translate(${node.x}, ${node.y})`,
      "data-node": star.id
    });
    if (isLoop(star)) {
      group.appendChild(el("rect", {
        class: `card loop${star.mark ? ` mark-${star.mark}` : ""}`,
        width: node.width,
        height: node.height
      }));
      const text = el("text", { class: "dlabel", x: node.width / 2, y: node.height / 2 + 4, "text-anchor": "middle" });
      text.textContent = "循环";
      group.appendChild(text);
    } else if (star.kind === "if") {
      const cx = node.width / 2;
      const cy = node.height / 2;
      group.appendChild(el("polygon", {
        class: `diamond${star.mark ? ` mark-${star.mark}` : ""}`,
        points: `${cx},1 ${node.width - 1},${cy} ${cx},${node.height - 1} 1,${cy}`
      }));
      const text = el("text", { class: "dlabel", x: cx, y: cy + 4, "text-anchor": "middle" });
      text.textContent = star.label === "switch" ? "选择" : star.label === "catch" ? "捕获" : "判定";
      group.appendChild(text);
    } else {
      const eyebrow = { entry: "入口", leaf: "叶子", fn: "方法" }[star.kind] || "方法";
      group.appendChild(el("rect", {
        class: `card kind-${star.kind || "fn"}${star.mark ? ` mark-${star.mark}` : ""}`,
        width: node.width,
        height: node.height
      }));
      const kind = el("text", { class: "eyebrow", x: 14, y: 18 });
      kind.textContent = eyebrow;
      group.appendChild(kind);
      const name = el("text", { class: "name", x: 14, y: 37 });
      name.textContent = star.label || star.id;
      group.appendChild(name);
    }
    picture.appendChild(group);
  });

  // 标签层最后落位，永远在节点之上。
  pills.forEach((pill) => picture.appendChild(pill));

  const fit = document.createElement("div");
  fit.className = "fit";
  fit.appendChild(picture);
  host.replaceChildren(fit);
  svg = picture;
  refit();
  // 停在「适应」上的人，改窗口之后还是适应。
  if (atFit) userScale = fitScale;
  applyScale();
  ensureBar(host);
}

// 布局没变时只重画，不重新排版。窗口、分隔条、缩放都走这里。
function repaintSoon() {
  if (repaintTicket) return;
  repaintTicket = requestAnimationFrame(() => {
    repaintTicket = 0;
    if (!lastPaint || !lastPaint.host.isConnected) return;
    paint(lastPaint.host, lastPaint.item, lastPaint.laid, lastPaint.highlight);
  });
}

function armPan(host) {
  if (host.dataset.pan) return;
  host.dataset.pan = "1";
  let drag = null;
  host.addEventListener("pointerdown", (event) => {
    if (event.button !== 0 || event.target.closest("[data-node], [data-edge]")) return;
    drag = { x: event.clientX, y: event.clientY, left: host.scrollLeft, top: host.scrollTop, moved: false };
  });
  host.addEventListener("pointermove", (event) => {
    if (!drag) return;
    const dx = event.clientX - drag.x;
    const dy = event.clientY - drag.y;
    if (Math.hypot(dx, dy) > 4) drag.moved = true;
    host.scrollLeft = drag.left - dx;
    host.scrollTop = drag.top - dy;
  });
  host.addEventListener("pointerup", () => {
    // 拖过就不算点选。告诉外面一声，比在这儿拦一次 click 干净。
    if (drag && drag.moved) swallow = true;
    drag = null;
  });
  host.addEventListener("pointerleave", () => { drag = null; });
  host.addEventListener("dblclick", (event) => {
    const node = event.target.closest("[data-node]");
    if (node) reveal([node.dataset.node]);
    else fitView();
  });
  // ⌘/Ctrl 加滚轮缩放，触控板捏合也走 ctrlKey。普通滚轮留给滚动。
  host.addEventListener("wheel", (event) => {
    if (!event.ctrlKey && !event.metaKey) return;
    event.preventDefault();
    zoomBy(Math.exp(-event.deltaY * 0.0022), event.clientX, event.clientY);
  }, { passive: false });
}

function focusIds() {
  if (!lastPaint) return [];
  const where = (lastPaint.highlight && lastPaint.highlight.focus) || {};
  const item = lastPaint.item;
  if (where.type === "node") return [where.id];
  if (where.type === "edge") {
    const call = (item.calls || []).find((one) => one.key === where.id);
    return call ? [call.from, call.to] : [];
  }
  if (where.type === "chain") {
    const chain = (item.chains || [])[Number(String(where.id).replace(/^c/, ""))];
    return chain ? chain.stars : [];
  }
  return [];
}

function onKey(event) {
  if (!svg || !hostEl || !hostEl.isConnected) return;
  if (event.metaKey || event.ctrlKey || event.altKey) return;
  // 事件可能落在 window 或 document 上，它们没有 closest。
  if (event.target instanceof Element && event.target.closest("input, textarea, [contenteditable]")) return;
  const big = event.shiftKey;
  const step = big ? PAN_STEP_BIG : PAN_STEP;
  const keys = {
    ArrowLeft: () => pan(step, 0),
    ArrowRight: () => pan(-step, 0),
    ArrowUp: () => pan(0, step),
    ArrowDown: () => pan(0, -step),
    "+": () => zoomBy(1.25),
    "=": () => zoomBy(1.25),
    "-": () => zoomBy(1 / 1.25),
    _: () => zoomBy(1 / 1.25),
    "0": () => fitView(),
    "1": () => oneToOne(),
    f: () => reveal(focusIds()),
    F: () => reveal(focusIds())
  };
  const run = keys[event.key];
  if (!run) return;
  event.preventDefault();
  run();
}

function dataSig(item) {
  const stars = (item.stars || []).map((star) => `${star.id}:${star.kind}:${star.mark || ""}:${star.label}`).join(",");
  const calls = (item.calls || []).map((call) => `${call.key}:${call.change}:${call.guard || ""}:${call.uncertain ? 1 : 0}`).join(",");
  return `${item.id}|${stars}|${calls}`;
}

const Chain = {
  ok() { return Boolean(engine()); },
  clear() {
    ticket += 1;
    shown = "";
    cachedKey = "";
    cached = null;
    lastPaint = null;
    hostEl = null;
    svg = null;
    placed = new Map();
    if (barEl && barEl.isConnected) barEl.remove();
    barEl = null;
  },
  resize() { repaintSoon(); },
  // 拖过画布之后的那一次 click 不该算点选。外面取一次就清掉。
  swallowClick() {
    if (!swallow) return false;
    swallow = false;
    return true;
  },
  reveal,
  focusCurrent() { reveal(focusIds()); },
  render(host, item, highlight) {
    const layout = engine();
    if (!layout || !item) return false;
    armPan(host);
    const focus = (highlight && highlight.focus) || {};
    const view = `${dataSig(item)}|${focus.type || ""}:${focus.id || ""}|${highlight && highlight.nodes ? highlight.nodes.size : 0}|${direction}|${density}`;
    if (view === shown && host.querySelector("svg")) return true;
    shown = view;
    const mine = ++ticket;
    const key = `${dataSig(item)}|${direction}|${density}`;
    const reused = cachedKey === key && cached;
    const sameItem = Boolean(lastPaint && lastPaint.item === item);
    // keep 一直是「新布局里的一个点」。
    // 同一份布局就按旧视口中心那个精确坐标还原；换了布局（方向/疏密）就认视口中心那颗星，
    // 顺着它所在的那条链重新框一次。都没有就当作换了一件事，回到能看清的尺度重新起头。
    let keep = null;
    let keepIds = null;
    if (sameItem) {
      if (reused) {
        const point = centerPoint();
        if (point) keep = { x: point.x, y: point.y };
      } else {
        const anchor = nearestOfView();
        if (anchor) {
          const chain = (lastPaint.item.chains || []).find((one) => one.stars.includes(anchor));
          keepIds = chain ? chain.stars : [anchor];
        }
      }
    }
    const draw = (laid) => {
      if (mine !== ticket) return;
      paint(host, item, laid, highlight);
      if (keepIds) frame(keepIds);
      else if (keep) centerOn(keep);
      else resetView(item);
      // 排版期间人已经点了某条链，排完补上。
      if (pendingReveal) {
        const ids = pendingReveal;
        pendingReveal = null;
        frame(ids);
      }
      lastPaint = { host, item, laid, highlight };
    };
    if (reused) {
      draw(cached);
      return true;
    }
    console.info("调用链布局", {
      方向: direction,
      疏密: density,
      节点: (item.stars || []).length,
      判断: (item.stars || []).filter((star) => star.kind === "if").length,
      边: (item.calls || []).length
    });
    layout.layout(graphOf(item)).then((laid) => {
      if (mine !== ticket) return;
      cachedKey = key;
      cached = laid;
      draw(laid);
    }).catch((error) => {
      if (mine !== ticket) return;
      host.textContent = "这张图画不出来。";
      console.error("调用链绘图失败", error);
    });
    return true;
  }
};

function nearestOfView() {
  const point = centerPoint();
  return point ? point.id : null;
}

window.Chain = Chain;
window.addEventListener("resize", () => Chain.resize());
window.addEventListener("keydown", onKey);

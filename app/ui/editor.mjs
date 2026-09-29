// 点开方法、调用或文件时用的只读编辑器。前后不一样就在同一栏里画出 diff。
// 一个 host 一个视图：右栏的小窗和全屏那层可以同时开着。换亮暗时各自按新配色原地重开。

import { EditorState } from "@codemirror/state";
import { EditorView, lineNumbers, drawSelection } from "@codemirror/view";
import { bracketMatching, HighlightStyle, syntaxHighlighting } from "@codemirror/language";
import { unifiedMergeView } from "@codemirror/merge";
import { tags } from "@lezer/highlight";
import { javascript } from "@codemirror/lang-javascript";
import { java } from "@codemirror/lang-java";
import { python } from "@codemirror/lang-python";
import { go } from "@codemirror/lang-go";

const ink = HighlightStyle.define([
  { tag: tags.keyword, color: "#e0b15a" },
  { tag: tags.string, color: "#d7c48a" },
  { tag: tags.comment, color: "#6e6a5c", fontStyle: "italic" },
  { tag: tags.number, color: "#efe0b0" },
  { tag: tags.function(tags.variableName), color: "#f7f1e4" },
  { tag: tags.typeName, color: "#c6d4c0" },
  { tag: tags.operator, color: "#9a937f" },
  { tag: tags.bracket, color: "#9a937f" },
  { tag: tags.definition(tags.variableName), color: "#f3e2bc" }
]);

const inkLight = HighlightStyle.define([
  { tag: tags.keyword, color: "#8a5a00" },
  { tag: tags.string, color: "#7a5c12" },
  { tag: tags.comment, color: "#8d8a80", fontStyle: "italic" },
  { tag: tags.number, color: "#6f4b00" },
  { tag: tags.function(tags.variableName), color: "#1f2430" },
  { tag: tags.typeName, color: "#2f5d3a" },
  { tag: tags.operator, color: "#5c5a52" },
  { tag: tags.bracket, color: "#5c5a52" },
  { tag: tags.definition(tags.variableName), color: "#4a3a10" }
]);

const face = {
  "&": { height: "auto" },
  ".cm-scroller": { overflow: "visible", fontFamily: "\"SF Mono\", \"JetBrains Mono\", ui-monospace, monospace" },
  ".cm-content": { padding: "8px 0" },
  ".cm-activeLine": { background: "transparent" },
  ".cm-activeLineGutter": { background: "transparent" }
};

const themeDark = EditorView.theme(Object.assign({}, face, {
  "&": { background: "#10120c", color: "#efe8d6" },
  ".cm-content": { padding: "8px 0", caretColor: "#e0b15a" },
  ".cm-gutters": { background: "#16180f", color: "#6e6a5c", border: "none" },
  "& .cm-deletedChunk": { background: "rgba(210, 122, 104, 0.16)" },
  "& .cm-deletedLine, & .cm-deletedLine del": { color: "#d7b2a8", textDecoration: "line-through" },
  "& .cm-changedLine": { background: "rgba(224, 177, 90, 0.14)" },
  "& .cm-changedText": { background: "rgba(224, 177, 90, 0.28)" }
}), { dark: true });

const themeLight = EditorView.theme(Object.assign({}, face, {
  "&": { background: "#ffffff", color: "#24262b" },
  ".cm-content": { padding: "8px 0", caretColor: "#8a5a00" },
  ".cm-gutters": { background: "#f6f6f8", color: "#9a9aa3", border: "none" },
  "& .cm-deletedChunk": { background: "rgba(214, 69, 69, 0.10)" },
  "& .cm-deletedLine, & .cm-deletedLine del": { color: "#a83232", textDecoration: "line-through" },
  "& .cm-changedLine": { background: "rgba(180, 83, 9, 0.08)" },
  "& .cm-changedText": { background: "rgba(180, 83, 9, 0.20)" }
}), { dark: false });

// 主题由页面定：data-theme 一定被写上，读它比读 matchMedia 准。
function isDark() {
  return document.documentElement.dataset.theme === "dark";
}

function languageOf(path) {
  const ext = String(path || "").split(".").pop().toLowerCase();
  if (ext === "java") return java();
  if (ext === "py") return python();
  if (ext === "go") return go();
  if (ext === "ts" || ext === "tsx" || ext === "mts" || ext === "cts") return javascript({ typescript: true, jsx: true });
  return javascript({ jsx: true });
}

// 每个 host 一份：视图 + 打开时那份正文。换主题要按原样重开，所以正文得留着。
const open2 = new Map();

const CodeView = {
  clear() {
    for (const host of [...open2.keys()]) this.close(host);
  },
  close(host) {
    const entry = open2.get(host);
    if (!entry) return;
    entry.view.destroy();
    host.replaceChildren();
    open2.delete(host);
  },
  open(host, spec) {
    this.close(host);
    const before = spec.before || "";
    const after = spec.after || "";
    const changed = before !== after;
    const dark = isDark();
    const extensions = [
      lineNumbers(),
      drawSelection(),
      bracketMatching(),
      syntaxHighlighting(dark ? ink : inkLight),
      languageOf(spec.path),
      EditorView.editable.of(false),
      EditorState.readOnly.of(true),
      EditorView.lineWrapping
    ];
    if (changed) {
      extensions.push(unifiedMergeView({
        original: before,
        highlightChanges: true,
        gutter: true,
        syntaxHighlightDeletions: true,
        mergeControls: false,
        allowInlineDiffs: true,
        collapseUnchanged: { margin: 3, minSize: 6 }
      }));
    }
    extensions.push(dark ? themeDark : themeLight);
    const view = new EditorView({ parent: host, doc: after, extensions });
    open2.set(host, { view, spec: { path: spec.path || "", before, after } });
    console.info("代码编辑器", { 文件: spec.path || "", 对照: changed, 行: after.split("\n").length, 皮肤: dark ? "深" : "浅" });
  },
  // 换亮暗：把还开着的编辑器按新配色原地重开。
  refreshTheme() {
    for (const [host, entry] of [...open2.entries()]) {
      const spec = entry.spec;
      this.close(host);
      this.open(host, spec);
    }
  }
};

window.CodeView = CodeView;
window.addEventListener("klar:theme", () => window.CodeView && window.CodeView.refreshTheme());

// 点开方法或调用时用的只读编辑器。前后不一样就在同一栏里画出 diff。
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

const theme = EditorView.theme({
  "&": { background: "#10120c", color: "#efe8d6", height: "auto" },
  ".cm-scroller": { overflow: "visible", fontFamily: "\"SF Mono\", \"JetBrains Mono\", ui-monospace, monospace" },
  ".cm-content": { padding: "8px 0", caretColor: "#e0b15a" },
  ".cm-gutters": { background: "#16180f", color: "#6e6a5c", border: "none" },
  ".cm-activeLine": { background: "transparent" },
  ".cm-activeLineGutter": { background: "transparent" },
  "& .cm-deletedChunk": { background: "rgba(210, 122, 104, 0.16)" },
  "& .cm-deletedLine, & .cm-deletedLine del": { color: "#d7b2a8", textDecoration: "line-through" },
  "& .cm-changedLine": { background: "rgba(224, 177, 90, 0.14)" },
  "& .cm-changedText": { background: "rgba(224, 177, 90, 0.28)" }
}, { dark: true });

let current = null;

function languageOf(path) {
  const ext = String(path || "").split(".").pop().toLowerCase();
  if (ext === "java") return java();
  if (ext === "py") return python();
  if (ext === "go") return go();
  if (ext === "ts" || ext === "tsx" || ext === "mts" || ext === "cts") return javascript({ typescript: true, jsx: true });
  return javascript({ jsx: true });
}

const CodeView = {
  clear() {
    if (current) {
      current.destroy();
      current = null;
    }
  },
  open(host, spec) {
    this.clear();
    const before = spec.before || "";
    const after = spec.after || "";
    const changed = before !== after;
    const extensions = [
      lineNumbers(),
      drawSelection(),
      bracketMatching(),
      syntaxHighlighting(ink),
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
    extensions.push(theme);
    current = new EditorView({ parent: host, doc: after, extensions });
    console.info("代码编辑器", { 文件: spec.path || "", 对照: changed, 行: after.split("\n").length });
  }
};

window.CodeView = CodeView;

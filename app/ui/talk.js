// 主工作区的对话流。每回合一句你说的、一句它说的，改过的文件贴在回复下。
function renderTalk() {
  const sky = document.querySelector("#pane-sky");
  const root = document.querySelector("#talk");
  if (!sky || !root || !window.K) return;
  const mode = window.K.workspace();
  sky.dataset.workspace = mode;
  document.querySelectorAll("#workspace [data-workspace]").forEach((button) => {
    button.setAttribute("aria-current", button.dataset.workspace === mode ? "true" : "false");
  });
  const session = window.K.session();
  if (!session) {
    root.innerHTML = "";
    return;
  }
  const turns = [...(session.turns || [])].reverse();
  if (!turns.length) {
    root.innerHTML = `<p class="empty">这个会话还没有对话。</p>`;
    return;
  }
  const { esc } = window.K;
  root.innerHTML = turns.map((item) => {
    const files = [];
    const seen = new Set();
    (item.edits || []).forEach((edit) => {
      if (!edit.path || seen.has(edit.path)) return;
      seen.add(edit.path);
      files.push(edit.ok
        ? `<button type="button" class="attach" data-turn="${esc(item.id)}" data-file="${esc(edit.path)}" title="${esc(edit.path)}">${esc(edit.path)}</button>`
        : `<span class="attach skipped" title="${esc(edit.reason || "")}">${esc(edit.path)}</span>`);
    });
    return `
      <div class="bubble you">
        <p class="who">${esc(item.at || "")} 你</p>
        <p class="msg">${esc(item.prompt || "")}</p>
      </div>
      <div class="bubble ai">
        <p class="who">它</p>
        <p class="msg">${esc(item.reply || "这一回合没有文字回复。")}</p>
        ${files.length ? `<div class="files">${files.join("")}</div>` : ""}
      </div>
    `;
  }).join("");
  if (mode === "talk" && root.dataset.sid !== session.sid) {
    root.dataset.sid = session.sid;
    requestAnimationFrame(() => { root.scrollTop = root.scrollHeight; });
  }
}

document.querySelector("#workspace").addEventListener("click", (event) => {
  const tab = event.target.closest("[data-workspace]");
  if (!tab || !window.K) return;
  window.K.setWorkspace(tab.dataset.workspace);
});
document.querySelector("#talk").addEventListener("click", (event) => {
  const chip = event.target.closest("[data-file]");
  if (!chip || !window.K) return;
  event.stopPropagation();
  window.K.openFile(chip.dataset.turn, chip.dataset.file);
});
window.renderTalk = renderTalk;

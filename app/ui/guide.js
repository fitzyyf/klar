const GUIDE_KEY = "agent-review-project";

function shortPath(path) {
  return String(path || "").replace(/^\/Users\/[^/]+/, "~");
}

function renderGuide(projects, current) {
  const root = document.querySelector("#guide-list");
  if (!root) return;
  const rows = projects || [];
  root.innerHTML = rows.length ? rows.map((project) => `
    <button type="button" class="guide-project" data-project="${escAttr(project.path)}" aria-current="${project.path === current}">
      <span class="guide-path">${escAttr(shortPath(project.path))}</span>
      <span class="guide-counts">Claude ${project.claude || 0} · Codex ${project.codex || 0} · Grok ${project.grok || 0}</span>
      <span class="guide-when">${escAttr(project.latest || "")}</span>
    </button>
  `).join("") : '<p class="empty">没有找到带会话的项目。</p>';
}

function escAttr(value) {
  return String(value).replace(/[&<>"]/g, (char) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" }[char]));
}

function openGuide() {
  const guide = document.querySelector("#guide");
  if (guide) guide.hidden = false;
}

function closeGuide() {
  const guide = document.querySelector("#guide");
  if (guide) guide.hidden = true;
}

document.querySelector("#guide-list").addEventListener("click", (event) => {
  const button = event.target.closest("[data-project]");
  if (!button) return;
  localStorage.setItem(GUIDE_KEY, button.dataset.project);
  closeGuide();
  if (window.loadProject) window.loadProject(button.dataset.project);
});

document.querySelector("#guide-close").addEventListener("click", closeGuide);
document.querySelector("#guide").addEventListener("click", (event) => {
  if (event.target.id === "guide") closeGuide();
});

window.Guide = {
  render: renderGuide,
  open: openGuide,
  remembered() {
    return localStorage.getItem(GUIDE_KEY) || "";
  }
};

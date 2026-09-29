// 左右两道缝可以拖。宽度记在这个窗口里，下次打开还在。

const KEY = "agent-review-panes";
const leftPane = document.querySelector("#pane-sessions");
const rightPane = document.querySelector("#pane-detail");

function saved() {
  try {
    return JSON.parse(localStorage.getItem(KEY) || "null");
  } catch (error) {
    return null;
  }
}

function remember() {
  localStorage.setItem(KEY, JSON.stringify({
    left: Math.round(leftPane.getBoundingClientRect().width),
    right: Math.round(rightPane.getBoundingClientRect().width)
  }));
}

function nudge() {
  if (window.Chain && window.Chain.resize) window.Chain.resize();
}

function apply(left, right) {
  if (left) leftPane.style.width = `${Math.max(200, left)}px`;
  if (right) rightPane.style.width = `${Math.max(260, right)}px`;
  nudge();
}

const stored = saved();
if (stored) apply(stored.left, stored.right);

document.querySelectorAll(".gutter").forEach((gutter) => {
  gutter.addEventListener("pointerdown", (event) => {
    if (window.matchMedia("(max-width: 980px)").matches) return;
    event.preventDefault();
    const side = gutter.dataset.side;
    const startX = event.clientX;
    const startLeft = leftPane.getBoundingClientRect().width;
    const startRight = rightPane.getBoundingClientRect().width;
    document.body.classList.add("dragging");
    function move(next) {
      const dx = next.clientX - startX;
      if (side === "left") apply(startLeft + dx, null);
      else apply(null, startRight - dx);
    }
    function up() {
      document.body.classList.remove("dragging");
      window.removeEventListener("pointermove", move);
      window.removeEventListener("pointerup", up);
      remember();
      nudge();
    }
    window.addEventListener("pointermove", move);
    window.addEventListener("pointerup", up);
  });
});

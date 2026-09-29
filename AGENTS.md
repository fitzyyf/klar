# AGENTS.md

本仓库只写事实。通用规则见 `~/.claude/CLAUDE.md`。

## 形态：客户端模式

- 这是**桌面客户端**，不是 web 项目：`desktop/` 是 Wails v3 单进程客户端，`make app` 打出 `Agent会话Review.app`，双击运行。没有对外部署形态，不起对外服务。
- **硬限制：不得给这个项目加 web 形态**——不要加 `--serve` 之类的 HTTP 口子、不要建议用浏览器当运行或验证方式。看界面效果只有一个途径：`make app` 打包后直接看客户端窗口。
- 窗口里是 `app/ui` 的 HTML，页面和 `/api/*` 由同一个 handler 出（`desktop/api.go` 的 `handler`）。改界面、改接口都在这个客户端里生效。
- `app/` 只剩 `ui/`，`app/src-tauri`（Rust + Tauri）那一版已删。仓库里只有 `desktop/` 一套实现，`prd/` 是原型和文档。
- 打包入口只有一个：`desktop/Makefile`（对齐 magpie）。`make app` 出 macOS 的 `Agent会话Review.app`，`make windows` 在 Windows 上出单文件 `klar.exe`（go-winres 塞图标/版本/DPI 清单，需 MinGW-w64）。页面在编译前从 `app/ui` 复制进 `internal/uiembed` 用 `go:embed` 内嵌，二进制自带界面。tree-sitter 走 cgo，Windows 版不能从 Mac 交叉编。
- **启动**：`desktop/run.sh`（杀旧实例 → `make app` → 开窗口 → 确认进程活着）。23.yyf 说「启动」就是跑它。

## 画布

- 画布在 `app/ui/chain.js`。方向**默认纵向**，可切横向，记在 `localStorage`；疏密同理。24 条链一起铺开只有 37% 左右，所以起手不按「适应」，按 `START_MIN` 缩到看得清，再落在这次改动上。
- 视口记的是**布局坐标**（视口中心那个点，跨重排版则记中心那颗星的 id），不是像素。点节点、拖分隔条、改窗口、换方向都不能让视线跳。
- 画布行为只能靠 `make app` 打包后看窗口验。`node --check` 只管语法，管不了「点了会不会跳」。

## 评审结论怎么到 agent 手里

- **不投递。** 这个工具不跑 `smx-team`、不跑 tmux、不碰任何 agent 进程。会话名单只从三家的记录目录读。
- 点「记下通过 / 记下退回」只做两件事：把结论写进 `~/.klar/reviews.sqlite`，把要交回的那句话连同参照和提交说明一起返回给界面。
- 界面给一个只读框加「复制」按钮，**由人自己粘给 agent**。谁粘、粘到哪，是人的事，工具不代劳。
- 界面上不许出现「已发给」「已交给这个会话」这类话。记下了就是记下了，投递没发生就不能写成投递成功。

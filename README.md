# Agent会话Review

**给 AI agent 会话的评审台。** 按「这一回合让它做什么」，看它的改动碰到了哪些调用全链，看懂了，再下结论。桌面客户端，单二进制，本机跑，不用联网。

简体中文 | [English](README.en.md)

## 为什么需要它

一个人同时开几个 agent（Claude、Codex、Grok）在同一个仓库里干活，收尾时现有的办法都不够：

| 现在的做法 | 缺什么 |
| --- | --- |
| `git diff` / PR 按文件看行 | 几个会话的改动混在一个工作区，分不清谁改的、看不出调用关系、看不到为什么改 |
| IDE 里逐块接受 | 只在单个会话里，改一个方法连带影响了哪些入口看不见 |
| AI 评审机器人 | 评的还是 diff，结论还要人再读一遍 |

Agent会话Review 把三样东西连在一起：**谁改的**（归属到会话）、**为什么改**（那一回合的指令）、**波及到哪**（从入口到叶子的全链）。改的是下单，每晚对账也经过被改的库存预占——这条线会自己出现。

## 它是怎么看的

- **改动来自会话记录，不是 `git diff`。** 读 Claude / Codex / Grok 三家的本地会话记录，把每回合的写文件调用叠成改前、改后两份正文。几个会话并行时，各看各的，互不混。
- **全链画成 UML 活动图。** tree-sitter 切函数、按名字接调用：菱形是判定，方框写「循环」，箭头上 `[条件]` 是分支；金色是新加的，虚线是拆掉的，纵向横向都能摆。
- **每句说明都来自解析，不是模型写的。** 不跑 AI 评审，结论由人下。

## 开始

macOS + Go 1.26：

```sh
git clone https://github.com/fitzyyf/klar.git && cd klar/desktop
make app && open Agent会话Review.app    # 或 ./run.sh，一条命令重打包并启动
```

Windows：装 [MinGW-w64](https://www.mingw-w64.org/) 后在 `desktop/` 下 `make windows`，出单文件 `klar.exe`（WebView2 系统自带）。

## 怎么用

1. **选仓库选会话。** 左栏列出这个仓库里三家已经结束的会话，带 agent 名、时间、写过的文件。
2. **先看净改，再钻回合。** 中间默认是整个会话的净改全链；回合条切到单个回合，对话流对照你说的和它说的。
3. **点开看细节。** 点星看函数体加了删了什么，点跳看调用参数改了什么，断掉的链会写明断在哪个文件。
4. **下结论。** 「记下通过 / 记下退回」写进本机 `~/.klar/reviews.sqlite`，要交回的那句话给一个复制按钮，由人自己粘给 agent——工具不投递、不碰 agent 进程、不跑 git。

## 它不做什么

不做 git 客户端（不暂存不提交不推送）、不跑 AI 评审、不跑测试、不管人手改的文件。评审时它是阅读器，提交权在人。

## 从源码构建

| 目标 | 命令 | 产物 |
| --- | --- | --- |
| macOS 应用 | `make app` | `Agent会话Review.app`（页面已内嵌，含图标，ad-hoc 签名） |
| Windows 程序 | `make windows` | 单文件 `klar.exe`（go-winres 塞图标/版本/DPI 清单） |
| 测试 | `make test` | go vet + 全部测试 |
| 启动 | `./run.sh` | 重打包并开窗口 |

语言支持：Java、TypeScript / JavaScript / Vue、Python、Go。解析不接语言服务器，接不准的调用标「指向不确定」。

## 目录

```
desktop/   Go + Wails v3 客户端：engine（叠正文、编链）、三家会话解析、SQLite
app/ui/    原生 HTML 界面，编译时 go:embed 进二进制
prd/       产品定位、技术实现、界面原型
```

## License

MIT

---

英文版：[README.en.md](README.en.md) · 仓库事实与硬约束见 [AGENTS.md](AGENTS.md)

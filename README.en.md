# Agent会话Review (Agent Session Review)

**A review gate for AI agent sessions.** See what a turn was asked to do, what call chains its changes actually touched, understand it, then decide. A desktop client — single binary, runs locally, no cloud.

简体中文 | English

## Why

You run several agents (Claude, Codex, Grok) in parallel in the same repo. When it's time to wrap up, nothing today holds up:

| Today's approach | What's missing |
| --- | --- |
| `git diff` / PR line-by-line | Sessions blend into one working tree: who changed what, why, and what it ripples into — all invisible |
| Accepting hunks in an IDE | Trapped in one session; you can't see which entry points a changed method drags along |
| AI review bots | Still reviewing a diff, and their verdict needs another read |

Agent会话Review ties three things together: **who changed it** (attributed to a session), **why** (the turn's instruction), **what it touches** (entry-to-leaf call chains). Change order placement, and the nightly settlement job that passes through the same reserve method — that chain shows up on its own.

## How it looks at code

- **Changes come from session logs, not `git diff`.** It reads local Claude / Codex / Grok session records and folds each turn's file-write calls into before/after texts. Parallel sessions stay separate.
- **Chains render as a UML activity diagram.** tree-sitter carves out functions, calls are linked by name: diamonds are decisions, boxes labeled「循环」are loops, `[condition]` on an arrow is a branch. Gold is added, dashed is removed; layout goes vertical or horizontal.
- **Every explanation comes from parsing, not a model.** No AI reviewing; the verdict is yours.

## Getting started

macOS + Go 1.26:

```sh
git clone https://github.com/fitzyyf/klar.git && cd klar/desktop
make app && open Agent会话Review.app    # or ./run.sh — rebuild and launch in one step
```

Windows: install [MinGW-w64](https://www.mingw-w64.org/), then `make windows` in `desktop/` produces a single-file `klar.exe` (WebView2 comes with the OS).

## Usage

1. **Pick a repo, pick a session.** The left pane lists finished sessions from all three agents, with agent name, time, and touched files.
2. **Net changes first.** The canvas shows the whole session's net diff as chains, and the talk view lines up what you said with what it said.
3. **Click into details.** Click a star for the function body diff, a hop for changed call arguments; a broken chain names the file it broke on.
4. **Verdict.** 记下通过 / 记下退回 writes to local `~/.klar/reviews.sqlite` and hands you the reply sentence with a copy button — you paste it back to the agent yourself. The tool never delivers, never touches agent processes, never runs git.

## What it doesn't do

No git client (no staging/commit/push), no AI review, no test runs, and files you edited by hand are out of scope. It's a reader for review; committing stays with you.

## Build from source

| Target | Command | Output |
| --- | --- | --- |
| macOS app | `make app` | `Agent会话Review.app` (embedded UI, icon, ad-hoc signed) |
| Windows binary | `make windows` | single-file `klar.exe` (icon/version/DPI manifest via go-winres) |
| Tests | `make test` | go vet + full suite |
| Launch | `./run.sh` | rebuild + open window |

Languages: Java, TypeScript / JavaScript / Vue, Python, Go. No language server — calls that can't be resolved by name are flagged as uncertain.

## Layout

```
desktop/   Go + Wails v3 client: engine (replay, chain building), session readers, SQLite
app/ui/    plain-HTML UI, embedded into the binary at build time
prd/       product docs, tech notes, prototype
```

## License

MIT

// 一个进程里读会话、编链、记下评审。不再另起 Rust。
package engine

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"klar.dev/desktop/internal/graph"
	"klar.dev/desktop/internal/history"
	"klar.dev/desktop/internal/model"
	"klar.dev/desktop/internal/replay"
	"klar.dev/desktop/internal/sess"
	"klar.dev/desktop/internal/store"
)

type rawEntry struct {
	path string
	mod  time.Time
	size int64
	raw  *sess.RawSession
}

type App struct {
	mu      sync.Mutex
	rawMu   sync.Mutex
	home    string
	db      string
	indexes map[string]*graph.Index
	raws    map[string]rawEntry
}

func New() *App {
	home, _ := os.UserHomeDir()
	return WithHome(home)
}

// WithHome 换掉家目录。测试和「不在自己家目录跑」时用。
func WithHome(home string) *App {
	return &App{home: home, db: store.DefaultPath(home), indexes: map[string]*graph.Index{}, raws: map[string]rawEntry{}}
}

// readRaw 按记录文件的路径、修改时间和大小缓存读出来的回合。会话记录只在追加时变化，
// 没变的直接用上一次的解析结果，刷新不用整份重读。
func (a *App) readRaw(home, agent, sid string) *sess.RawSession {
	path := sess.Locate(home, agent, sid)
	if path == "" {
		log.Printf("没找到 %s 会话 %s 的记录", agent, sid)
		return nil
	}
	st, err := os.Stat(path)
	if err != nil {
		return nil
	}
	key := agent + "\x00" + sid
	a.rawMu.Lock()
	defer a.rawMu.Unlock()
	if hit, ok := a.raws[key]; ok && hit.path == path && hit.mod == st.ModTime() && hit.size == st.Size() {
		return hit.raw
	}
	raw := sess.Parse(agent, path)
	if raw == nil {
		return nil
	}
	if len(a.raws) >= 160 {
		a.raws = map[string]rawEntry{}
	}
	a.raws[key] = rawEntry{path: path, mod: st.ModTime(), size: st.Size(), raw: raw}
	return raw
}

// Load 列出这个仓库里已经结束的会话，并编出调用链。
func (a *App) Load(ctx context.Context, asked string) (model.Output, error) {
	started := time.Now()
	if err := ctx.Err(); err != nil {
		return model.Output{}, err
	}
	projects := history.Projects(a.home)
	out := model.Output{
		Repos:    projectPaths(projects),
		Projects: projectOuts(projects),
		Sessions: []model.SessionOut{},
		Notes:    []string{},
	}
	repo := a.pick(asked)
	if repo == "" {
		out.Notes = []string{"还没有选项目。"}
		nonempty(&out)
		return out, nil
	}
	out.Repo = repo
	log.Printf("列出会话 %s", repo)
	past := history.Closed(a.home, repo, nil)
	base := baseline(repo)
	log.Printf("参照 %s", base)
	saved := a.reviews(repo)
	sessions := make([]model.SessionOut, 0, len(past))
	for _, item := range past {
		if err := ctx.Err(); err != nil {
			return model.Output{}, err
		}
		if item.Agent != "claude" && item.Agent != "codex" && item.Agent != "grok" {
			continue
		}
		session := sketch(repo, base, item, a.readRaw(a.home, item.Agent, item.Sid))
		attachReview(&session, saved)
		sessions = append(sessions, session)
	}
	out.Sessions = sessions
	nonempty(&out)
	log.Printf("列出会话完成 %d 个，用时 %d 毫秒，先不编链", len(sessions), time.Since(started).Milliseconds())
	return out, nil
}

// Open 只组装选中的这一条：复原改过的文件，再编调用链。
func (a *App) Open(ctx context.Context, repo, agent, sid string) (model.SessionOut, error) {
	if err := ctx.Err(); err != nil {
		return model.SessionOut{}, err
	}
	repo = history.Canon(repo)
	agent = strings.ToLower(strings.TrimSpace(agent))
	if repo == "" || sid == "" || (agent != "claude" && agent != "codex" && agent != "grok") {
		return model.SessionOut{}, fmt.Errorf("没有这条会话")
	}
	log.Printf("组装会话 %s %s", agent, sid)
	base := baseline(repo)
	a.mu.Lock()
	defer a.mu.Unlock()
	idx := a.indexes[repo]
	if idx == nil {
		idx = &graph.Index{}
		a.indexes[repo] = idx
	}
	idx.Refresh(repo)
	session := buildSession(repo, base, history.Past{Agent: agent, Sid: sid}, idx, a.readRaw(a.home, agent, sid))
	session.Assembled = true
	attachReview(&session, a.reviews(repo))
	fillSession(&session)
	return session, nil
}

func (a *App) reviews(repo string) map[string]store.Saved {
	saved, err := store.Latest(a.db, repo)
	if err != nil {
		log.Printf("%s", err.Error())
		return map[string]store.Saved{}
	}
	return saved
}

func attachReview(session *model.SessionOut, saved map[string]store.Saved) {
	rec, ok := saved[session.Sid]
	if !ok {
		return
	}
	tip := ""
	if len(session.Turns) > 0 {
		tip = session.Turns[0].ID
	}
	session.Review = &model.ReviewOut{
		At: rec.At, Verdict: rec.Verdict, Text: rec.Text, Current: rec.TurnID == tip,
	}
}

// Deliver 把评审记在本机，并交回一句要粘给 agent 的话。这个工具自己不投递。
func (a *App) Deliver(repo, sid, verdict, opinion, title, turnID string) model.VerdictOut {
	if strings.TrimSpace(repo) == "" || sid == "" {
		return model.VerdictOut{Ok: false, Note: "没有会话。"}
	}
	if verdict != "pass" && verdict != "back" {
		return model.VerdictOut{Ok: false, Note: "不认识的结论。"}
	}
	text := handoff(repo, verdict, opinion, title)
	if err := store.Insert(a.db, repo, sid, verdict, text, turnID); err != nil {
		log.Printf("评审没记下：%v", err)
		return model.VerdictOut{Ok: false, Note: err.Error()}
	}
	log.Printf("会话 %s 记下评审 %s，回合 %s", sid, verdict, turnID)
	return model.VerdictOut{Ok: true, Note: "结论记在本机了。话复制给 agent，它才收得到。", Text: text}
}

// handoff 是交回给 agent 的那句话：过了让这个会话自己提交，不过让它按意见返工。
func handoff(repo, verdict, opinion, title string) string {
	base := baseline(repo)
	if verdict == "back" {
		opinion = strings.TrimSpace(opinion)
		if opinion == "" {
			opinion = "没说哪里不对"
		}
		opinion = sentence(opinion)
		return "评审没通过：" + opinion + "参照 " + base + "。改完后停下，等下一次评审。"
	}
	title = strings.TrimSpace(title)
	if title == "" {
		title = "本次评审"
	}
	return "评审通过。请在这个会话里提交。参照 " + base + "。提交说明：" + title + "。只提交这个会话写过的文件。"
}

// sentence 补句号。人写意见时不看标点，交回的话要成句。
func sentence(text string) string {
	r, _ := utf8.DecodeLastRuneInString(text)
	if strings.ContainsRune("。！？.!?", r) {
		return text
	}
	return text + "。"
}

func (a *App) pick(asked string) string {
	if strings.TrimSpace(asked) != "" {
		return history.Canon(asked)
	}
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return history.Canon(wd)
}

func buildSession(repo, base string, past history.Past, idx *graph.Index, raw *sess.RawSession) model.SessionOut {
	out := model.SessionOut{
		Agent: displayAgent(past.Agent), Sid: past.Sid,
		Baseline: base, Bound: "start", Source: "没找到这个会话的记录文件",
		Turns: []model.TurnOut{}, Net: blankTurn("net"),
	}
	if past.Agent == "codex" {
		out.Bound = "late"
	}
	if raw == nil {
		return out
	}
	out.Source = raw.Source
	files := replay.Replay(repo, *raw)
	for i := len(raw.Turns) - 1; i >= 0; i-- {
		t := raw.Turns[i]
		edits := editsOf(repo, t.Edits)
		broken, errOut := errorOuts(files[i].Errors)
		stars, calls, chains := []model.StarOut{}, []model.CallOut{}, []model.ChainOut{}
		omitted := 0
		if len(edits) > 0 {
			g := graph.Analyze(idx, files[i].Files, broken)
			stars, calls, chains, omitted = g.Stars, g.Calls, g.Chains, g.Omitted
		}
		out.Turns = append(out.Turns, model.TurnOut{
			ID: "t" + strconv.Itoa(i+1), At: clock(t.At), Prompt: t.Prompt, Reply: t.Reply,
			Edits: edits, Files: fileBodies(files[i].Files), Stars: stars, Calls: calls, Chains: chains, Omitted: omitted, Errors: errOut,
		})
	}
	if out.Title == "" && len(raw.Turns) > 0 {
		out.Title = raw.Turns[0].Prompt
	}
	folded := replay.FoldSession(files)
	broken, errOut := errorOuts(folded.Errors)
	g := graph.Analyze(idx, folded.Files, broken)
	netEdits := make([]model.EditOut, 0)
	for _, t := range raw.Turns {
		netEdits = append(netEdits, editsOf(repo, t.Edits)...)
	}
	marked := 0
	for _, s := range g.Stars {
		if s.Mark != nil {
			marked++
		}
	}
	log.Printf("会话 %s 净改：%d 个文件，改动方法 %d，全链 %d，对不上 %d", out.Sid, len(folded.Files), marked, len(g.Chains), len(folded.Errors))
	at := ""
	if len(out.Turns) > 0 {
		at = out.Turns[0].At
	}
	out.Net = model.TurnOut{
		ID: "net", At: at, Edits: netEdits, Files: fileBodies(folded.Files), Stars: g.Stars, Calls: g.Calls, Chains: g.Chains, Omitted: g.Omitted, Errors: errOut,
	}
	return out
}

// sketch 只给出选择空间要用的标题、时间和改过的文件，不复原正文，也不编链。
// 回合都列出来，没有写过文件的回合也在，对话流和回合条要靠它们。
func sketch(repo, base string, past history.Past, raw *sess.RawSession) model.SessionOut {
	out := model.SessionOut{
		Agent: displayAgent(past.Agent), Sid: past.Sid,
		Baseline: base, Bound: "start", Source: "没找到这个会话的记录文件",
		Turns: []model.TurnOut{}, Net: blankTurn("net"),
	}
	if past.Agent == "codex" {
		out.Bound = "late"
	}
	if raw == nil {
		return out
	}
	out.Source = raw.Source
	if len(raw.Turns) > 0 {
		out.Title = raw.Turns[0].Prompt
	}
	for i := len(raw.Turns) - 1; i >= 0; i-- {
		t := raw.Turns[i]
		out.Turns = append(out.Turns, model.TurnOut{
			ID: "t" + strconv.Itoa(i+1), At: clock(t.At), Prompt: t.Prompt, Reply: t.Reply, Edits: editsOf(repo, t.Edits),
			Stars: []model.StarOut{}, Calls: []model.CallOut{}, Chains: []model.ChainOut{}, Errors: []model.ErrorOut{},
		})
	}
	netEdits := make([]model.EditOut, 0)
	for _, t := range raw.Turns {
		netEdits = append(netEdits, editsOf(repo, t.Edits)...)
	}
	at := ""
	if len(out.Turns) > 0 {
		at = out.Turns[0].At
	}
	out.Net = blankTurn("net")
	out.Net.At = at
	out.Net.Edits = netEdits
	return out
}

func fileBodies(files map[string][2]*string) []model.FileOut {
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	out := make([]model.FileOut, 0, len(paths))
	for _, path := range paths {
		pair := files[path]
		out = append(out, model.FileOut{Path: path, Before: textOf(pair[0]), After: textOf(pair[1])})
	}
	return out
}

func textOf(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func editsOf(repo string, raw []sess.RawEdit) []model.EditOut {
	out := make([]model.EditOut, 0)
	for _, e := range raw {
		path, ok := replay.Rel(repo, e.Path)
		if !ok {
			continue
		}
		out = append(out, model.EditOut{Tool: e.Tool, Path: path, Ok: e.Ok, Reason: e.Reason})
	}
	return out
}

func errorOuts(errs [][2]string) ([]string, []model.ErrorOut) {
	broken := make([]string, 0, len(errs))
	out := make([]model.ErrorOut, 0, len(errs))
	for _, e := range errs {
		broken = append(broken, e[0])
		out = append(out, model.ErrorOut{Path: e[0], Reason: e[1]})
	}
	return broken, out
}

func blankTurn(id string) model.TurnOut {
	return model.TurnOut{
		ID: id, Edits: []model.EditOut{}, Stars: []model.StarOut{}, Calls: []model.CallOut{},
		Chains: []model.ChainOut{}, Errors: []model.ErrorOut{},
	}
}

func projectPaths(list []history.Project) []string {
	out := make([]string, 0, len(list))
	for _, p := range list {
		out = append(out, p.Path)
	}
	return out
}

func projectOuts(list []history.Project) []model.ProjectOut {
	out := make([]model.ProjectOut, 0, len(list))
	for _, p := range list {
		out = append(out, model.ProjectOut{
			Path: p.Path, Claude: p.Claude, Codex: p.Codex, Grok: p.Grok, Latest: clockTime(p.Latest),
		})
	}
	return out
}

func baseline(repo string) string {
	out, err := exec.Command("git", "-C", repo, "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return "不是 git 仓库"
	}
	s := strings.TrimSpace(string(out))
	if s == "" {
		return "不是 git 仓库"
	}
	return s
}

func clock(ts string) string {
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return ""
	}
	local := t.In(time.Local)
	now := time.Now()
	if local.Year() == now.Year() && local.YearDay() == now.YearDay() {
		return local.Format("15:04")
	}
	return local.Format("01-02 15:04")
}

func clockTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.In(time.Local).Format("01-02 15:04")
}

func displayAgent(agent string) string {
	if agent == "" {
		return ""
	}
	r, size := utf8.DecodeRuneInString(agent)
	return string(unicode.ToUpper(r)) + agent[size:]
}

func nonempty(out *model.Output) {
	if out.Repos == nil {
		out.Repos = []string{}
	}
	if out.Projects == nil {
		out.Projects = []model.ProjectOut{}
	}
	if out.Sessions == nil {
		out.Sessions = []model.SessionOut{}
	}
	if out.Notes == nil {
		out.Notes = []string{}
	}
	for i := range out.Sessions {
		if out.Sessions[i].Turns == nil {
			out.Sessions[i].Turns = []model.TurnOut{}
		}
		fillTurn(&out.Sessions[i].Net)
		for j := range out.Sessions[i].Turns {
			fillTurn(&out.Sessions[i].Turns[j])
		}
	}
}

func fillSession(s *model.SessionOut) {
	if s.Turns == nil {
		s.Turns = []model.TurnOut{}
	}
	fillTurn(&s.Net)
	for i := range s.Turns {
		fillTurn(&s.Turns[i])
	}
}

func fillTurn(t *model.TurnOut) {
	if t.Edits == nil {
		t.Edits = []model.EditOut{}
	}
	if t.Stars == nil {
		t.Stars = []model.StarOut{}
	}
	if t.Calls == nil {
		t.Calls = []model.CallOut{}
	}
	if t.Chains == nil {
		t.Chains = []model.ChainOut{}
	}
	if t.Errors == nil {
		t.Errors = []model.ErrorOut{}
	}
}

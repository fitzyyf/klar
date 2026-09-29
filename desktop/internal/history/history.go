// 已经关掉的会话。从三家的记录目录里按仓库找。
package history

import (
	"bufio"
	"encoding/json"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type Past struct {
	Agent string
	Sid   string
	When  time.Time
}

type Project struct {
	Path   string
	Claude uint32
	Codex  uint32
	Grok   uint32
	Latest time.Time
}

// ClaudeKey 是 Claude 项目目录名：路径里每个非字母数字都换成 -。
func ClaudeKey(path string) string {
	var b strings.Builder
	for _, c := range path {
		if c < 128 && isAlnum(byte(c)) {
			b.WriteRune(c)
		} else {
			b.WriteByte('-')
		}
	}
	return b.String()
}

// GrokKey 是 Grok 会话目录名：绝对路径按百分号编码。
func GrokKey(path string) string {
	var b strings.Builder
	for _, c := range []byte(path) {
		if isAlnum(c) || c == '-' || c == '_' || c == '.' || c == '~' {
			b.WriteByte(c)
			continue
		}
		const hex = "0123456789ABCDEF"
		b.WriteByte('%')
		b.WriteByte(hex[c>>4])
		b.WriteByte(hex[c&0x0f])
	}
	return b.String()
}

func isAlnum(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')
}

// PercentDecode 把 Grok 目录名还原成路径。
func PercentDecode(raw string) string {
	b := []byte(raw)
	out := make([]byte, 0, len(b))
	for i := 0; i < len(b); {
		if b[i] == '%' && i+2 < len(b) {
			if v, err := strconv.ParseUint(string(b[i+1:i+3]), 16, 8); err == nil {
				out = append(out, byte(v))
				i += 3
				continue
			}
		}
		out = append(out, b[i])
		i++
	}
	if !utf8.Valid(out) {
		return raw
	}
	return string(out)
}

// Canon 尽量解析成绝对路径。目录不存在时退回绝对路径。
func Canon(path string) string {
	if path == "" {
		return ""
	}
	if p, err := filepath.EvalSymlinks(path); err == nil {
		return p
	}
	if p, err := filepath.Abs(path); err == nil {
		return p
	}
	return path
}

func mtime(path string) time.Time {
	st, err := os.Stat(path)
	if err != nil {
		return time.Unix(0, 0)
	}
	return st.ModTime()
}

func push(out *[]Past, live map[string]struct{}, agent, sid string, when time.Time) {
	if sid == "" {
		return
	}
	if _, ok := live[sid]; ok {
		return
	}
	for _, p := range *out {
		if p.Sid == sid {
			return
		}
	}
	*out = append(*out, Past{Agent: agent, Sid: sid, When: when})
}

// Closed 列出这个仓库里已经关掉的会话，最多 80 个，新的在前。
func Closed(home, repo string, live map[string]struct{}) []Past {
	repo = Canon(repo)
	var out []Past
	claudeClosed(home, repo, live, &out)
	grokClosed(home, repo, live, &out)
	codexClosed(home, repo, live, &out)
	sort.Slice(out, func(i, j int) bool { return out[i].When.After(out[j].When) })
	if len(out) > 80 {
		out = out[:80]
	}
	if len(out) > 0 {
		log.Printf("仓库里还有 %d 个已经关掉的会话", len(out))
	}
	return out
}

func claudeClosed(home, repo string, live map[string]struct{}, out *[]Past) {
	dir := filepath.Join(home, ".claude", "projects", ClaudeKey(repo))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		name := e.Name()
		sid, ok := strings.CutSuffix(name, ".jsonl")
		if !ok {
			continue
		}
		push(out, live, "claude", sid, mtime(filepath.Join(dir, name)))
	}
}

func grokClosed(home, repo string, live map[string]struct{}, out *[]Past) {
	dir := filepath.Join(home, ".grok", "sessions", GrokKey(repo))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		logPath := filepath.Join(dir, e.Name(), "chat_history.jsonl")
		if st, err := os.Stat(logPath); err != nil || st.IsDir() {
			continue
		}
		push(out, live, "grok", e.Name(), mtime(logPath))
	}
}

func codexClosed(home, repo string, live map[string]struct{}, out *[]Past) {
	want := Canon(repo)
	for _, path := range newestJSONL(filepath.Join(home, ".codex", "sessions"), 400) {
		v := firstJSON(path)
		d := obj(v)
		if str(d["type"]) != "session_meta" {
			continue
		}
		payload := obj(d["payload"])
		cwd := str(payload["cwd"])
		if cwd == "" || Canon(cwd) != want {
			continue
		}
		sid := str(payload["session_id"])
		if sid == "" {
			sid = str(payload["id"])
		}
		push(out, live, "codex", sid, mtime(path))
	}
}

// Projects 是三家记录里出现过的项目，新的在前。
func Projects(home string) []Project {
	mapOf := map[string]*Project{}
	claudeProjects(home, mapOf)
	grokProjects(home, mapOf)
	codexProjects(home, mapOf)
	out := make([]Project, 0, len(mapOf))
	var claudeN uint32
	for _, p := range mapOf {
		out = append(out, *p)
		claudeN += p.Claude
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Latest.After(out[j].Latest) })
	log.Printf("有会话的项目 %d 个，其中 Claude 会话 %d", len(out), claudeN)
	return out
}

func addProject(mapOf map[string]*Project, path, agent string, n uint32, when time.Time) {
	if path == "" || n == 0 {
		return
	}
	path = Canon(path)
	slot := mapOf[path]
	if slot == nil {
		slot = &Project{Path: path}
		mapOf[path] = slot
	}
	switch agent {
	case "claude":
		slot.Claude += n
	case "codex":
		slot.Codex += n
	case "grok":
		slot.Grok += n
	}
	if when.After(slot.Latest) {
		slot.Latest = when
	}
}

func claudeProjects(home string, mapOf map[string]*Project) {
	root := filepath.Join(home, ".claude", "projects")
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(root, e.Name())
		files, err := filepath.Glob(filepath.Join(dir, "*.jsonl"))
		if err != nil || len(files) == 0 {
			continue
		}
		sort.Slice(files, func(i, j int) bool { return mtime(files[i]).After(mtime(files[j])) })
		cwd := cwdOf(files[0])
		if cwd == "" {
			continue
		}
		addProject(mapOf, cwd, "claude", uint32(len(files)), mtime(files[0]))
	}
}

func grokProjects(home string, mapOf map[string]*Project) {
	root := filepath.Join(home, ".grok", "sessions")
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	for _, e := range entries {
		dir := filepath.Join(root, e.Name())
		subs, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		var n uint32
		latest := time.Unix(0, 0)
		for _, sub := range subs {
			logPath := filepath.Join(dir, sub.Name(), "chat_history.jsonl")
			if st, err := os.Stat(logPath); err != nil || st.IsDir() {
				continue
			}
			n++
			when := mtime(logPath)
			if when.After(latest) {
				latest = when
			}
		}
		addProject(mapOf, PercentDecode(e.Name()), "grok", n, latest)
	}
}

func codexProjects(home string, mapOf map[string]*Project) {
	for _, path := range newestJSONL(filepath.Join(home, ".codex", "sessions"), 400) {
		d := obj(firstJSON(path))
		if str(d["type"]) != "session_meta" {
			continue
		}
		addProject(mapOf, str(obj(d["payload"])["cwd"]), "codex", 1, mtime(path))
	}
}

func newestJSONL(root string, limit int) []string {
	st, err := os.Stat(root)
	if err != nil || !st.IsDir() {
		return nil
	}
	type item struct {
		when time.Time
		path string
	}
	var files []item
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d == nil || d.IsDir() || !strings.HasSuffix(d.Name(), ".jsonl") {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		files = append(files, item{info.ModTime(), path})
		return nil
	})
	sort.Slice(files, func(i, j int) bool { return files[i].when.After(files[j].when) })
	if len(files) > limit {
		files = files[:limit]
	}
	out := make([]string, len(files))
	for i, f := range files {
		out[i] = f.path
	}
	return out
}

func firstJSON(path string) any {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	r := bufio.NewReaderSize(f, 64*1024)
	line, err := r.ReadBytes('\n')
	if err != nil && err != io.EOF {
		return nil
	}
	var v any
	if json.Unmarshal(line, &v) != nil {
		return nil
	}
	return v
}

func cwdOf(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 4<<20)
	for i := 0; i < 31 && sc.Scan(); i++ {
		line := sc.Text()
		if !strings.Contains(line, `"cwd"`) {
			continue
		}
		var v any
		if json.Unmarshal(sc.Bytes(), &v) != nil {
			continue
		}
		if cwd := str(obj(v)["cwd"]); cwd != "" {
			return cwd
		}
	}
	return ""
}

func obj(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func str(v any) string {
	s, _ := v.(string)
	return s
}

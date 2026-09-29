package sess

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Locate 按三家自己的目录找到这一条会话记录。
func Locate(home, agent, sid string) string {
	switch agent {
	case "claude":
		root := filepath.Join(home, ".claude", "projects")
		entries, err := os.ReadDir(root)
		if err != nil {
			return ""
		}
		name := sid + ".jsonl"
		for _, e := range entries {
			p := filepath.Join(root, e.Name(), name)
			if st, err := os.Stat(p); err == nil && !st.IsDir() {
				return p
			}
		}
	case "codex":
		return newestCodex(home, sid)
	case "grok":
		root := filepath.Join(home, ".grok", "sessions")
		entries, err := os.ReadDir(root)
		if err != nil {
			return ""
		}
		for _, e := range entries {
			p := filepath.Join(root, e.Name(), sid, "chat_history.jsonl")
			if st, err := os.Stat(p); err == nil && !st.IsDir() {
				return p
			}
		}
	}
	return ""
}

func newestCodex(home, sid string) string {
	root := filepath.Join(home, ".codex", "sessions")
	if st, err := os.Stat(root); err != nil || !st.IsDir() {
		return ""
	}
	suffix := sid + ".jsonl"
	var best string
	var bestAt time.Time
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d == nil || d.IsDir() {
			return nil
		}
		name := d.Name()
		if !strings.HasPrefix(name, "rollout-") || !strings.HasSuffix(name, suffix) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if best == "" || info.ModTime().After(bestAt) {
			best = path
			bestAt = info.ModTime()
		}
		return nil
	})
	return best
}

// Read 读出回合和写文件记录。找不到就返回 nil。
func Read(home, agent, sid string) *RawSession {
	path := Locate(home, agent, sid)
	if path == "" {
		log.Printf("没找到 %s 会话 %s 的记录", agent, sid)
		return nil
	}
	return Parse(agent, path)
}

// Parse 从已经定位的记录文件读出回合。调用方先 Locate，好顺便记录修改时间做缓存。
func Parse(agent, path string) *RawSession {
	var turns []RawTurn
	switch agent {
	case "claude":
		turns = claude(path)
	case "codex":
		turns = codex(path)
	case "grok":
		turns = grok(path)
	default:
		return nil
	}
	n := 0
	for _, t := range turns {
		n += len(t.Edits)
	}
	log.Printf("读到 %s 会话记录 %s：%d 回合，%d 次写文件", agent, path, len(turns), n)
	// Source 只是给人看的路径，这里自己找一次家目录缩成 ~。
	home, _ := os.UserHomeDir()
	return &RawSession{Source: tilde(home, path), Turns: turns}
}

func tilde(home, path string) string {
	if home != "" && strings.HasPrefix(path, home) {
		return "~" + strings.TrimPrefix(path, home)
	}
	return path
}

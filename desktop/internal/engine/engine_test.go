package engine

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"klar.dev/desktop/internal/history"
	"klar.dev/desktop/internal/model"
	"klar.dev/desktop/internal/store"
)

func TestLoadBuildsChainAndVerdictStaysLocal(t *testing.T) {
	dir := t.TempDir()
	home := filepath.Join(dir, "home")
	repo := filepath.Join(dir, "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	src := "function create(order) {\n  charge();\n}\nfunction charge() {}\nfunction reserve() {}\n"
	if err := os.WriteFile(filepath.Join(repo, "a.js"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	repo = history.Canon(repo)
	sessDir := filepath.Join(home, ".claude", "projects", history.ClaudeKey(repo))
	if err := os.MkdirAll(sessDir, 0o755); err != nil {
		t.Fatal(err)
	}
	rows := []any{
		map[string]any{
			"type": "user", "timestamp": "2026-09-27T01:00:00Z",
			"message": map[string]any{"content": []any{map[string]any{"type": "text", "text": "改价格判断"}}},
		},
		map[string]any{
			"type": "assistant",
			"message": map[string]any{"content": []any{
				map[string]any{"type": "text", "text": "加上判断"},
				map[string]any{
					"type": "tool_use", "id": "t1", "name": "Edit",
					"input": map[string]any{
						"file_path":  filepath.Join(repo, "a.js"),
						"old_string": "charge();",
						"new_string": "if (order.price < 0) {\n    reserve(order);\n  }",
					},
				},
			}},
		},
		map[string]any{
			"type": "user",
			"message": map[string]any{"content": []any{
				map[string]any{"type": "tool_result", "tool_use_id": "t1", "content": "ok"},
			}},
			"toolUseResult": map[string]any{"originalFile": src},
		},
		map[string]any{
			"type": "user", "timestamp": "2026-09-27T01:05:00Z",
			"message": map[string]any{"content": []any{map[string]any{"type": "text", "text": "再检查一下别处有没有类似写法"}}},
		},
		map[string]any{
			"type": "assistant",
			"message": map[string]any{"content": []any{
				map[string]any{"type": "text", "text": "检查过了，没有别的写法要改。"},
			}},
		},
	}
	f, err := os.Create(filepath.Join(sessDir, "sess1.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	enc := json.NewEncoder(f)
	for _, row := range rows {
		if err := enc.Encode(row); err != nil {
			t.Fatal(err)
		}
	}
	f.Close()

	app := New()
	app.home = home
	app.db = filepath.Join(dir, "reviews.sqlite")
	out, err := app.Load(context.Background(), repo)
	if err != nil {
		t.Fatal(err)
	}
	if out.Repo != repo || len(out.Sessions) != 1 {
		t.Fatalf("repo=%s sessions=%d", out.Repo, len(out.Sessions))
	}
	s := out.Sessions[0]
	if s.Agent != "Claude" || s.Sid != "sess1" {
		t.Fatalf("%+v", s)
	}
	if len(s.Turns) != 2 || !s.Turns[1].Edits[0].Ok || s.Turns[1].Edits[0].Path != "a.js" || s.Assembled || hasPrice(s.Net, s.Turns) {
		t.Fatalf("列表不该编链 %#v", s.Turns)
	}
	// 没写过文件的回合也在列表里，对话流和回合条靠它。新的在前，t2 是有编辑的那回合。
	if s.Turns[0].ID != "t2" || s.Turns[1].ID != "t1" || s.Turns[0].Prompt != "再检查一下别处有没有类似写法" {
		t.Fatalf("回合没按新的在前 %#v", s.Turns)
	}
	full, err := app.Open(context.Background(), repo, "Claude", "sess1")
	if err != nil {
		t.Fatal(err)
	}
	if !full.Assembled || !hasPrice(full.Net, full.Turns) {
		t.Fatal("选中后没有组装出价格判断")
	}
	// 无编辑回合不编链但有对话；有编辑回合照旧编链。
	if len(full.Turns) != 2 || len(full.Turns[0].Stars) != 0 || full.Turns[0].Reply == nil || !hasPrice(full.Turns[1], nil) {
		t.Fatalf("组装后的回合不对 %#v", full.Turns)
	}
	if len(full.Net.Files) != 1 || full.Net.Files[0].Path != "a.js" || !strings.Contains(full.Net.Files[0].After, "if") {
		t.Fatalf("文件没复原 %#v", full.Net.Files)
	}

	// 界面记录的是评审时的最新回合 t2，并把要粘给 agent 的话一并交回。
	sent := app.Deliver(repo, "sess1", "back", "价格判断写反了", "改价格判断", "t2")
	if !sent.Ok {
		t.Fatalf("%+v", sent)
	}
	if !strings.HasPrefix(sent.Text, "评审没通过：价格判断写反了。参照 ") ||
		!strings.Contains(sent.Text, "改完后停下，等下一次评审。") {
		t.Fatalf("交回的话不对：%s", sent.Text)
	}
	saved, err := store.Latest(app.db, repo)
	if err != nil {
		t.Fatal(err)
	}
	if saved["sess1"].Verdict != "back" || saved["sess1"].TurnID != "t2" || saved["sess1"].Text != sent.Text {
		t.Fatalf("%+v", saved["sess1"])
	}
	again, err := app.Load(context.Background(), repo)
	if err != nil {
		t.Fatal(err)
	}
	rev := again.Sessions[0].Review
	if rev == nil || rev.Verdict != "back" || !rev.Current {
		t.Fatalf("%+v", rev)
	}
}

func hasPrice(net model.TurnOut, turns []model.TurnOut) bool {
	for _, turn := range append([]model.TurnOut{net}, turns...) {
		for _, star := range turn.Stars {
			if star.Kind == "if" {
				return true
			}
		}
		for _, call := range turn.Calls {
			if call.Guard != nil && strings.Contains(*call.Guard, "price") {
				return true
			}
		}
	}
	return false
}

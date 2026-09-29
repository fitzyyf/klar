package sess

import (
	"encoding/json"
	"path/filepath"
	"strings"
)

type grokSnap struct {
	at     string
	before Snapshot
	after  Snapshot
}

func grok(path string) []RawTurn {
	dir := filepath.Dir(path)
	snaps := map[int64]grokSnap{}
	for _, raw := range parseJSONL(filepath.Join(dir, "rewind_points.jsonl")) {
		d := obj(raw)
		idx, ok := asInt(d["prompt_index"])
		if !ok {
			continue
		}
		snaps[idx] = grokSnap{
			at:     str(d["created_at"]),
			before: takeSnap(d["file_snapshots"]),
			after:  takeSnap(d["after_snapshots"]),
		}
	}
	var turns []RawTurn
	pending := map[string][2]int{}
	for _, raw := range parseJSONL(path) {
		d := obj(raw)
		switch str(d["type"]) {
		case "user":
			idx, ok := asInt(d["prompt_index"])
			if !ok {
				continue
			}
			turn := RawTurn{Prompt: firstLine(textOf(d["content"]))}
			if snap, ok := snaps[idx]; ok {
				turn.At = snap.at
				if len(snap.before) > 0 || len(snap.after) > 0 {
					turn.SnapBefore = snap.before
					turn.SnapAfter = snap.after
				}
			}
			turns = append(turns, turn)
		case "assistant":
			if len(turns) == 0 {
				continue
			}
			ti := len(turns) - 1
			if r := shortReply(str(d["content"])); r != nil {
				turns[ti].Reply = r
			}
			for _, call := range arr(d["tool_calls"]) {
				cm := obj(call)
				var args any
				_ = json.Unmarshal([]byte(str(cm["arguments"])), &args)
				am := obj(args)
				name := str(cm["name"])
				var op Op
				switch name {
				case "write":
					op = Write(str(am["content"]))
				case "search_replace":
					op = Replace(str(am["old_string"]), str(am["new_string"]), boolean(am["replace_all"]))
				default:
					continue
				}
				why := "没有找到工具结果"
				turns[ti].Edits = append(turns[ti].Edits, RawEdit{
					Tool: name, Path: str(am["file_path"]), Op: op, Reason: &why,
				})
				if id := str(cm["id"]); id != "" {
					pending[id] = [2]int{ti, len(turns[ti].Edits) - 1}
				}
			}
		case "tool_result":
			at, ok := pending[str(d["tool_call_id"])]
			if !ok {
				continue
			}
			text := textOf(d["content"])
			edit := &turns[at[0]].Edits[at[1]]
			edit.Ok = strings.Contains(text, "has been created") || strings.Contains(text, "successfully")
			if edit.Ok {
				edit.Reason = nil
			} else {
				msg := "工具报错：" + clip(text, 120)
				edit.Reason = &msg
			}
		}
	}
	return turns
}

func takeSnap(v any) Snapshot {
	m := obj(v)
	if len(m) == 0 {
		return Snapshot{}
	}
	out := Snapshot{}
	for k, raw := range m {
		if s, ok := obj(raw)["content"].(string); ok {
			p := s
			out[k] = &p
		} else {
			out[k] = nil
		}
	}
	return out
}

package sess

func codex(path string) []RawTurn {
	var turns []RawTurn
	open := false
	for _, raw := range parseJSONL(path) {
		d := obj(raw)
		if str(d["type"]) != "event_msg" {
			continue
		}
		p := obj(d["payload"])
		ts := str(d["timestamp"])
		switch str(p["type"]) {
		case "task_started":
			turns = append(turns, RawTurn{At: ts})
			open = true
		case "task_complete", "turn_aborted":
			open = false
		case "item_completed":
			if !open && len(turns) == 0 {
				turns = append(turns, RawTurn{At: ts})
			}
			if len(turns) == 0 {
				continue
			}
			turn := &turns[len(turns)-1]
			item := obj(p["item"])
			switch str(item["type"]) {
			case "UserMessage":
				if turn.Prompt == "" {
					turn.Prompt = firstLine(textOf(item["content"]))
				}
			case "AgentMessage":
				if r := shortReply(textOf(item["content"])); r != nil {
					turn.Reply = r
				}
			case "FileChange":
				status, isStr := item["status"].(string)
				failed := isStr && status != "completed"
				changes := obj(item["changes"])
				for file, rawCh := range changes {
					ch := obj(rawCh)
					var op Op
					switch str(ch["type"]) {
					case "add":
						op = Add(str(ch["content"]))
					case "delete":
						op = Delete(str(ch["content"]))
					case "update":
						op = Update(str(ch["unified_diff"]))
					default:
						continue
					}
					path := str(ch["move_path"])
					if path == "" {
						path = file
					}
					var reason *string
					if failed {
						msg := "补丁状态：" + status
						reason = &msg
					}
					turn.Edits = append(turn.Edits, RawEdit{
						Tool: "apply_patch", Path: path, Op: op, Ok: !failed, Reason: reason,
					})
				}
			}
		}
	}
	kept := turns[:0]
	for _, t := range turns {
		if t.Prompt == "" && len(t.Edits) == 0 {
			continue
		}
		if t.Prompt == "" {
			t.Prompt = "（没有取到指令）"
		}
		kept = append(kept, t)
	}
	return kept
}

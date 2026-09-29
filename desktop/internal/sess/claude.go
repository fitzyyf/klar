package sess

import "strings"

// 这些开头是 Claude 塞进来的系统话，不当成用户的一回合。
var injected = []string{
	"<task-notification>", "<command-", "<local-command", "<system-reminder>", "<bash-", "Caveat:",
}

func claude(path string) []RawTurn {
	var turns []RawTurn
	pending := map[string][2]int{}
	for _, raw := range parseJSONL(path) {
		d := obj(raw)
		if boolean(d["isSidechain"]) {
			continue
		}
		kind := str(d["type"])
		content := obj(d["message"])["content"]
		if kind == "user" {
			var results []any
			for _, item := range arr(content) {
				if str(obj(item)["type"]) == "tool_result" {
					results = append(results, item)
				}
			}
			if len(results) == 0 {
				if boolean(d["isMeta"]) {
					continue
				}
				text := textOf(content)
				t := trimStart(text)
				if t == "" || startsAny(t, injected) || strings.HasPrefix(t, "[Request interrupted") {
					continue
				}
				prompt := firstLine(text)
				if strings.HasPrefix(t, "This session is being continued") {
					prompt = "（上下文压缩后继续）"
				}
				turns = append(turns, RawTurn{At: str(d["timestamp"]), Prompt: prompt})
				continue
			}
			for _, r := range results {
				rm := obj(r)
				at, ok := pending[str(rm["tool_use_id"])]
				if !ok {
					continue
				}
				edit := &turns[at[0]].Edits[at[1]]
				if boolean(rm["is_error"]) {
					edit.Ok = false
					msg := "工具报错：" + clip(textOf(rm["content"]), 120)
					edit.Reason = &msg
					continue
				}
				edit.Ok = true
				edit.Reason = nil
				res, _ := d["toolUseResult"].(map[string]any)
				if res == nil {
					continue
				}
				orig, exists := res["originalFile"]
				if !exists {
					continue
				}
				switch s := orig.(type) {
				case string:
					edit.Base = presentBase(s)
				case nil:
					if edit.Tool == "Write" && str(res["type"]) == "create" {
						edit.Base = missingBase()
					}
				}
			}
			continue
		}
		if kind != "assistant" {
			continue
		}
		items := arr(content)
		if items == nil {
			continue
		}
		if len(turns) == 0 {
			turns = append(turns, RawTurn{Prompt: "（会话开头）"})
		}
		ti := len(turns) - 1
		for _, item := range items {
			x := obj(item)
			switch str(x["type"]) {
			case "text":
				if r := shortReply(str(x["text"])); r != nil {
					turns[ti].Reply = r
				}
			case "tool_use":
				input := obj(x["input"])
				name := str(x["name"])
				var op Op
				switch name {
				case "Write":
					op = Write(str(input["content"]))
				case "Edit":
					op = Replace(str(input["old_string"]), str(input["new_string"]), boolean(input["replace_all"]))
				case "MultiEdit":
					var list [][3]string
					for _, e := range arr(input["edits"]) {
						em := obj(e)
						all := ""
						if boolean(em["replace_all"]) {
							all = "1"
						}
						list = append(list, [3]string{str(em["old_string"]), str(em["new_string"]), all})
					}
					op = Multi(list)
				default:
					continue
				}
				why := "没有找到工具结果"
				turns[ti].Edits = append(turns[ti].Edits, RawEdit{
					Tool: name, Path: str(input["file_path"]), Op: op, Reason: &why,
				})
				if id := str(x["id"]); id != "" {
					pending[id] = [2]int{ti, len(turns[ti].Edits) - 1}
				}
			}
		}
	}
	return turns
}

func startsAny(s string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

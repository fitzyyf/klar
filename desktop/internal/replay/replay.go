// 把一个会话的编辑叠成每个回合开始、结束时的正文。只信会话记录。
package replay

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"klar.dev/desktop/internal/sess"
)

type Content = *string

type TurnFiles struct {
	Files  map[string][2]Content
	Errors [][2]string
}

func Rel(repo, file string) (string, bool) {
	p := file
	if !filepath.IsAbs(p) {
		p = filepath.Join(repo, p)
	}
	rel, err := filepath.Rel(repo, p)
	if err != nil || rel == "" || rel == "." || strings.HasPrefix(rel, "..") {
		return "", false
	}
	return filepath.ToSlash(rel), true
}

func replace(text, old, new string, all bool) (string, bool) {
	if old == "" || !strings.Contains(text, old) {
		return "", false
	}
	if all {
		return strings.ReplaceAll(text, old, new), true
	}
	return strings.Replace(text, old, new, 1), true
}

func unreplace(text, old, new string, all bool) (string, bool) {
	if new == "" || !strings.Contains(text, new) {
		return "", false
	}
	if all {
		return strings.ReplaceAll(text, new, old), true
	}
	at := strings.Index(text, new)
	return text[:at] + old + text[at+len(new):], true
}

type hunk struct{ old, new []string }

// linesOf 对齐 Rust 的 str::lines：丢掉结尾换行带出来的空行，中间空行留着。
func linesOf(text string) []string {
	if text == "" {
		return nil
	}
	parts := strings.Split(text, "\n")
	if strings.HasSuffix(text, "\n") {
		parts = parts[:len(parts)-1]
	}
	for i := range parts {
		parts[i] = strings.TrimSuffix(parts[i], "\r")
	}
	return parts
}

func hunks(diff string) []hunk {
	var out []hunk
	for _, line := range linesOf(diff) {
		if strings.HasPrefix(line, "@@") {
			out = append(out, hunk{})
			continue
		}
		if strings.HasPrefix(line, "---") || strings.HasPrefix(line, "+++") || strings.HasPrefix(line, `\`) {
			continue
		}
		if len(out) == 0 {
			continue
		}
		h := &out[len(out)-1]
		if line == "" {
			h.old = append(h.old, "")
			h.new = append(h.new, "")
			continue
		}
		body := line[1:]
		switch line[0] {
		case ' ':
			h.old = append(h.old, body)
			h.new = append(h.new, body)
		case '-':
			h.old = append(h.old, body)
		case '+':
			h.new = append(h.new, body)
		}
	}
	return out
}

func swapBlock(text string, from, to []string) (string, bool) {
	trailing := strings.HasSuffix(text, "\n")
	lines := linesOf(text)
	if len(from) == 0 {
		lines = append(lines, to...)
	} else {
		at := -1
		for i := 0; i+len(from) <= len(lines); i++ {
			ok := true
			for j := range from {
				if lines[i+j] != from[j] {
					ok = false
					break
				}
			}
			if ok {
				at = i
				break
			}
		}
		if at < 0 {
			return "", false
		}
		var next []string
		next = append(next, lines[:at]...)
		next = append(next, to...)
		next = append(next, lines[at+len(from):]...)
		lines = next
	}
	out := strings.Join(lines, "\n")
	if trailing || text == "" {
		out += "\n"
	}
	return out, true
}

func applyDiff(text, diff string, reverse bool) (string, bool) {
	list := hunks(diff)
	cur := text
	var err bool
	step := func(h hunk, rev bool) {
		if err {
			return
		}
		var ok bool
		if rev {
			cur, ok = swapBlock(cur, h.new, h.old)
		} else {
			cur, ok = swapBlock(cur, h.old, h.new)
		}
		if !ok {
			err = true
		}
	}
	if reverse {
		for i := len(list) - 1; i >= 0; i-- {
			step(list[i], true)
		}
	} else {
		for _, h := range list {
			step(h, false)
		}
	}
	return cur, !err
}

func forward(before Content, op sess.Op) (Content, string) {
	need := func() (string, bool) {
		if before == nil {
			return "", false
		}
		return *before, true
	}
	switch op.Kind {
	case "write", "add":
		return &op.Text, ""
	case "delete":
		return nil, ""
	case "replace":
		cur, ok := need()
		if !ok {
			return nil, "文件当时不存在"
		}
		next, ok := replace(cur, op.Old, op.New, op.All)
		if !ok {
			return nil, "old_string 在当时的正文里找不到"
		}
		return &next, ""
	case "multi":
		cur, ok := need()
		if !ok {
			return nil, "文件当时不存在"
		}
		for i, item := range op.List {
			next, ok := replace(cur, item[0], item[1], item[2] == "1")
			if !ok {
				return nil, "MultiEdit 第 " + itoa(i+1) + " 条的 old_string 找不到"
			}
			cur = next
		}
		return &cur, ""
	case "update":
		cur, ok := need()
		if !ok {
			return nil, "文件当时不存在"
		}
		next, ok := applyDiff(cur, op.Diff, false)
		if !ok {
			return nil, "补丁里要换的行在当时的正文里找不到"
		}
		return &next, ""
	default:
		return nil, "不认识的编辑"
	}
}

func backward(after Content, op sess.Op) (known bool, value Content, err string) {
	need := func() (string, bool) {
		if after == nil {
			return "", false
		}
		return *after, true
	}
	switch op.Kind {
	case "add":
		return true, nil, ""
	case "delete":
		return true, &op.Text, ""
	case "write":
		return false, nil, ""
	case "replace":
		cur, ok := need()
		if !ok {
			return false, nil, "倒放时文件已经不存在"
		}
		next, ok := unreplace(cur, op.Old, op.New, op.All)
		if !ok {
			return false, nil, "倒放时 new_string 在正文里找不到"
		}
		return true, &next, ""
	case "multi":
		cur, ok := need()
		if !ok {
			return false, nil, "倒放时文件已经不存在"
		}
		for i := len(op.List) - 1; i >= 0; i-- {
			next, ok := unreplace(cur, op.List[i][0], op.List[i][1], op.List[i][2] == "1")
			if !ok {
				return false, nil, "倒放 MultiEdit 时 new_string 找不到"
			}
			cur = next
		}
		return true, &cur, ""
	case "update":
		cur, ok := need()
		if !ok {
			return false, nil, "倒放时文件已经不存在"
		}
		next, ok := applyDiff(cur, op.Diff, true)
		if !ok {
			return false, nil, "倒放补丁时，加上的行在当前正文里找不到"
		}
		return true, &next, ""
	default:
		return false, nil, "不认识的编辑"
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [8]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

type step struct {
	turn int
	op   sess.Op
	base *Content
}

type cell struct {
	known bool
	value Content
}

// Replay 叠出一个会话每个回合的正文。
func Replay(repo string, session sess.RawSession) []TurnFiles {
	out := make([]TurnFiles, len(session.Turns))
	for i := range out {
		out[i].Files = map[string][2]Content{}
	}
	per := map[string][]step{}
	for ti, turn := range session.Turns {
		for _, e := range turn.Edits {
			if !e.Ok {
				continue
			}
			rel, ok := Rel(repo, e.Path)
			if !ok {
				continue
			}
			per[rel] = append(per[rel], step{turn: ti, op: e.Op, base: e.Base})
		}
	}
	paths := make([]string, 0, len(per))
	for path := range per {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		steps := per[path]
		n := len(steps)
		before := make([]cell, n)
		after := make([]cell, n)
		var brokenFrom, brokenUpto *int
		var why string
		cur := cell{}
		stop := false
		for i, s := range steps {
			if s.base != nil {
				cur = cell{known: true, value: *s.base}
			}
			before[i] = cur
			if s.op.Kind == "write" || s.op.Kind == "add" || s.op.Kind == "delete" {
				v, _ := forward(nil, s.op)
				cur = cell{known: true, value: v}
			} else if cur.known {
				v, err := forward(cur.value, s.op)
				if err != "" {
					brokenFrom = &i
					why = err
					stop = true
					break
				}
				cur = cell{known: true, value: v}
			}
			if stop {
				break
			}
			after[i] = cur
		}
		unresolved := false
		for i := 0; i < n; i++ {
			if !before[i].known || !after[i].known {
				unresolved = true
			}
		}
		if brokenFrom == nil && unresolved {
			disk, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(path)))
			var cur Content
			if err == nil {
				s := string(disk)
				cur = &s
			}
			for i := n - 1; i >= 0; i-- {
				if !after[i].known {
					after[i] = cell{known: true, value: cur}
				}
				if !before[i].known {
					known, v, err := backward(after[i].value, steps[i].op)
					if err != "" {
						brokenUpto = &i
						why = err
						break
					}
					if !known {
						before[i] = cell{known: true, value: nil}
					} else {
						before[i] = cell{known: true, value: v}
					}
				}
				cur = before[i].value
			}
		}
		bad := func(j int) bool {
			if brokenFrom != nil {
				return j >= *brokenFrom
			}
			if brokenUpto != nil {
				return j <= *brokenUpto
			}
			return false
		}
		for i := 0; i < n; i++ {
			t := steps[i].turn
			badTurn := false
			for j, s := range steps {
				if s.turn == t && bad(j) {
					badTurn = true
				}
			}
			if badTurn {
				seen := false
				for _, e := range out[t].Errors {
					if e[0] == path {
						seen = true
					}
				}
				if !seen {
					out[t].Errors = append(out[t].Errors, [2]string{path, why + "。这个文件停在这里，不往下编链。"})
				}
				delete(out[t].Files, path)
				continue
			}
			if !before[i].known || !after[i].known {
				continue
			}
			entry := out[t].Files[path]
			if _, ok := out[t].Files[path]; !ok {
				entry = [2]Content{before[i].value, after[i].value}
			}
			entry[1] = after[i].value
			out[t].Files[path] = entry
		}
	}
	for ti, turn := range session.Turns {
		if turn.SnapBefore == nil || turn.SnapAfter == nil {
			continue
		}
		paths := map[string]struct{}{}
		for p := range out[ti].Files {
			paths[p] = struct{}{}
		}
		for _, e := range out[ti].Errors {
			paths[e[0]] = struct{}{}
		}
		for p := range paths {
			b, bok := turn.SnapBefore[p]
			a, aok := turn.SnapAfter[p]
			if bok && aok {
				out[ti].Files[p] = [2]Content{b, a}
				var keep [][2]string
				for _, e := range out[ti].Errors {
					if e[0] != p {
						keep = append(keep, e)
					}
				}
				out[ti].Errors = keep
			}
		}
	}
	return out
}

// FoldSession 取每个文件第一次改之前、最后一次改成功之后。
func FoldSession(turns []TurnFiles) TurnFiles {
	first := map[string]Content{}
	last := map[string]Content{}
	seenFirst := map[string]bool{}
	errors := map[string]string{}
	for _, turn := range turns {
		for path, pair := range turn.Files {
			if !seenFirst[path] {
				first[path] = pair[0]
				seenFirst[path] = true
			}
			last[path] = pair[1]
			delete(errors, path)
		}
		for _, e := range turn.Errors {
			errors[e[0]] = e[1]
		}
	}
	files := map[string][2]Content{}
	for path, after := range last {
		files[path] = [2]Content{first[path], after}
	}
	errs := make([][2]string, 0, len(errors))
	for path, why := range errors {
		errs = append(errs, [2]string{path, why})
	}
	sort.Slice(errs, func(i, j int) bool { return errs[i][0] < errs[j][0] })
	return TurnFiles{Files: files, Errors: errs}
}

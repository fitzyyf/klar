package sess

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"log"
	"os"
	"strings"
	"unicode"
)

func parseJSONL(path string) []any {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	r := bufio.NewReaderSize(f, 64*1024)
	var out []any
	for {
		line, err := r.ReadBytes('\n')
		if len(line) > 0 {
			line = bytes.TrimRight(line, "\r\n")
			if len(line) > 0 {
				var v any
				if json.Unmarshal(line, &v) == nil {
					out = append(out, v)
				}
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("会话记录没读完 %s：%v", path, err)
			break
		}
	}
	return out
}

func obj(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func str(v any) string {
	s, _ := v.(string)
	return s
}

func boolean(v any) bool {
	b, _ := v.(bool)
	return b
}

func arr(v any) []any {
	a, _ := v.([]any)
	return a
}

func asInt(v any) (int64, bool) {
	f, ok := v.(float64)
	if !ok {
		return 0, false
	}
	return int64(f), true
}

func textOf(content any) string {
	if s, ok := content.(string); ok {
		return s
	}
	var parts []string
	for _, item := range arr(content) {
		m := obj(item)
		switch str(m["type"]) {
		case "text", "input_text", "output_text", "Text":
			parts = append(parts, str(m["text"]))
		}
	}
	return strings.Join(parts, "\n")
}

func clip(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

func firstLine(text string) string {
	body := text
	a := strings.LastIndex(text, "<user_query>")
	b := strings.LastIndex(text, "</user_query>")
	if a >= 0 && b > a {
		body = text[a+len("<user_query>") : b]
	}
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return clip(line, 200)
		}
	}
	return ""
}

func shortReply(text string) *string {
	t := clip(strings.TrimSpace(text), 280)
	if t == "" {
		return nil
	}
	return &t
}

func trimStart(s string) string {
	return strings.TrimLeftFunc(s, unicode.IsSpace)
}

func presentBase(s string) *Content {
	p := new(string)
	*p = s
	c := Content(p)
	return &c
}

func missingBase() *Content {
	var c Content
	return &c
}

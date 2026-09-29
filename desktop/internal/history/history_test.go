package history

import (
	"testing"
)

func TestDirectoryNamesMatchTheAgents(t *testing.T) {
	// 测的是纯字符串编码，路径用正斜杠写死，别掺平台分隔符（Windows 会给反斜杠）。
	path := "/Users/fitz/f-project/shop"
	if ClaudeKey(path) != "-Users-fitz-f-project-shop" {
		t.Fatal(ClaudeKey(path))
	}
	if GrokKey(path) != "%2FUsers%2Ffitz%2Ff-project%2Fshop" {
		t.Fatal(GrokKey(path))
	}
	if PercentDecode(GrokKey(path)) != path {
		t.Fatal(PercentDecode(GrokKey(path)))
	}
}

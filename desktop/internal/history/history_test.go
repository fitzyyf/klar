package history

import (
	"path/filepath"
	"testing"
)

func TestDirectoryNamesMatchTheAgents(t *testing.T) {
	path := filepath.Join("/Users/fitz/f-project/shop")
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

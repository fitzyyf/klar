package store

import (
	"path/filepath"
	"testing"
)

func TestLatestReviewWins(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reviews.sqlite")
	if err := Insert(path, "/repo", "s1", "pass", "第一次", "t1"); err != nil {
		t.Fatal(err)
	}
	if err := Insert(path, "/repo", "s1", "back", "第二次", "t2"); err != nil {
		t.Fatal(err)
	}
	got, err := Latest(path, "/repo")
	if err != nil {
		t.Fatal(err)
	}
	if got["s1"].Verdict != "back" || got["s1"].TurnID != "t2" || got["s1"].Text != "第二次" {
		t.Fatalf("%+v", got["s1"])
	}
	other, err := Latest(path, "/else")
	if err != nil || len(other) != 0 {
		t.Fatalf("%v %v", other, err)
	}
}

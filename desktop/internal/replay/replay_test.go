package replay

import (
	"testing"

	"klar.dev/desktop/internal/sess"
)

func strp(s string) *string { return &s }

func TestFoldUsesFirstBeforeAndLastAfter(t *testing.T) {
	early := TurnFiles{Files: map[string][2]Content{"a.js": {strp("v1"), strp("v2")}}}
	broken := TurnFiles{Errors: [][2]string{{"a.js", "中间对不上"}}}
	late := TurnFiles{Files: map[string][2]Content{"a.js": {strp("v2"), strp("v4")}}}
	net := FoldSession([]TurnFiles{early, broken, late})
	if net.Files["a.js"][0] == nil || *net.Files["a.js"][0] != "v1" {
		t.Fatalf("before %v", net.Files["a.js"][0])
	}
	if net.Files["a.js"][1] == nil || *net.Files["a.js"][1] != "v4" {
		t.Fatalf("after %v", net.Files["a.js"][1])
	}
	if len(net.Errors) != 0 {
		t.Fatalf("errors %#v", net.Errors)
	}
}

func TestFoldKeepsErrorWhenLaterTurnDoesNotFixIt(t *testing.T) {
	early := TurnFiles{Files: map[string][2]Content{"a.js": {strp("v1"), strp("v2")}}}
	broken := TurnFiles{Errors: [][2]string{{"b.js", "对不上"}}}
	net := FoldSession([]TurnFiles{early, broken})
	if net.Files["a.js"][1] == nil || *net.Files["a.js"][1] != "v2" {
		t.Fatal(net.Files["a.js"])
	}
	if len(net.Errors) != 1 || net.Errors[0][0] != "b.js" || net.Errors[0][1] != "对不上" {
		t.Fatalf("%#v", net.Errors)
	}
}

func TestPatchDoesNotGrowATrailingBlank(t *testing.T) {
	before := "a\n\nb\n"
	diff := "@@ -1,3 +1,3 @@\n a\n \n-b\n+c\n"
	after, ok := applyDiff(before, diff, false)
	if !ok || after != "a\n\nc\n" {
		t.Fatalf("ok=%v after=%q", ok, after)
	}
}

func TestReplayUsesRecordedBase(t *testing.T) {
	base := "alpha\n"
	hold := Content(&base)
	raw := sess.RawSession{Turns: []sess.RawTurn{{
		Edits: []sess.RawEdit{{
			Path: "a.js", Ok: true, Op: sess.Replace("alpha", "beta", false), Base: &hold,
		}},
	}}}
	got := Replay("/repo", raw)
	pair := got[0].Files["a.js"]
	if pair[0] == nil || *pair[0] != "alpha\n" || pair[1] == nil || *pair[1] != "beta\n" {
		t.Fatalf("before=%v after=%v errors=%v", pair[0], pair[1], got[0].Errors)
	}
}

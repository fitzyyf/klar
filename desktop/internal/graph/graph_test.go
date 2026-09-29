package graph

import (
	"strings"
	"testing"

	"klar.dev/desktop/internal/lang"
)

func pair(before, after string) [2]*string { return [2]*string{&before, &after} }

func TestNetDiffDropsCallAddedThenRemoved(t *testing.T) {
	start := "function create() {\n  charge();\n}\nfunction charge() {\n}\n"
	end := "function create() {\n  charge();\n}\nfunction charge() {\n}\nfunction reserve() {\n}\n"
	files := map[string][2]*string{"a.js": pair(start, end)}
	g := Analyze(&Index{}, files, nil)
	var added bool
	for _, s := range g.Stars {
		if s.Label == "a.reserve" && s.Mark != nil && *s.Mark == "add" {
			added = true
		}
	}
	if !added {
		t.Fatal("reserve 应该是新方法")
	}
	for _, c := range g.Calls {
		if c.Change == "add" || c.Change == "del" {
			t.Fatalf("净改不该留下 %s", c.Change)
		}
	}
}

func TestChainPutsIfBetween(t *testing.T) {
	before := "function create(order) {\n  reserve(order);\n}\nfunction reserve() {}\n"
	after := "function create(order) {\n  if (order.price < 0) {\n    reserve(order);\n  }\n}\nfunction reserve() {}\n"
	g := Analyze(&Index{}, map[string][2]*string{"a.js": pair(before, after)}, nil)
	var decision *string
	for _, s := range g.Stars {
		if s.Kind == "if" {
			if s.Label != "if" {
				t.Fatalf("判断标签 %s", s.Label)
			}
			decision = &s.ID
		}
		if s.Body != nil && s.Body.Before != s.Body.After && !strings.Contains(s.Body.After, "if") {
			t.Fatal("改过的函数体没有 if")
		}
	}
	if decision == nil {
		t.Fatal("判断没有进图")
	}
	guarded := false
	for _, c := range g.Calls {
		if c.Guard != nil && strings.Contains(*c.Guard, "price") {
			guarded = true
		}
	}
	if !guarded {
		t.Fatal("边上没有判断条件")
	}
	onChain := false
	for _, ch := range g.Chains {
		for i := 0; i+1 < len(ch.Stars); i++ {
			if ch.Stars[i] == *decision || ch.Stars[i+1] == *decision {
				onChain = true
			}
		}
	}
	if !onChain {
		t.Fatal("判断不在链上")
	}
}

func TestChainStopsAtFailedFile(t *testing.T) {
	before := "function create() {\n}\n"
	after := "function create() {\n  missing();\n}\n"
	idx := &Index{Files: map[string][]lang.Func{"b.js": lang.Parse("b.js", "function missing() {\n}\n")}}
	g := Analyze(idx, map[string][2]*string{"a.js": pair(before, after)}, []string{"b.js"})
	hit := false
	for _, c := range g.Chains {
		if c.Broken != nil && *c.Broken == "b.js" {
			hit = true
		}
	}
	if !hit {
		t.Fatal("链没有停在对不上的文件")
	}
}

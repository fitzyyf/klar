package graph

import (
	"log"
	"sort"
	"strings"
	"time"

	"klar.dev/desktop/internal/lang"
	"klar.dev/desktop/internal/model"
)

const (
	perEntry = 20
	perTurn  = 24
)

type TurnGraph struct {
	Stars   []model.StarOut
	Calls   []model.CallOut
	Chains  []model.ChainOut
	Omitted int
}

type found struct {
	path   []string
	dashed bool
	hole   *string
}

func ptr(s string) *string { return &s }

func bodies(mark *string, previous *string, after string) model.BodyOut {
	before := after
	if mark != nil && *mark == "add" {
		before = ""
	} else if mark != nil && *mark == "edit" {
		if previous != nil {
			before = *previous
		} else {
			before = ""
		}
	}
	return model.BodyOut{Before: before, After: after}
}

func label(f *lang.Func) string { return f.Container + "." + f.Name }

// Analyze 从前后两份正文编出全链。
func Analyze(base *Index, files map[string][2]*string, broken []string) TurnGraph {
	if base == nil {
		base = &Index{}
		base.ensure()
	}
	parseSide := func(side int) map[string][]lang.Func {
		out := map[string][]lang.Func{}
		for path, pair := range files {
			if _, ok := lang.LangOf(path); !ok {
				continue
			}
			src := pair[side]
			if src == nil {
				out[path] = nil
				continue
			}
			out[path] = lang.Parse(path, *src)
		}
		return out
	}
	beforeFiles := parseSide(0)
	afterFiles := parseSide(1)
	skip := map[string]struct{}{}
	for _, p := range broken {
		skip[p] = struct{}{}
	}
	walls := map[string]string{}
	for _, path := range broken {
		for _, fn := range base.Files[path] {
			if _, ok := walls[fn.Name]; !ok {
				walls[fn.Name] = path
			}
		}
	}
	vb := newView(base, beforeFiles, skip)
	va := newView(base, afterFiles, skip)
	old := map[string]*lang.Func{}
	for _, funcs := range beforeFiles {
		for i := range funcs {
			old[funcs[i].ID] = &funcs[i]
		}
	}
	marks := map[string]string{}
	for _, funcs := range afterFiles {
		for i := range funcs {
			f := &funcs[i]
			prev, ok := old[f.ID]
			if !ok {
				marks[f.ID] = "add"
			} else if prev.Hash != f.Hash {
				marks[f.ID] = "edit"
			}
		}
	}
	edges := map[string]Hop{}
	for _, funcs := range beforeFiles {
		for i := range funcs {
			for _, e := range vb.outgoing(&funcs[i]) {
				text := e.text
				edges[e.key] = Hop{From: funcs[i].ID, To: e.to.ID, Before: &text, Uncertain: e.uncertain, GuardBefore: e.guard}
			}
		}
	}
	for _, funcs := range afterFiles {
		for i := range funcs {
			for _, e := range va.outgoing(&funcs[i]) {
				text := e.text
				slot, ok := edges[e.key]
				if !ok {
					slot = Hop{From: funcs[i].ID, To: e.to.ID}
				}
				slot.After = &text
				slot.Uncertain = e.uncertain
				slot.GuardAfter = e.guard
				edges[e.key] = slot
			}
		}
	}
	changed := map[[2]string]struct{}{}
	for _, hop := range edges {
		if hopChange(hop) != "none" {
			changed[[2]string{hop.From, hop.To}] = struct{}{}
		}
	}
	deadline := time.Now().Add(time.Second)
	sa := &search{view: va, deadline: deadline, changed: changed, walls: walls}
	sb := &search{view: vb, deadline: deadline, changed: changed, walls: walls}
	var got []found
	for id := range marks {
		if _, touched := func() (struct{}, bool) {
			for pair := range changed {
				if pair[0] == id {
					return struct{}{}, true
				}
			}
			return struct{}{}, false
		}(); touched {
			continue
		}
		f := va.get(id)
		if f == nil {
			continue
		}
		for _, u := range sa.up(f) {
			for _, d := range sa.down(f) {
				overlap := false
				for _, x := range d.path[1:] {
					if contains(u.path, x) {
						overlap = true
						break
					}
				}
				if overlap {
					continue
				}
				path := append(append([]string(nil), u.path...), d.path[1:]...)
				hole := u.hole
				if hole == nil {
					hole = d.hole
				}
				got = append(got, found{path, false, hole})
			}
		}
	}
	for _, hop := range edges {
		switch hopChange(hop) {
		case "add", "edit":
			if x, y := va.get(hop.From), va.get(hop.To); x != nil && y != nil {
				for _, t := range sa.through(x, y) {
					got = append(got, found{t.path, false, t.hole})
				}
			}
		case "del":
			if x, y := vb.get(hop.From), vb.get(hop.To); x != nil && y != nil {
				for _, t := range sb.through(x, y) {
					got = append(got, found{t.path, true, t.hole})
				}
			}
		}
	}
	score := func(c found) int {
		marked := 0
		for _, id := range c.path {
			if !c.dashed {
				if _, ok := marks[id]; ok {
					marked++
				}
			}
		}
		hops := 0
		for i := 0; i+1 < len(c.path); i++ {
			if _, ok := changed[[2]string{c.path[i], c.path[i+1]}]; ok {
				hops++
			}
		}
		return marked + hops*2
	}
	seen := map[string]struct{}{}
	var uniq []found
	for _, c := range got {
		key := stringsKey(c)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		uniq = append(uniq, c)
	}
	sort.SliceStable(uniq, func(i, j int) bool {
		si, sj := score(uniq[i]), score(uniq[j])
		if si != sj {
			return si > sj
		}
		return len(uniq[i].path) < len(uniq[j].path)
	})
	per := map[string]int{}
	var kept []found
	omitted := 0
	for _, c := range uniq {
		if len(c.path) == 0 {
			continue
		}
		if per[c.path[0]] >= perEntry || len(kept) >= perTurn {
			omitted++
			continue
		}
		per[c.path[0]]++
		kept = append(kept, c)
	}
	decisions := map[string]string{}
	branches := map[[2]string]string{}
	for i := range kept {
		kept[i].path = insertDecisions(kept[i].path, edges, kept[i].dashed, decisions, branches)
	}
	if len(decisions) > 0 {
		log.Printf("链上有 %d 个判断", len(decisions))
	}
	return emit(kept, decisions, branches, edges, marks, old, va, vb, omitted)
}

func stringsKey(c found) string {
	s := strings.Join(c.path, "\x00")
	if c.dashed {
		s += "#d"
	}
	if c.hole != nil {
		s += "#" + *c.hole
	}
	return s
}

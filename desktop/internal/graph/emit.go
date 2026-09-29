package graph

import (
	"log"

	"klar.dev/desktop/internal/lang"
	"klar.dev/desktop/internal/model"
)

func emit(kept []found, decisions map[string]string, branches map[[2]string]string, edges map[string]Hop, marks map[string]string, old map[string]*lang.Func, va, vb *view, omitted int) TurnGraph {
	var stars []model.StarOut
	starSeen := map[string]struct{}{}
	var calls []model.CallOut
	callSeen := map[[2]string]struct{}{}
	for _, item := range kept {
		view := va
		if item.dashed {
			view = vb
		}
		for i, id := range item.path {
			if _, ok := starSeen[id]; ok {
				continue
			}
			starSeen[id] = struct{}{}
			if label, ok := decisions[id]; ok {
				stars = append(stars, model.StarOut{ID: id, Label: label, Kind: "if", Mark: decisionMark(id, edges)})
				continue
			}
			f := view.get(id)
			if f == nil {
				f = va.get(id)
			}
			if f == nil {
				f = vb.get(id)
			}
			if f == nil {
				continue
			}
			var mark *string
			if !item.dashed {
				if m, ok := marks[id]; ok {
					mark = &m
				}
			}
			var prev *string
			if o := old[id]; o != nil {
				prev = &o.Body
			}
			raw := bodies(mark, prev, f.Body)
			var body *model.BodyOut
			if raw.Before != "" || raw.After != "" {
				body = &raw
			}
			kind := "fn"
			if leafLike(f) || (i+1 == len(item.path) && f.Abstract) {
				kind = "leaf"
			} else if i == 0 {
				kind = "entry"
			}
			var unknown *bool
			if f.UnknownCalls {
				t := true
				unknown = &t
			}
			stars = append(stars, model.StarOut{ID: id, Label: labelOf(f), Kind: kind, Mark: mark, UnknownCalls: unknown, Body: body})
		}
		for i := 0; i+1 < len(item.path); i++ {
			pair := [2]string{item.path[i], item.path[i+1]}
			if _, ok := callSeen[pair]; ok {
				continue
			}
			callSeen[pair] = struct{}{}
			bracket := branches[pair]
			var guard *string
			if bracket != "" {
				guard = &bracket
			}
			_, toIsDec := decisions[pair[1]]
			_, fromIsDec := decisions[pair[0]]
			if toIsDec && !fromIsDec {
				change := "none"
				if m := decisionMark(pair[1], edges); m != nil {
					change = *m
				}
				calls = append(calls, model.CallOut{Key: pair[0] + ">" + pair[1], From: pair[0], To: pair[1], Change: change, Guard: guard})
				continue
			}
			if fromIsDec {
				var hitKey string
				var hit *Hop
				for key, hop := range edges {
					if hop.To != pair[1] {
						continue
					}
					raw := hopGuard(hop, item.dashed)
					if raw == nil {
						continue
					}
					for _, step := range lang.Decode(*raw) {
						if decisionID(hop.From, step.Decision) == pair[0] {
							h := hop
							hit = &h
							hitKey = key
						}
					}
				}
				if hit != nil {
					change := hopChange(*hit)
					calls = append(calls, callOut(hitKey, pair[0], pair[1], change, hit, guard))
				} else {
					change := "none"
					if m := decisionMark(pair[0], edges); m != nil {
						change = *m
					}
					calls = append(calls, model.CallOut{Key: pair[0] + ">" + pair[1], From: pair[0], To: pair[1], Change: change, Guard: guard})
				}
				continue
			}
			var bestKey string
			var best *Hop
			for key, hop := range edges {
				if hop.From != pair[0] || hop.To != pair[1] {
					continue
				}
				h := hop
				if best == nil || (hopChange(*best) == "none" && hopChange(h) != "none") {
					best = &h
					bestKey = key
				}
			}
			if best != nil {
				calls = append(calls, callOut(bestKey, best.From, best.To, hopChange(*best), best, hopGuard(*best, item.dashed)))
				continue
			}
			eView := va
			if item.dashed {
				eView = vb
			}
			var picked *edge
			if f := eView.get(pair[0]); f != nil {
				for _, e := range eView.outgoing(f) {
					if e.to.ID == pair[1] {
						ee := e
						picked = &ee
						break
					}
				}
			}
			key := pair[0] + ">" + pair[1] + "#0"
			var text *string
			var uncertain *bool
			var g *string
			if picked != nil {
				key = picked.key
				text = &picked.text
				if picked.uncertain {
					t := true
					uncertain = &t
				}
				g = picked.guard
			}
			calls = append(calls, model.CallOut{Key: key, From: pair[0], To: pair[1], Change: "none", Uncertain: uncertain, Text: text, Guard: g})
		}
	}
	var chains []model.ChainOut
	holes := 0
	for _, item := range kept {
		ch := model.ChainOut{Stars: item.path, Broken: item.hole}
		if item.dashed {
			t := true
			ch.Dashed = &t
		}
		if item.hole != nil {
			holes++
		}
		chains = append(chains, ch)
	}
	if holes > 0 {
		log.Printf("%d 条链断在对不上的文件", holes)
	}
	diffs := 0
	for _, s := range stars {
		if s.Body != nil && s.Body.Before != s.Body.After {
			diffs++
		}
	}
	if diffs > 0 {
		log.Printf("编辑器里有 %d 处函数体对照", diffs)
	}
	if stars == nil {
		stars = []model.StarOut{}
	}
	if calls == nil {
		calls = []model.CallOut{}
	}
	if chains == nil {
		chains = []model.ChainOut{}
	}
	return TurnGraph{Stars: stars, Calls: calls, Chains: chains, Omitted: omitted}
}

func labelOf(f *lang.Func) string { return f.Container + "." + f.Name }

func callOut(key, from, to, change string, hop *Hop, guard *string) model.CallOut {
	out := model.CallOut{Key: key, From: from, To: to, Change: change, Guard: guard}
	if hop.Uncertain {
		t := true
		out.Uncertain = &t
	}
	if change == "del" || change == "edit" {
		out.Before = hop.Before
	}
	if change == "add" || change == "edit" {
		out.After = hop.After
	}
	if change == "none" {
		if hop.After != nil {
			out.Text = hop.After
		} else {
			out.Text = hop.Before
		}
	}
	return out
}

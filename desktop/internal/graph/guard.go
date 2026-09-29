// 把 if、else、switch、循环插进调用链。菱形不是方法，是包着下一跳的判断。
package graph

import (
	"hash/fnv"
	"strconv"

	"klar.dev/desktop/internal/lang"
)

// Hop：调用方、被调方、改前文本、改后文本、不确定、改前判断、改后判断。
type Hop struct {
	From, To       string
	Before, After  *string
	Uncertain      bool
	GuardBefore    *string
	GuardAfter     *string
}

func hopChange(h Hop) string {
	switch {
	case h.Before == nil && h.After != nil:
		return "add"
	case h.Before != nil && h.After == nil:
		return "del"
	case h.Before != nil && h.After != nil && (*h.Before != *h.After || !same(h.GuardBefore, h.GuardAfter)):
		return "edit"
	default:
		return "none"
	}
}

func same(a, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func hopGuard(h Hop, dashed bool) *string {
	if dashed {
		return h.GuardBefore
	}
	if h.GuardAfter != nil {
		return h.GuardAfter
	}
	return h.GuardBefore
}

func decisionID(from, decision string) string {
	h := fnv.New64a()
	_, _ = h.Write([]byte(decision))
	return "if#" + from + "#" + strconv.FormatUint(h.Sum64(), 10)
}

func idsOf(from string, encoded *string) []string {
	if encoded == nil {
		return nil
	}
	var out []string
	for _, step := range lang.Decode(*encoded) {
		out = append(out, decisionID(from, step.Decision))
	}
	return out
}

func insertDecisions(chain []string, edges map[string]Hop, dashed bool, decisions map[string]string, branches map[[2]string]string) []string {
	if len(chain) == 0 {
		return nil
	}
	out := []string{chain[0]}
	for i := 0; i < len(chain)-1; i++ {
		var steps []lang.Step
		for _, hop := range edges {
			if hop.From == chain[i] && hop.To == chain[i+1] {
				if raw := hopGuard(hop, dashed); raw != nil {
					steps = lang.Decode(*raw)
				}
				break
			}
		}
		segment := []string{chain[i]}
		for _, step := range steps {
			id := decisionID(chain[i], step.Decision)
			if _, ok := decisions[id]; !ok {
				decisions[id] = step.Kind
			}
			segment = append(segment, id)
		}
		segment = append(segment, chain[i+1])
		for index := 1; index < len(segment)-1; index++ {
			branches[[2]string{segment[index], segment[index+1]}] = "[" + steps[index-1].Branch + "]"
		}
		for _, id := range segment[1:] {
			if out[len(out)-1] != id {
				out = append(out, id)
			}
		}
	}
	return out
}

func decisionMark(id string, edges map[string]Hop) *string {
	added, edited, existed := false, false, false
	for _, hop := range edges {
		for _, got := range idsOf(hop.From, hop.GuardAfter) {
			if got != id {
				continue
			}
			if hop.Before == nil {
				added = true
			} else if !same(hop.GuardBefore, hop.GuardAfter) {
				edited = true
			}
		}
		for _, got := range idsOf(hop.From, hop.GuardBefore) {
			if got == id {
				existed = true
			}
		}
	}
	var mark string
	switch {
	case added && !existed:
		mark = "add"
	case edited:
		mark = "edit"
	default:
		return nil
	}
	return &mark
}

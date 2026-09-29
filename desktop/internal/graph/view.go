package graph

import (
	"strings"

	"klar.dev/desktop/internal/lang"
)

type edge struct {
	to        *lang.Func
	key       string
	text      string
	guard     *string
	uncertain bool
}

type view struct {
	byID      map[string]*lang.Func
	byName    map[string][]*lang.Func
	callersBy map[string][]*lang.Func
}

func newView(base *Index, overlay map[string][]lang.Func, skip map[string]struct{}) *view {
	v := &view{byID: map[string]*lang.Func{}, byName: map[string][]*lang.Func{}, callersBy: map[string][]*lang.Func{}}
	add := func(f *lang.Func) {
		v.byID[f.ID] = f
		v.byName[f.Name] = append(v.byName[f.Name], f)
		seen := map[string]struct{}{}
		for i := range f.Calls {
			if _, ok := seen[f.Calls[i].Name]; ok {
				continue
			}
			seen[f.Calls[i].Name] = struct{}{}
			v.callersBy[f.Calls[i].Name] = append(v.callersBy[f.Calls[i].Name], f)
		}
	}
	for path, funcs := range base.Files {
		if _, covered := overlay[path]; covered {
			continue
		}
		if _, bad := skip[path]; bad {
			continue
		}
		for i := range funcs {
			add(&funcs[i])
		}
	}
	for _, funcs := range overlay {
		for i := range funcs {
			add(&funcs[i])
		}
	}
	return v
}

func (v *view) get(id string) *lang.Func { return v.byID[id] }

func stem(path string) string { return strings.ToLower(lang.ModuleName(path)) }

func leafLike(f *lang.Func) bool {
	if !f.Abstract {
		return false
	}
	for _, s := range []string{"Mapper", "Repository", "Dao", "Repo"} {
		if strings.HasSuffix(f.Container, s) {
			return true
		}
	}
	return false
}

func (v *view) resolve(caller *lang.Func, call *lang.Call) (*lang.Func, bool, bool) {
	var cands []*lang.Func
	for _, f := range v.byName[call.Name] {
		if f.ID != caller.ID {
			cands = append(cands, f)
		}
	}
	if len(cands) == 0 {
		return nil, false, false
	}
	var recv *string
	if call.Receiver != nil {
		r := strings.TrimPrefix(*call.Receiver, "this.")
		r = strings.TrimPrefix(r, "self.")
		if i := strings.LastIndexAny(r, ".:"); i >= 0 {
			r = r[i+1:]
		}
		r = strings.Trim(r, "_")
		r = strings.TrimPrefix(r, "$")
		r = strings.ToLower(r)
		recv = &r
	}
	var picks []*lang.Func
	fuzzy := false
	if recv == nil || *recv == "this" || *recv == "self" || *recv == "super" || *recv == "cls" || *recv == "" {
		var same, file, module []*lang.Func
		for _, f := range cands {
			if f.Path == caller.Path && f.Container == caller.Container {
				same = append(same, f)
			}
			if f.Path == caller.Path {
				file = append(file, f)
			}
			if strings.ToLower(f.Container) == stem(f.Path) {
				module = append(module, f)
			}
		}
		switch {
		case len(same) > 0:
			picks = same
		case len(file) > 0:
			picks = file
		case recv == nil && len(module) > 0:
			picks = module
			fuzzy = len(module) > 1
		default:
			return nil, false, false
		}
	} else {
		r := *recv
		var exact []*lang.Func
		for _, f := range cands {
			if strings.ToLower(f.Container) == r || stem(f.Path) == r {
				exact = append(exact, f)
			}
		}
		if len(exact) > 0 {
			picks = exact
		} else if len(r) >= 3 {
			var loose []*lang.Func
			for _, f := range cands {
				c := strings.ToLower(f.Container)
				if len(c) >= 3 && (strings.HasPrefix(c, r) || strings.HasPrefix(r, c) || strings.HasSuffix(r, c)) {
					loose = append(loose, f)
				}
			}
			if len(loose) == 0 {
				return nil, false, false
			}
			picks = loose
			fuzzy = true
		} else {
			return nil, false, false
		}
	}
	var concrete []*lang.Func
	for _, f := range picks {
		if !f.Abstract {
			concrete = append(concrete, f)
		}
	}
	first := picks[0]
	if len(concrete) > 0 {
		first = concrete[0]
	}
	if first.Abstract && !leafLike(first) {
		var impls []*lang.Func
		for _, f := range cands {
			if f.Abstract {
				continue
			}
			for _, s := range f.Supers {
				if s == first.Container {
					impls = append(impls, f)
				}
			}
		}
		if len(impls) == 1 {
			return impls[0], true, true
		}
		return first, true, true
	}
	many := len(picks)
	if len(concrete) > 0 {
		many = len(concrete)
	}
	return first, fuzzy || many > 1, true
}

func (v *view) links(caller *lang.Func, call *lang.Call) bool {
	_, _, ok := v.resolve(caller, call)
	return ok
}

func (v *view) outgoing(f *lang.Func) []edge {
	count := map[string]int{}
	var out []edge
	for i := range f.Calls {
		to, uncertain, ok := v.resolve(f, &f.Calls[i])
		if !ok {
			continue
		}
		n := count[to.ID]
		out = append(out, edge{to: to, key: f.ID + ">" + to.ID + "#" + itoa(n), text: f.Calls[i].Text, guard: f.Calls[i].Guard, uncertain: uncertain})
		count[to.ID] = n + 1
	}
	return out
}

func (v *view) callers(target *lang.Func) []*lang.Func {
	var out []*lang.Func
	for _, c := range v.callersBy[target.Name] {
		for i := range c.Calls {
			if c.Calls[i].Name != target.Name {
				continue
			}
			to, _, ok := v.resolve(c, &c.Calls[i])
			if ok && to.ID == target.ID {
				out = append(out, c)
				break
			}
		}
	}
	return out
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [16]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

package graph

import (
	"time"

	"klar.dev/desktop/internal/lang"
)

const (
	maxDepth = 12
	branchN  = 6
)

type trail struct {
	path []string
	hole *string
}

type search struct {
	view     *view
	deadline time.Time
	changed  map[[2]string]struct{}
	walls    map[string]string
}

func (s *search) late() bool { return time.Now().After(s.deadline) }

func (s *search) downWall(f *lang.Func) *string {
	for i := range f.Calls {
		if s.view.links(f, &f.Calls[i]) {
			continue
		}
		if path, ok := s.walls[f.Calls[i].Name]; ok {
			return &path
		}
	}
	return nil
}

func (s *search) up(f *lang.Func) []trail {
	var out []trail
	path := []string{f.ID}
	s.upFrom(f, &path, &out)
	for i := range out {
		reverse(out[i].path)
	}
	return out
}

func (s *search) upFrom(f *lang.Func, path *[]string, out *[]trail) {
	if len(*out) >= branchN {
		return
	}
	var callers []*lang.Func
	if !f.Entry && len(*path) < maxDepth && !s.late() {
		for _, c := range s.view.callers(f) {
			if contains(*path, c.ID) {
				continue
			}
			callers = append(callers, c)
			if len(callers) >= branchN {
				break
			}
		}
	}
	if len(callers) == 0 {
		*out = append(*out, trail{path: append([]string(nil), *path...)})
		return
	}
	for _, c := range callers {
		*path = append(*path, c.ID)
		s.upFrom(c, path, out)
		*path = (*path)[:len(*path)-1]
	}
}

func (s *search) down(f *lang.Func) []trail {
	var out []trail
	path := []string{f.ID}
	s.downFrom(f, &path, &out)
	return out
}

func (s *search) downFrom(f *lang.Func, path *[]string, out *[]trail) {
	if len(*out) >= branchN {
		return
	}
	var next []edge
	if !f.Abstract && len(*path) < maxDepth && !s.late() {
		seen := map[string]struct{}{}
		for _, e := range s.view.outgoing(f) {
			key := e.to.ID + "|" + str(e.guard)
			if contains(*path, e.to.ID) {
				continue
			}
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			next = append(next, e)
		}
	}
	// 改过的调用排前面。
	for i := 0; i < len(next); i++ {
		for j := i + 1; j < len(next); j++ {
			ai := s.changed[[2]string{f.ID, next[i].to.ID}]
			aj := s.changed[[2]string{f.ID, next[j].to.ID}]
			_, aok := s.changed[[2]string{f.ID, next[i].to.ID}]
			_, bok := s.changed[[2]string{f.ID, next[j].to.ID}]
			_ = ai
			_ = aj
			if !aok && bok {
				next[i], next[j] = next[j], next[i]
			}
		}
	}
	hole := s.downWall(f)
	if len(next) == 0 {
		*out = append(*out, trail{path: append([]string(nil), *path...), hole: hole})
		return
	}
	if hole != nil {
		cp := *hole
		*out = append(*out, trail{path: append([]string(nil), *path...), hole: &cp})
	}
	if len(next) > branchN {
		next = next[:branchN]
	}
	for _, e := range next {
		*path = append(*path, e.to.ID)
		s.downFrom(e.to, path, out)
		*path = (*path)[:len(*path)-1]
	}
}

func (s *search) through(from, to *lang.Func) []trail {
	var out []trail
	for _, u := range s.up(from) {
		for _, d := range s.down(to) {
			overlap := false
			for _, x := range d.path {
				if contains(u.path, x) {
					overlap = true
					break
				}
			}
			if overlap {
				continue
			}
			path := append(append([]string(nil), u.path...), d.path...)
			hole := u.hole
			if hole == nil {
				hole = d.hole
			}
			out = append(out, trail{path: path, hole: hole})
		}
	}
	return out
}

func contains(path []string, id string) bool {
	for _, x := range path {
		if x == id {
			return true
		}
	}
	return false
}

func reverse(s []string) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}

func str(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

package graph

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"klar.dev/desktop/internal/head"
	"klar.dev/desktop/internal/lang"
)

const skipDirs = "\x00node_modules\x00dist\x00build\x00target\x00vendor\x00__pycache__\x00.venv\x00out\x00coverage\x00"

type Index struct {
	Files  map[string][]lang.Func
	head   string
	stamps map[string][2]int64
}

func (idx *Index) ensure() {
	if idx.Files == nil {
		idx.Files = map[string][]lang.Func{}
	}
	if idx.stamps == nil {
		idx.stamps = map[string][2]int64{}
	}
}

// Refresh 从参照提交读调用方。这个会话改过的文件由外面的正文盖掉。
func (idx *Index) Refresh(repo string) {
	idx.ensure()
	if sha, files, ok := head.Tracked(repo); ok {
		if idx.head == sha {
			return
		}
		shown := sha
		if len(shown) > 7 {
			shown = shown[:7]
		}
		log.Printf("参照 %s 接调用方，%d 个文件", shown, len(files))
		idx.Files = map[string][]lang.Func{}
		idx.stamps = map[string][2]int64{}
		for _, file := range files {
			idx.Files[file.Path] = lang.Parse(file.Path, file.Src)
		}
		idx.head = sha
		return
	}
	if idx.head == "" && len(idx.Files) == 0 {
		log.Printf("不是 git 仓库，调用方按磁盘上的当前源码接")
	}
	idx.head = ""
	idx.refreshWorktree(repo)
}

func (idx *Index) refreshWorktree(repo string) {
	var paths []struct {
		rel  string
		mod  int64
		size int64
	}
	_ = filepath.WalkDir(repo, func(path string, d os.DirEntry, err error) error {
		if err != nil || d == nil {
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			if strings.Contains(skipDirs, "\x00"+name+"\x00") {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(repo, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if _, ok := lang.LangOf(rel); !ok || strings.HasSuffix(rel, ".min.js") || strings.HasSuffix(rel, ".d.ts") {
			return nil
		}
		info, err := d.Info()
		if err != nil || info.Size() > 512*1024 {
			return nil
		}
		paths = append(paths, struct {
			rel  string
			mod  int64
			size int64
		}{rel, info.ModTime().UnixNano(), info.Size()})
		if len(paths) >= 40000 {
			return filepath.SkipAll
		}
		return nil
	})
	alive := map[string]struct{}{}
	for _, p := range paths {
		alive[p.rel] = struct{}{}
		stamp := [2]int64{p.mod, p.size}
		if idx.stamps[p.rel] == stamp {
			continue
		}
		src, _ := os.ReadFile(filepath.Join(repo, filepath.FromSlash(p.rel)))
		idx.stamps[p.rel] = stamp
		idx.Files[p.rel] = lang.Parse(p.rel, string(src))
	}
	for p := range idx.Files {
		if _, ok := alive[p]; !ok {
			delete(idx.Files, p)
			delete(idx.stamps, p)
		}
	}
}

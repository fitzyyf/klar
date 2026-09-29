package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"klar.dev/desktop/internal/engine"
	"klar.dev/desktop/internal/model"
	"klar.dev/desktop/internal/uiembed"
)

func uiDir() (string, error) {
	// 开发时直接用仓库里的 app/ui，改了就生效；没有磁盘页面就走编译进二进制的内嵌页（Makefile 编译前复制）。
	wd, err := os.Getwd()
	if err == nil {
		for _, rel := range []string{"app/ui", "../app/ui"} {
			dir := filepath.Join(wd, rel)
			if st, err := os.Stat(filepath.Join(dir, "index.html")); err == nil && !st.IsDir() {
				return dir, nil
			}
		}
	}
	if _, err := uiembed.UI.ReadFile("ui/index.html"); err == nil {
		return "", nil
	}
	return "", fmt.Errorf("没有页面：仓库里的 app/ui 找不到，内嵌页也没编进来（用 make build）")
}

func handler(ui string, app *engine.App) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/load", func(w http.ResponseWriter, r *http.Request) { serveLoad(w, r, app) })
	mux.HandleFunc("POST /api/session", func(w http.ResponseWriter, r *http.Request) { serveSession(w, r, app) })
	mux.HandleFunc("POST /api/verdict", func(w http.ResponseWriter, r *http.Request) { serveVerdict(w, r, app) })
	mux.HandleFunc("GET /api/watch", serveWatch)
	page := http.FileServer(http.Dir(ui))
	if ui == "" {
		page = http.FileServer(http.FS(uiembed.Root))
	}
	mux.Handle("/", page)
	return mux
}

func serveLoad(w http.ResponseWriter, r *http.Request, app *engine.App) {
	var in struct {
		Repo *string `json:"repo"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil && err != io.EOF {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	asked := ""
	if in.Repo != nil {
		asked = *in.Repo
	}
	log.Printf("读会话 repo=%q", asked)
	out, err := app.Load(r.Context(), asked)
	if err != nil {
		log.Printf("读会话失败：%v", err)
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	raw, err := json.Marshal(out)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	log.Printf("读会话完成 %d 字节", len(raw))
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Write(raw)
}

func serveSession(w http.ResponseWriter, r *http.Request, app *engine.App) {
	var in struct {
		Repo  string `json:"repo"`
		Sid   string `json:"sid"`
		Agent string `json:"agent"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil && err != io.EOF {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	log.Printf("组装请求 %s %s", in.Agent, in.Sid)
	out, err := app.Open(r.Context(), in.Repo, in.Agent, in.Sid)
	if err != nil {
		log.Printf("组装失败：%v", err)
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, out)
}

func serveVerdict(w http.ResponseWriter, r *http.Request, app *engine.App) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var in struct {
		Repo    string `json:"repo"`
		Sid     string `json:"sid"`
		Verdict string `json:"verdict"`
		Opinion string `json:"opinion"`
		Title   string `json:"title"`
		TurnID  string `json:"turnId"`
	}
	if err := json.Unmarshal(body, &in); err != nil {
		writeJSON(w, model.VerdictOut{Ok: false, Note: "评审请求读不懂：" + err.Error()})
		return
	}
	log.Printf("记下评审请求 会话 %s 结论 %s", in.Sid, in.Verdict)
	writeJSON(w, app.Deliver(in.Repo, in.Sid, in.Verdict, in.Opinion, in.Title, in.TurnID))
}

func writeJSON(w http.ResponseWriter, v any) {
	raw, err := json.Marshal(v)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Write(raw)
}

var (
	watchMu sync.Mutex
	watches = map[chan struct{}]struct{}{}
)

func serveWatch(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "不能推送", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	ch := make(chan struct{}, 1)
	watchMu.Lock()
	watches[ch] = struct{}{}
	watchMu.Unlock()
	defer func() {
		watchMu.Lock()
		delete(watches, ch)
		watchMu.Unlock()
	}()
	fmt.Fprintf(w, ": ok\n\n")
	flusher.Flush()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ch:
			fmt.Fprintf(w, "data: 1\n\n")
			flusher.Flush()
		}
	}
}

func poke() {
	watchMu.Lock()
	defer watchMu.Unlock()
	for ch := range watches {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

func watchSessions() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("盯不住会话记录：%v", err)
		return
	}
	defer watcher.Close()
	home, _ := os.UserHomeDir()
	var any bool
	for _, rel := range []string{".claude/projects", ".grok/sessions", ".codex/sessions"} {
		root := filepath.Join(home, rel)
		if st, err := os.Stat(root); err != nil || !st.IsDir() {
			continue
		}
		if err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil || d == nil || !d.IsDir() {
				return nil
			}
			if err := watcher.Add(path); err != nil {
				log.Printf("没盯上 %s：%v", path, err)
			}
			return nil
		}); err != nil {
			log.Printf("没走完 %s：%v", root, err)
			continue
		}
		log.Printf("盯着 %s", root)
		any = true
	}
	if !any {
		return
	}
	var pending *time.Timer
	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Op&fsnotify.Create != 0 {
				if st, err := os.Stat(event.Name); err == nil && st.IsDir() {
					_ = watcher.Add(event.Name)
				}
			}
			if pending != nil {
				pending.Stop()
			}
			pending = time.AfterFunc(200*time.Millisecond, poke)
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Printf("盯会话出错：%v", err)
		}
	}
}

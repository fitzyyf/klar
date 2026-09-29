package head

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestTrackedDoesNotDeadlock(t *testing.T) {
	dir := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init")
	for i := 0; i < 900; i++ {
		name := filepath.Join(dir, fmt.Sprintf("F%04d.java", i))
		body := "class A { void m() { int x = 1; int y = 2; int z = x + y; } }\n"
		if err := os.WriteFile(name, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git("add", ".")
	git("commit", "-m", "t")
	done := make(chan int, 1)
	go func() {
		_, files, ok := Tracked(dir)
		if !ok {
			done <- -1
			return
		}
		done <- len(files)
	}()
	select {
	case n := <-done:
		if n < 800 {
			t.Fatalf("读回 %d 个文件", n)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("读取参照提交堵住了")
	}
}

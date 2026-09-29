// 调用方从参照提交读，不读工作区里还没提交的改动。
package head

import (
	"bufio"
	"io"
	"log"
	"os/exec"
	"strings"
)

type File struct {
	Path string
	Src  string
}

func keep(name string) bool {
	ext := name
	if i := strings.LastIndex(name, "."); i >= 0 {
		ext = strings.ToLower(name[i+1:])
	}
	switch ext {
	case "java", "ts", "mts", "cts", "tsx", "js", "jsx", "mjs", "cjs", "vue", "py", "go":
	default:
		return false
	}
	return !strings.HasSuffix(name, ".min.js") && !strings.HasSuffix(name, ".d.ts")
}

// Tracked 读 HEAD 里能解析的源码。不是 git 仓库就返回 ok=false。
func Tracked(repo string) (string, []File, bool) {
	shaOut, err := exec.Command("git", "-C", repo, "rev-parse", "HEAD").Output()
	if err != nil {
		return "", nil, false
	}
	sha := strings.TrimSpace(string(shaOut))
	if sha == "" {
		return "", nil, false
	}
	list, err := exec.Command("git", "-C", repo, "ls-tree", "-r", "--name-only", "-z", "HEAD").Output()
	if err != nil {
		return "", nil, false
	}
	var names []string
	for _, raw := range strings.Split(string(list), "\x00") {
		if raw == "" || !keep(raw) {
			continue
		}
		names = append(names, raw)
		if len(names) >= 40000 {
			break
		}
	}
	log.Printf("读取参照提交 %s，%d 个源码文件", sha[:min(7, len(sha))], len(names))
	cmd := exec.Command("git", "-C", repo, "cat-file", "--batch")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", nil, false
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", nil, false
	}
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return "", nil, false
	}
	// 请求和正文要同时进行。先写完再读，管道一满 git 和这边会互相堵住。
	go func() {
		for _, name := range names {
			if _, err := io.WriteString(stdin, "HEAD:"+name+"\n"); err != nil {
				break
			}
		}
		_ = stdin.Close()
	}()
	rd := bufio.NewReader(stdout)
	var files []File
	for _, name := range names {
		header, err := rd.ReadString('\n')
		if err != nil {
			break
		}
		if strings.Contains(header, " missing") {
			continue
		}
		fields := strings.Fields(header)
		if len(fields) < 3 {
			continue
		}
		var size int
		for _, c := range fields[2] {
			if c < '0' || c > '9' {
				size = -1
				break
			}
			size = size*10 + int(c-'0')
		}
		if size < 0 {
			continue
		}
		buf := make([]byte, size)
		if _, err := io.ReadFull(rd, buf); err != nil {
			break
		}
		_, _ = rd.ReadByte()
		if size > 512*1024 || strings.ContainsRune(string(buf), 0) {
			continue
		}
		files = append(files, File{Path: name, Src: string(buf)})
	}
	_ = cmd.Wait()
	return sha, files, true
}

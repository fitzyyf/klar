VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS = -s -w
APP = Agent会话Review

# 页面用 go:embed 编进二进制（internal/uiembed/ui，编译前从 ../app/ui 复制）。
# tree-sitter 走 cgo，全平台都要 C 编译器：mac 用系统 clang，Windows 用 MinGW-w64。

.PHONY: build app windows winres check-windows uiembed editor test clean

build: uiembed
	go build -trimpath -ldflags="$(LDFLAGS)" -o klar .

# macOS 包：普通窗口应用（有 Dock 图标，不是菜单栏应用）。
app: build
	@rm -rf "$(APP).app"
	@mkdir -p "$(APP).app/Contents/MacOS" "$(APP).app/Contents/Resources"
	@cp klar "$(APP).app/Contents/MacOS/klar"
	@rm -rf build/darwin/AppIcon.iconset && mkdir -p build/darwin/AppIcon.iconset
	@for s in 16 32 128 256 512; do \
		sips -z $$s $$s icons/icon.png --out build/darwin/AppIcon.iconset/icon_$${s}x$${s}.png >/dev/null; \
		sips -z $$((s*2)) $$((s*2)) icons/icon.png --out build/darwin/AppIcon.iconset/icon_$${s}x$${s}@2x.png >/dev/null; \
	done
	@iconutil -c icns build/darwin/AppIcon.iconset -o "$(APP).app/Contents/Resources/AppIcon.icns"
	@rm -rf build/darwin/AppIcon.iconset
	@sed 's/@VERSION@/$(VERSION)/' build/darwin/Info.plist > "$(APP).app/Contents/Info.plist"
	@codesign --force --deep -s - "$(APP).app" 2>/dev/null || true
	@echo "  $(APP).app"

# Windows 版：WebView2 不用 cgo，但 tree-sitter 要，所以得在装了 MinGW-w64 的 Windows 上编。
# go-winres 把图标、版本、DPI 清单塞进 exe，出来是单文件。
windows: check-windows uiembed winres
	CGO_ENABLED=1 go build -trimpath -ldflags="$(LDFLAGS) -H windowsgui" -o klar.exe .
	@rm -f rsrc_windows_*.syso

# Windows 的 make 未必有 uname/grep，用系统变量判；其余平台看 uname。
ifeq ($(OS),Windows_NT)
check-windows:
	@echo Windows 环境
else
check-windows:
	@uname -s | grep -qi mingw || { echo "Windows 版要在 Windows 上编（tree-sitter 走 cgo，需要 MinGW-w64 的 gcc）。"; exit 1; }
endif

winres:
	go run github.com/tc-hib/go-winres@v0.3.3 make --in build/windows/winres.json --arch amd64 --out rsrc

uiembed:
	@rm -rf internal/uiembed/ui
	@cp -R ../app/ui internal/uiembed/ui
	@find internal/uiembed/ui -name .DS_Store -delete

test: uiembed
	go vet ./... && go test ./...

clean:
	rm -rf klar klar.exe "$(APP).app" rsrc_windows_*.syso

# 编辑器包：app/ui/editor.mjs 是源码，app/ui/vendor/editor.js 是产物。
# 改了 editor.mjs 必须重跑这个，否则页面用的还是旧包，源码改动等于没改。
# 不挂在 uiembed 后面：它要装包、要联网，普通构建不该被它拖住。
editor:
	@rm -rf build/editor
	@mkdir -p build/editor
	@cd build/editor && printf '{"name":"klar-editor","private":true,"type":"module"}\n' > package.json
	@cd build/editor && bun add @codemirror/state @codemirror/view @codemirror/language @codemirror/merge @lezer/highlight @codemirror/lang-javascript @codemirror/lang-java @codemirror/lang-python @codemirror/lang-go
	@cp ../app/ui/editor.mjs build/editor/editor.mjs
	@cd build/editor && bun build editor.mjs --minify --format=iife --outfile editor.js
	@cp build/editor/editor.js ../app/ui/vendor/editor.js
	@echo "  app/ui/vendor/editor.js"

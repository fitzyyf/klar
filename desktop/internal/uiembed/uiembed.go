// Package uiembed 对齐 magpie：页面由 Makefile 在编译前从 ../app/ui 复制进来，go:embed 进二进制。
// 单个二进制自带界面，mac 的 .app 和 Windows 的 exe 都不用再带一份页面目录。
package uiembed

import (
	"embed"
	"io/fs"
)

//go:embed all:ui
var UI embed.FS

// Root 去掉 ui/ 前缀，直接对到站点根。
var Root, _ = fs.Sub(UI, "ui")

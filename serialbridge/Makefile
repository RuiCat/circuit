# serialbridge/Makefile 是串口桥的构建入口：本机构建 / 测试 / 三平台交叉编译 / 模块化机检。
#
# 变体轴 = GOOS，轴值 = linux / darwin / windows（只声明实际存在的值）：
#   共享层：bridge.go relay.go slots.go errors.go link.go port.go（无平台依赖）
#   Unix 层：ptyUnix.go linkUnix.go portUnix.go（//go:build unix）
#   平台差异：ptyLinux.go ptyDarwin.go portWindows.go（各放该平台差异段）
# 平台实现的选择由 Go 构建约束（文件头 //go:build）完成，这里只负责把矩阵跑一遍。
SHELL := /bin/sh

.PHONY: all build test cross lint

all: build

build:
	go build ./...

test:
	go test -count=1 ./...

# cross 逐个平台编译本包与 elementio 子包（含其 element 依赖），任一个失败即整体失败。
cross:
	GOOS=linux GOARCH=amd64 go build ./...
	GOOS=darwin GOARCH=arm64 go build ./...
	GOOS=windows GOARCH=amd64 go build ./...

lint:
	bash $(HOME)/.dsh/skills/code-modularity/scripts/lintCodeModularity.sh . --naming --comments

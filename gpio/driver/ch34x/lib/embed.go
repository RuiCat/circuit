// Package lib 内嵌 CH34x 动态链接库 (.so)，按编译平台自动选择。
// 运行时优先使用环境变量 CH34X_LIB_PATH 或系统路径，
// 均不可用时自动提取内嵌库到临时目录并加载。
package lib

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// data 由各平台约束文件在 init() 中赋值。
var data []byte

// Data 返回当前编译平台对应的内嵌动态库字节数据。
// 若当前平台无内嵌库，返回 nil。
func Data() []byte {
	return data
}

// Extract 将内嵌动态库写入 destDir 目录，返回写入后的完整路径。
// 若当前平台无内嵌库则返回错误。
func Extract(destDir string) (string, error) {
	if len(data) == 0 {
		return "", fmt.Errorf("ch34x/lib: 当前平台 %s/%s 无内嵌动态库，请通过 CH34X_LIB_PATH 环境变量指定",
			runtime.GOOS, runtime.GOARCH)
	}

	fname := "libch347.so"
	if runtime.GOOS == "windows" {
		if runtime.GOARCH == "amd64" {
			fname = "CH347DLLA64.dll"
		} else {
			fname = "CH347DLL.dll"
		}
	}

	dest := filepath.Join(destDir, fname)
	if err := os.WriteFile(dest, data, 0755); err != nil {
		return "", fmt.Errorf("ch34x/lib: 提取内嵌库失败: %w", err)
	}
	return dest, nil
}

//go:build unix

// portUnix.go 是 OpenPort 的 Unix 实现：把 tty 路径交给 OpenSlave（含原始模式）并包装成 PtyLink。
package serialbridge

import (
	"errors"
	"fmt"
	"io/fs"
)

// OpenPort 打开一个 Unix tty 设备并返回带期限的 Link；Name 为空返回 ErrPortName。
// 路径不存在时返回 ErrPortNotFound；其余打开/配置错误原样返回（可用 errors.Is 判定）。
func OpenPort(cfg PortConfig) (Link, error) {
	if cfg.Name == "" {
		return nil, ErrPortName
	}
	file, err := OpenSlave(cfg.Name)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("%w: %s", ErrPortNotFound, cfg.Name)
		}
		return nil, err
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = DefaultReadTimeout
	}
	return NewPtyLink(file, timeout), nil
}

// 静态断言：PtyLink 是 Link 的平台实现。
var _ Link = (*PtyLink)(nil)

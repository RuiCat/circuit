//go:build windows

package dynlib

import (
	"fmt"
	"syscall"
	"unsafe"
)

// Load 使用 LoadLibrary 加载动态库。
func Load(path string) (Handle, error) {
	h, err := syscall.LoadLibrary(path)
	if err != nil {
		return nil, fmt.Errorf("dynlib: LoadLibrary %s 失败: %w", path, err)
	}
	return Handle(unsafe.Pointer(h)), nil
}

// Lookup 使用 GetProcAddress 查找符号地址。
func Lookup(h Handle, name string) (unsafe.Pointer, error) {
	ptr, err := syscall.GetProcAddress(syscall.Handle(h), name)
	if err != nil {
		return nil, fmt.Errorf("dynlib: GetProcAddress %s 失败: %w", name, err)
	}
	return ptr, nil
}

// Close 使用 FreeLibrary 卸载动态库。
func Close(h Handle) error {
	if h == nil {
		return nil
	}
	err := syscall.FreeLibrary(syscall.Handle(h))
	if err != nil {
		return fmt.Errorf("dynlib: FreeLibrary 失败: %w", err)
	}
	return nil
}

// Package dynlib 提供跨平台的动态库加载功能。
// Linux/macOS 通过 cgo 调用 dlopen/dlsym/dlclose，
// Windows 通过 syscall 调用 LoadLibrary/GetProcAddress/FreeLibrary。
package dynlib

import "unsafe"

// Handle 表示已加载的动态库句柄。
type Handle unsafe.Pointer

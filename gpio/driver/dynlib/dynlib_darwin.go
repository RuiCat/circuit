//go:build darwin

package dynlib

/*
#include <dlfcn.h>
#include <stdlib.h>
*/
import "C"
import (
	"fmt"
	"unsafe"
)

// Load 使用 dlopen 加载动态库。
func Load(path string) (Handle, error) {
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))
	h := C.dlopen(cPath, C.RTLD_NOW)
	if h == nil {
		errMsg := C.GoString(C.dlerror())
		return nil, fmt.Errorf("dynlib: dlopen %s 失败: %s", path, errMsg)
	}
	return Handle(h), nil
}

// Lookup 使用 dlsym 查找符号地址。
func Lookup(h Handle, name string) (unsafe.Pointer, error) {
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))
	ptr := C.dlsym(unsafe.Pointer(h), cName)
	if ptr == nil {
		errMsg := C.GoString(C.dlerror())
		return nil, fmt.Errorf("dynlib: dlsym %s 失败: %s", name, errMsg)
	}
	return ptr, nil
}

// Close 使用 dlclose 卸载动态库。
func Close(h Handle) error {
	if h == nil {
		return nil
	}
	ret := C.dlclose(unsafe.Pointer(h))
	if ret != 0 {
		errMsg := C.GoString(C.dlerror())
		return fmt.Errorf("dynlib: dlclose 失败: %s", errMsg)
	}
	return nil
}

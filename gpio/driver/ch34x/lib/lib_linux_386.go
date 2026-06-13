//go:build linux && 386

package lib

import _ "embed"

//go:embed x86/libch347.so
var soData []byte

func init() {
	data = soData
}

//go:build linux && mips64le

package lib

import _ "embed"

//go:embed mips64/libch347.so
var soData []byte

func init() {
	data = soData
}

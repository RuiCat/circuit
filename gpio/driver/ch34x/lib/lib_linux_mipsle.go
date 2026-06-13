//go:build linux && mipsle

package lib

import _ "embed"

//go:embed mips32/libch347.so
var soData []byte

func init() {
	data = soData
}

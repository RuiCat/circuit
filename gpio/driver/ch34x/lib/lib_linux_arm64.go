//go:build linux && arm64

package lib

import _ "embed"

//go:embed aarch64/libch347.so
var soData []byte

func init() {
	data = soData
}

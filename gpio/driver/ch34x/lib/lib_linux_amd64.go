//go:build linux && amd64

package lib

import _ "embed"

//go:embed x64/libch347.so
var soData []byte

func init() {
	data = soData
}

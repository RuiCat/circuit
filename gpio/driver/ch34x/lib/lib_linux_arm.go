//go:build linux && arm

package lib

import _ "embed"

//go:embed arm-gnueabihf/libch347.so
var soData []byte

func init() {
	data = soData
}

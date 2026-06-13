//go:build windows && amd64

package lib

import _ "embed"

//go:embed win/CH347DLLA64.dll
var soData []byte

func init() {
	data = soData
}

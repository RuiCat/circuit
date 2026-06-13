//go:build windows && 386

package lib

import _ "embed"

//go:embed win/CH347DLL.dll
var soData []byte

func init() {
	data = soData
}

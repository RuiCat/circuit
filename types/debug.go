package types

import "io"

// Debug 调试j接口
type Debug interface {
	Init(mna Stamp)
	IsDebug() bool
	SetDebug(is bool)
	Update(mna Stamp)
	Render(w io.Writer) error
	Error(err error)
}

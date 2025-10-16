package types

import "io"

// Debug 调试j接口
type Debug interface {
	Init(mna MNA)
	IsDebug() bool
	SetDebug(is bool)
	Update(mna MNA)
	Render(w io.Writer) error
	Error(err error)
}

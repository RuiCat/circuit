package mna

import (
	"io"
)

// Debug 调试j接口
type Debug interface {
	Init(mna *MNA)
	IsDebug() bool
	SetDebug(is bool)
	Update(mna *MNA)
	Render(w io.Writer) error
}

type debug struct{ is bool }

func (debug) Init(mna *MNA)            {}
func (debug *debug) IsDebug() bool     { return debug.is }
func (debug *debug) SetDebug(is bool)  { debug.is = is }
func (debug) Update(mna *MNA)          {}
func (debug) Render(w io.Writer) error { return nil }

package gluabit32

import (
	"github.com/yuin/gopher-lua"
)

// ----------------------------------------------------------------------------

var exports = map[string]lua.LGFunction{
	"band":   bandFn,
	"bnot":   bnotFn,
	"bor":    borFn,
	"bxor":   bxorFn,
	"lshift": lshiftFn,
	"rshift": rshiftFn,
}

// ----------------------------------------------------------------------------

func Loader(l *lua.LState) int {
	mod := l.SetFuncs(l.NewTable(), exports)
	l.Push(mod)
	return 1
}

// ----------------------------------------------------------------------------

func Preload(L *lua.LState) {
	L.PreloadModule("bit32", Loader)
}

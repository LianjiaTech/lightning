package gluabit32

import (
	"github.com/yuin/gopher-lua"
)

func bnotFn(L *lua.LState) int {
	a := L.CheckInt(1)
	result := ^a
	L.Push(lua.LNumber(result))
	return 1
}

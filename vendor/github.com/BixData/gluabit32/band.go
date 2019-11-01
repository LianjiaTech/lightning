package gluabit32

import (
	"github.com/yuin/gopher-lua"
)

func bandFn(L *lua.LState) int {
	a := L.CheckInt(1)
	b := L.CheckInt(2)
	result := a & b
	L.Push(lua.LNumber(result))
	return 1
}

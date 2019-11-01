package gluabit32

import (
	"github.com/yuin/gopher-lua"
)

func lshiftFn(L *lua.LState) int {
	a := L.CheckInt(1)
	b := L.CheckInt(2)
	result := a << uint(b)
	L.Push(lua.LNumber(result))
	return 1
}

package gluabit32

import (
	"github.com/yuin/gopher-lua"
)

// rshift (JavaScript >>> operator)
func rshiftFn(L *lua.LState) int {
	n := L.CheckInt64(1)
	n2 := L.CheckInt(2)
	result := n >> uint8(n2)
	L.Push(lua.LNumber(result))
	return 1
}

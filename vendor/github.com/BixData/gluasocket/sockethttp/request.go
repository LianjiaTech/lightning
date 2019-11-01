package gluasocket_sockethttp

import (
	"github.com/yuin/gopher-lua"
)

func requestFn(L *lua.LState) int {
	if L.Get(1).Type() == lua.LTString {
		return requestSimpleFn(L)
	}

	L.RaiseError("http.request() not implemented yet")
	return 0
}

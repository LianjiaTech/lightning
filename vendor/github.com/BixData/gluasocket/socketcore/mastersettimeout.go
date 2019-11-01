package gluasocket_socketcore

import (
	"time"

	"github.com/yuin/gopher-lua"
)

func masterSetTimeoutMethod(L *lua.LState) int {
	master := checkMaster(L)
	timeout := L.CheckNumber(2)
	master.Timeout = time.Duration(timeout * 1.0e9)
	return 0
}

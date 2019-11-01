package gluasocket_socketcore

import (
	"net"
	"time"

	"github.com/yuin/gopher-lua"
)

const (
	MASTER_TYPENAME = "tcp{master}"
)

type Master struct {
	Listener net.Listener
	BindAddr string
	BindPort lua.LValue
	Timeout  time.Duration
	Family   int
	Options  map[string]lua.LValue
}

var masterMethods = map[string]lua.LGFunction{
	"bind":       masterBindMethod,
	"close":      masterCloseMethod,
	"connect":    masterConnectMethod,
	"listen":     masterListenMethod,
	"setoption":  masterSetOptionMethod,
	"settimeout": masterSetTimeoutMethod,
}

// ----------------------------------------------------------------------------

func checkMaster(L *lua.LState) *Master {
	ud := L.CheckUserData(1)
	if v, ok := ud.Value.(*Master); ok {
		return v
	}
	L.ArgError(1, "master expected")
	return nil
}

package gluasocket_socketcore

import (
	"github.com/yuin/gopher-lua"
)

func dnsToHostName(l *lua.LState) int {
	l.RaiseError("socket.dns.tohostname(address) not implemented yet")
	return 0
}

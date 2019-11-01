package gluasocket

import (
	"github.com/BixData/gluasocket/ltn12"
	"github.com/BixData/gluasocket/mime"
	"github.com/BixData/gluasocket/mimecore"
	"github.com/BixData/gluasocket/socket"
	"github.com/BixData/gluasocket/socketcore"
	"github.com/BixData/gluasocket/socketexcept"
	"github.com/BixData/gluasocket/socketftp"
	"github.com/BixData/gluasocket/socketheaders"
	"github.com/BixData/gluasocket/sockethttp"
	"github.com/BixData/gluasocket/socketsmtp"
	"github.com/BixData/gluasocket/sockettp"
	"github.com/BixData/gluasocket/socketurl"
	"github.com/yuin/gopher-lua"
)

func Preload(L *lua.LState) {
	L.PreloadModule("ltn12", gluasocket_ltn12.Loader)
	L.PreloadModule("mime.core", gluasocket_mimecore.Loader)
	L.PreloadModule("mime", gluasocket_mime.Loader)
	L.PreloadModule("socket", gluasocket_socket.Loader)
	L.PreloadModule("socket.core", gluasocket_socketcore.Loader)
	L.PreloadModule("socket.except", gluasocket_socketexcept.Loader)
	L.PreloadModule("socket.ftp", gluasocket_socketftp.Loader)
	L.PreloadModule("socket.headers", gluasocket_socketheaders.Loader)
	L.PreloadModule("socket.http", gluasocket_sockethttp.Loader)
	L.PreloadModule("socket.smtp", gluasocket_socketsmtp.Loader)
	L.PreloadModule("socket.tp", gluasocket_sockettp.Loader)
	L.PreloadModule("socket.url", gluasocket_socketurl.Loader)
}

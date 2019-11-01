package gluadb

import (
	"github.com/yuin/gopher-lua"

	"github.com/zhu327/gluadb/mysql"
	"github.com/zhu327/gluadb/ngx"
	"github.com/zhu327/gluadb/redis"
)

func Preload(L *lua.LState) {
	L.PreloadModule("ngx", ngx.Loader)
	L.PreloadModule("mysql", mysql.Loader)
	L.PreloadModule("redis", redis.Loader)
}

# glua-db
MySQL and Redis client for gopher lua

## require

```shell
go get github.com/zhu327/gluadb
```

## document

<https://github.com/openresty/lua-resty-mysql>

<https://github.com/openresty/lua-resty-redis>

## example

```go
package main

import (
    "github.com/BixData/gluabit32"
    "github.com/BixData/gluasocket"
    "github.com/yuin/gopher-lua"
    "github.com/zhu327/gluadb"
)

func main() {
    L := lua.NewState()
    gluasocket.Preload(L)
    gluabit32.Preload(L)
    gluadb.Preload(L)
    defer L.Close()
    if err := L.DoString(`

    local mysql = require "mysql"
    local db = mysql:new()

    local ok, err, errcode, sqlstate = db:connect{
        host = "127.0.0.1",
        port = 3306,
        database = "mysql",
        user = "root",
        password = "",
        charset = "utf8",
        max_packet_size = 1024 * 1024,
    }

    local res, err, errcode, sqlstate =
        db:query("select * from db", 10)

    print(#res)
    print(res[1].Host)

    db:close()

    local redis = require "redis"
    local red = redis:new()
    local ok, err = red:connect("127.0.0.1", 6379)
    ok, err = red:set("dog", "an animal")
    local res, err = red:get("dog")
    print(res)

    `); err != nil {
        panic(err)
    }
}
```

package lfs // import "layeh.com/gopher-lfs"

import (
	"github.com/yuin/gopher-lua"
)

// Preload adds lfs to the given Lua state's package.preload table. After it
// has been preloaded, it can be loaded using require:
//
//  local lfs = require("lfs")
func Preload(L *lua.LState) {
	L.PreloadModule("lfs", load)
}

func load(L *lua.LState) int {
	t := L.NewTable()
	L.SetFuncs(t, api)
	L.Push(t)
	return 1
}

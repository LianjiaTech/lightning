package ngx

import (
	"github.com/yuin/gopher-lua"

	"crypto/sha1"
	"encoding/base64"
	"fmt"
)

type Null struct{}

func Loader(L *lua.LState) int {
	tb := L.NewTable()
	L.SetFuncs(tb, map[string]lua.LGFunction{
		"sha1_bin":      sha1_bin,
		"decode_base64": base64_decode,
		"encode_base64": base64_encode,
		"tohex":         to_hex,
	})

	ud := L.NewUserData()
	ud.Value = &Null{}
	L.SetField(tb, "null", ud)

	L.Push(tb)
	return 1
}

func sha1_bin(L *lua.LState) int {
	lv := L.CheckString(1)
	hash := sha1.Sum([]byte(lv))
	L.Push(lua.LString(string(hash[:])))
	return 1
}

func base64_encode(L *lua.LState) int {
	lv := L.CheckString(1)
	str := base64.StdEncoding.EncodeToString([]byte(lv))
	L.Push(lua.LString(str))
	return 1
}

func base64_decode(L *lua.LState) int {
	lv := L.CheckString(1)
	data, err := base64.StdEncoding.DecodeString(lv)
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString(err.Error()))
		return 2
	}
	L.Push(lua.LString(string(data)))
	return 1
}

func to_hex(L *lua.LState) int {
	lv := L.CheckInt(1)
	hex := fmt.Sprintf("%08x", lv)
	L.Push(lua.LString(hex))
	return 1
}

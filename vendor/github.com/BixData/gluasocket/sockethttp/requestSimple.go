package gluasocket_sockethttp

import (
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/yuin/gopher-lua"
)

func requestSimpleFn(L *lua.LState) int {
	httpClient := http.Client{Timeout: time.Second * 15}
	urlParam := L.ToString(1)

	// LuaSocket allows webhdfs://
	if parsedUrl, err := url.Parse(urlParam); err != nil {
		L.RaiseError(err.Error())
		return 0
	} else {
		if parsedUrl.Scheme == "webhdfs" {
			parsedUrl.Scheme = "http"
		}
		urlParam = parsedUrl.String()
	}

	var res *http.Response
	var err error
	if L.Get(2).Type() == lua.LTNil {
		res, err = httpClient.Get(urlParam)
	} else {
		body := L.ToString(2)
		res, err = httpClient.Post(urlParam, "text/plain", strings.NewReader(body))
	}
	if err != nil {
		L.RaiseError(err.Error())
		return 0
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		L.RaiseError(err.Error())
		return 0
	}

	L.Push(lua.LString(string(body)))
	headers := createHeadersTable(L, res.Header)
	L.Push(headers)
	L.Push(lua.LNumber(res.StatusCode))
	return 3
}

func createHeadersTable(L *lua.LState, header http.Header) *lua.LTable {
	table := L.NewTable()
	for name, value := range header {
		table.RawSetString(strings.ToLower(name), lua.LString(strings.Join(value, "\n")))
	}
	return table
}

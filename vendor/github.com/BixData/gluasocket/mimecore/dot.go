package gluasocket_mimecore

import (
	"bytes"

	"github.com/yuin/gopher-lua"
)

/*-------------------------------------------------------------------------*\
* Incrementally applies smtp stuffing to a string
* A, n = dot(l, D)
\*-------------------------------------------------------------------------*/
func dotFn(l *lua.LState) int {
	state := l.ToNumber(1)
	input := l.ToString(2)
	var buffer bytes.Buffer
	for _, c := range input {
		state = dot(c, state, &buffer)
	}
	l.Push(lua.LString(buffer.String()))
	l.Push(state)
	return 2
}

/*-------------------------------------------------------------------------*\
* Takes one byte and stuff it if needed.
\*-------------------------------------------------------------------------*/
func dot(c rune, state lua.LNumber, buffer *bytes.Buffer) lua.LNumber {
	buffer.WriteRune(c)
	switch c {
	case '\r':
		return 1
	case '\n':
		if state == 1 {
			return 2
		} else {
			return 0
		}
	case '.':
		if state == 2 {
			buffer.WriteRune('.')
		}
	}
	return 0
}

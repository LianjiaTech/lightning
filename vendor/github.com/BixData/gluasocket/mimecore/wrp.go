package gluasocket_mimecore

import (
	"bytes"

	"github.com/yuin/gopher-lua"
)

/*-------------------------------------------------------------------------*\
* Incrementaly breaks a string into lines. The string can have CRLF breaks.
* A, n = wrp(l, B, length)
* A is a copy of B, broken into lines of at most 'length' bytes.
* 'l' is how many bytes are left for the first line of B.
* 'n' is the number of bytes left in the last line of A.
\*-------------------------------------------------------------------------*/
func wrpFn(L *lua.LState) int {
	left := L.ToNumber(1)
	length := L.OptNumber(3, lua.LNumber(76))

	/* end of input black-hole */
	if L.Get(2).Type() == lua.LTNil {
		if left < length {
			/* if last line has not been terminated, add a line break */
			L.Push(lua.LString("\r\n"))
		} else {
			/* otherwise, we are done */
			L.Push(lua.LNil)
		}
		L.Push(length)
		return 2
	}

	var buffer bytes.Buffer

	input := L.ToString(2)
	for _, c := range input {
		//    switch (*input) {
		switch c {
		case '\r':
			break
		case '\n':
			buffer.WriteString("\r\n")
			left = length
			break
		default:
			if left <= 0 {
				left = length
				buffer.WriteString("\r\n")
			}
			buffer.WriteRune(c)
			left--
			break
		}
	}
	L.Push(lua.LString(buffer.String()))
	L.Push(left)
	return 2
}

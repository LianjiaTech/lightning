package gluasocket_mimecore

import (
	"bytes"

	"github.com/yuin/gopher-lua"
)

/*-------------------------------------------------------------------------*\
* Incrementally breaks a quoted-printed string into lines
* A, n = qpwrp(l, B, length)
* A is a copy of B, broken into lines of at most 'length' bytes.
* 'l' is how many bytes are left for the first line of B.
* 'n' is the number of bytes left in the last line of A.
* There are two complications: lines can't be broken in the middle
* of an encoded =XX, and there might be line breaks already
\*-------------------------------------------------------------------------*/
func qpwrpFn(L *lua.LState) int {
	left := L.ToNumber(1)

	inputArg := L.Get(2)
	var input string
	if inputArg.Type() != lua.LTNil {
		input = inputArg.String()
	}

	lengthArg := L.Get(3)
	var length lua.LNumber
	if lengthArg.Type() == lua.LTNumber {
		length = lengthArg.(lua.LNumber)
	} else {
		length = 76
	}

	var buffer bytes.Buffer

	// end-of-input blackhole
	if inputArg.Type() == lua.LTNil {
		if left < length {
			L.Push(lua.LString("=\r\n"))
		} else {
			L.Push(lua.LNil)
		}
		L.Push(lua.LNumber(length))
		return 2
	}

	// process all input
	for _, c := range input {
		switch c {
		case '\r':
			break
		case '\n':
			left = length
			buffer.WriteString("\r\n")
			break
		case '=':
			if left <= 3 {
				left = length
				buffer.WriteString("=\r\n")
			}
			buffer.WriteRune(c)
			left--
			break
		default:
			if left <= 1 {
				left = length
				buffer.WriteString("=\r\n")
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

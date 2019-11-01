package gluasocket_mimecore

import (
	"bytes"

	"github.com/yuin/gopher-lua"
)

/*-------------------------------------------------------------------------*\
* Converts a string to uniform EOL convention.
* A, n = eol(o, B, marker)
* A is the converted version of the largest prefix of B that can be
* converted unambiguously. 'o' is the context returned by the previous
* call. 'n' is the new context.
\*-------------------------------------------------------------------------*/
func eolFn(L *lua.LState) int {
	ctx := rune(L.OptInt(1, 1))

	/* end of input blackhole */
	if L.Get(2).Type() == lua.LTNil {
		L.Push(lua.LNil)
		L.Push(lua.LNumber(0))
		return 2
	}

	/* process all input */
	var buffer bytes.Buffer
	marker := L.OptString(3, "\r\n")
	input := L.ToString(2)
	for _, c := range input {
		ctx = eolprocess(c, ctx, marker, &buffer)
	}

	L.Push(lua.LString(buffer.String()))
	L.Push(lua.LNumber(ctx))
	return 2
}

/*-------------------------------------------------------------------------*\
* Here is what we do: \n, and \r are considered candidates for line
* break. We issue *one* new line marker if any of them is seen alone, or
* followed by a different one. That is, \n\n and \r\r will issue two
* end of line markers each, but \r\n, \n\r etc will only issue *one*
* marker.  This covers Mac OS, Mac OS X, VMS, Unix and DOS, as well as
* probably other more obscure conventions.
*
* c is the current character being processed
* last is the previous character
\*-------------------------------------------------------------------------*/
func eolprocess(c rune, last rune, marker string, buffer *bytes.Buffer) rune {
	if c == '\r' || c == '\n' {
		if last == '\r' || last == '\n' {
			if c == last {
				buffer.WriteString(marker)
			}
			return 0
		} else {
			buffer.WriteString(marker)
			return c
		}
	} else {
		buffer.WriteRune(c)
		return 0
	}
}

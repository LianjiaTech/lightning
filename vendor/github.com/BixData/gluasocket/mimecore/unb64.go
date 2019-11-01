package gluasocket_mimecore

import (
	"bytes"

	"github.com/yuin/gopher-lua"
)

/*-------------------------------------------------------------------------*\
* Incrementally removes the Base64 transfer content encoding from a string
* A, B = b64(C, D)
* A is the encoded version of the largest prefix of C .. D that is
* divisible by 4. B has the remaining bytes of C .. D, *without* encoding.
\*-------------------------------------------------------------------------*/
func unb64Fn(L *lua.LState) int {
	/* end-of-input blackhole */
	if L.Get(1).Type() == lua.LTNil {
		L.Push(lua.LNil)
		L.Push(lua.LNil)
		return 2
	}

	/* process first part of the input */
	input := L.ToString(1)
	var atom, buffer bytes.Buffer
	for _, c := range input {
		b64decode(c, &atom, &buffer)
	}

	if L.Get(2).Type() == lua.LTNil {
		if buffer.Len() == 0 {
			L.Push(lua.LNil)
		} else {
			L.Push(lua.LString(buffer.String()))
		}
		L.Push(lua.LNil)
		return 2
	}

	/* otherwise, process the rest of the input */
	input = L.ToString(2)
	for _, c := range input {
		b64decode(c, &atom, &buffer)
	}

	L.Push(lua.LString(buffer.String()))
	L.Push(lua.LString(atom.String()))
	return 2
}

/*-------------------------------------------------------------------------*\
* Acumulates bytes in input buffer until 4 bytes are available.
* Translate the 4 bytes from Base64 form and append to buffer.
* Returns new number of bytes in buffer.
\*-------------------------------------------------------------------------*/
func b64decode(c rune, input *bytes.Buffer, buffer *bytes.Buffer) {
	/* ignore invalid characters */
	if b64unbase[c] > 64 {
		return
	}
	input.WriteRune(c)

	/* decode atom */
	if input.Len() == 4 {
		var decoded [3]byte
		var value uint64
		var valid int

		input0 := input.Next(1)[0]
		value = uint64(b64unbase[input0])
		value <<= 6

		input1 := input.Next(1)[0]
		value |= uint64(b64unbase[input1])
		value <<= 6

		input2 := input.Next(1)[0]
		value |= uint64(b64unbase[input2])
		value <<= 6

		input3 := input.Next(1)[0]
		value |= uint64(b64unbase[input3])

		decoded[2] = byte(value & 0xff)

		value >>= 8
		decoded[1] = byte(value & 0xff)

		value >>= 8
		decoded[0] = byte(value)

		/* take care of padding */
		if input2 == '=' {
			valid = 1
		} else {
			if input3 == '=' {
				valid = 2
			} else {
				valid = 3
			}
		}

		for i := 0; i < valid; i++ {
			buffer.WriteByte(decoded[i])
		}
	}
}

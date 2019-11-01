package gluasocket_mimecore

import (
	"bytes"

	"github.com/yuin/gopher-lua"
)

var (
	b64base   = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	b64unbase [256]byte
)

/*-------------------------------------------------------------------------*\
* Fill base64 decode map.
\*-------------------------------------------------------------------------*/
func b64setup() {
	for i := 0; i <= 255; i++ {
		b64unbase[i] = 255
	}
	for i := 0; i < 64; i++ {
		b64unbase[b64base[i]] = byte(i)
	}
	b64unbase['='] = 0
}

/*-------------------------------------------------------------------------*\
* Incrementally applies the Base64 transfer content encoding to a string
* A, B = b64(C, D)
* A is the encoded version of the largest prefix of C .. D that is
* divisible by 3. B has the remaining bytes of C .. D, *without* encoding.
* The easiest thing would be to concatenate the two strings and
* encode the result, but we can't afford that or Lua would dupplicate
* every chunk we received.
\*-------------------------------------------------------------------------*/
func b64Fn(L *lua.LState) int {
	var atom bytes.Buffer

	///* end-of-input blackhole */
	if L.Get(1).Type() == lua.LTNil {
		L.Push(lua.LNil)
		L.Push(lua.LNil)
		return 2
	}

	input := L.ToString(1)

	/* process first part of the input */
	var buffer bytes.Buffer
	for i := 0; i < len(input); i++ {
		b64encode(input[i], &atom, &buffer)
	}

	/* if second part is nil, we are done */
	if L.Get(2).Type() == lua.LTNil {
		b64pad(atom, &buffer)
		if buffer.Len() == 0 {
			L.Push(lua.LNil)
		} else {
			L.Push(lua.LString(buffer.String()))
		}
		L.Push(lua.LNil)
		return 2
	}

	/* otherwise process the second part */
	input = L.ToString(2)
	for i := 0; i < len(input); i++ {
		b64encode(input[i], &atom, &buffer)
	}
	L.Push(lua.LString(buffer.String()))
	L.Push(lua.LString(atom.String()))
	return 2
}

/*-------------------------------------------------------------------------*\
* Acumulates bytes in input buffer until 3 bytes are available.
* Translate the 3 bytes into Base64 form and append to buffer.
* Returns new number of bytes in buffer.
\*-------------------------------------------------------------------------*/
func b64encode(c byte, input *bytes.Buffer, buffer *bytes.Buffer) {
	input.WriteByte(c)
	if input.Len() == 3 {
		var code [4]byte
		var value uint32

		value += uint32(input.Next(1)[0])
		value <<= 8
		value += uint32(input.Next(1)[0])
		value <<= 8
		value += uint32(input.Next(1)[0])
		code[3] = b64base[value&0x3f]

		value >>= 6
		code[2] = b64base[value&0x3f]

		value >>= 6
		code[1] = b64base[value&0x3f]

		value >>= 6
		code[0] = b64base[value]

		buffer.WriteString(string(code[:]))
	}
}

/*-------------------------------------------------------------------------*\
* Encodes the Base64 last 1 or 2 bytes and adds padding '='
* Result, if any, is appended to buffer.
* Returns 0.
\*-------------------------------------------------------------------------*/
func b64pad(input bytes.Buffer, buffer *bytes.Buffer) {
	var value uint64
	code := []byte("====")
	switch input.Len() {
	case 1:
		value = uint64(input.Next(1)[0]) << 4
		code[1] = b64base[value&0x3f]

		value >>= 6
		code[0] = b64base[value]

		buffer.WriteString(string(code))
		break
	case 2:
		value = uint64(input.Next(1)[0]) << 8
		value |= uint64(input.Next(1)[0])
		value <<= 2
		code[2] = b64base[value&0x3f]

		value >>= 6
		code[1] = b64base[value&0x3f]

		value >>= 6
		code[0] = b64base[value]

		buffer.WriteString(string(code))
		break
	default:
		break
	}
}

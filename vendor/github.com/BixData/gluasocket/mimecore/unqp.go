package gluasocket_mimecore

import (
	"bytes"

	"github.com/yuin/gopher-lua"
)

/*-------------------------------------------------------------------------*\
* Incrementally decodes a string in quoted-printable
* A, B = qp(C, D)
* A is the decoded version of the largest prefix of C .. D that
* can be decoded without doubts.
* B has the remaining bytes of C .. D, *without* decoding.
\*-------------------------------------------------------------------------*/
func unqpFn(l *lua.LState) int {
	var atom bytes.Buffer

	if l.Get(1).Type() == lua.LTNil {
		l.Push(lua.LNil)
		l.Push(lua.LNil)
		return 2
	}

	input := l.ToString(1)
	var buffer bytes.Buffer

	/* process first part of input */
	for _, c := range input {
		qpdecode(c, &atom, &buffer)
	}

	/* if second part is nil, we are done */
	if l.Get(2).Type() == lua.LTNil {
		if buffer.Len() == 0 {
			l.Push(lua.LNil)
		} else {
			l.Push(lua.LString(buffer.String()))
		}
		l.Push(lua.LNil)
		return 2
	}

	/* otherwise process rest of input */
	input = l.ToString(2)
	for _, c := range input {
		qpdecode(c, &atom, &buffer)
	}

	l.Push(lua.LString(buffer.String()))
	l.Push(lua.LString(atom.String()))
	return 2
}

/*-------------------------------------------------------------------------*\
* Accumulate characters until we are sure about how to deal with them.
* Once we are sure, output the to the buffer, in the correct form.
\*-------------------------------------------------------------------------*/
func qpdecode(c rune, input *bytes.Buffer, buffer *bytes.Buffer) {
	input.WriteRune(c)

	/* deal with all characters we can deal */
	inputBytes := input.Bytes()
	switch inputBytes[0] {
	/* if we have an escape character */
	case '=':
		if len(inputBytes) < 3 {
			return
		}
		/* eliminate soft line break */
		if inputBytes[1] == '\r' && inputBytes[2] == '\n' {
			return
		}
		/* decode quoted representation */
		c_ := qpunbase[inputBytes[1]]
		d := qpunbase[inputBytes[2]]

		/* if it is an invalid, do not decode */
		if c_ > 15 || d > 15 {
			buffer.WriteByte(inputBytes[0])
			buffer.WriteByte(inputBytes[1])
			buffer.WriteByte(inputBytes[2])
			input.Next(3)
		} else {
			buffer.WriteByte((c_ << 4) + d)
			input.Next(3)
			return
		}
	case '\r':
		if len(inputBytes) < 2 {
			return
		}
		if inputBytes[1] == '\n' {
			buffer.WriteByte(inputBytes[0])
			buffer.WriteByte(inputBytes[1])
			input.Next(2)
		}
		return
	default:
		if inputBytes[0] == '\t' || (inputBytes[0] > 31 && inputBytes[0] < 127) {
			buffer.WriteByte(inputBytes[0])
			input.Next(1)
		}
	}
}

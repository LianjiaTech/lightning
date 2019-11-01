package gluasocket_mimecore

import (
	"bytes"

	"github.com/yuin/gopher-lua"
)

const (
	qpbase = "0123456789ABCDEF"

	QP_PLAIN = iota
	QP_QUOTED
	QP_CR
	QP_IF_LAST
)

var (
	qpclass, qpunbase [256]byte
)

/*-------------------------------------------------------------------------*\
* Incrementally converts a string to quoted-printable
* A, B = qp(C, D, marker)
* Marker is the text to be used to replace CRLF sequences found in A.
* A is the encoded version of the largest prefix of C .. D that
* can be encoded without doubts.
* B has the remaining bytes of C .. D, *without* encoding.
\*-------------------------------------------------------------------------*/
func qpFn(l *lua.LState) int {
	var atom bytes.Buffer

	// end-of-input blackhole
	if l.Get(1).Type() == lua.LTNil {
		l.Push(lua.LNil)
		l.Push(lua.LNil)
		return 2
	}

	input := l.ToString(1)

	marker := "\r\n"
	if l.Get(3).Type() == lua.LTString {
		marker = l.ToString(3)
	}

	// process first part of input
	var buffer bytes.Buffer
	for i := 0; i < len(input); i++ {
		qpencode(input[i], &atom, marker, &buffer)
	}

	// if second part is nil, we are done
	if l.Get(2).Type() == lua.LTNil {
		qppad(atom, &buffer)
		if buffer.Len() == 0 {
			l.Push(lua.LNil)
		} else {
			l.Push(lua.LString(buffer.String()))
		}
		l.Push(lua.LNil)
		return 2
	}

	// otherwise process rest of input
	input = l.ToString(2)
	for i := 0; i < len(input); i++ {
		qpencode(input[i], &atom, marker, &buffer)
	}

	l.Push(lua.LString(buffer.String()))
	l.Push(lua.LString(atom.String()))
	return 2
}

/*-------------------------------------------------------------------------*\
* Split quoted-printable characters into classes
* Precompute reverse map for encoding
\*-------------------------------------------------------------------------*/
func qpsetup() {
	for i := 0; i < 256; i++ {
		qpclass[i] = QP_QUOTED
	}
	for i := 33; i <= 60; i++ {
		qpclass[i] = QP_PLAIN
	}
	for i := 62; i <= 126; i++ {
		qpclass[i] = QP_PLAIN
	}

	qpclass['\t'] = QP_IF_LAST
	qpclass[' '] = QP_IF_LAST
	qpclass['\r'] = QP_CR

	for i := 0; i < 256; i++ {
		qpunbase[i] = 255
	}
	qpunbase['0'] = 0
	qpunbase['1'] = 1
	qpunbase['2'] = 2
	qpunbase['3'] = 3
	qpunbase['4'] = 4
	qpunbase['5'] = 5
	qpunbase['6'] = 6
	qpunbase['7'] = 7
	qpunbase['8'] = 8
	qpunbase['9'] = 9
	qpunbase['A'] = 10
	qpunbase['a'] = 10
	qpunbase['B'] = 11
	qpunbase['b'] = 11
	qpunbase['C'] = 12
	qpunbase['c'] = 12
	qpunbase['D'] = 13
	qpunbase['d'] = 13
	qpunbase['e'] = 14
	qpunbase['e'] = 14
	qpunbase['F'] = 15
	qpunbase['f'] = 15
}

/*-------------------------------------------------------------------------*\
* Accumulate characters until we are sure about how to deal with them.
* Once we are sure, output to the buffer, in the correct form.
\*-------------------------------------------------------------------------*/
func qpencode(c byte, input *bytes.Buffer, marker string, buffer *bytes.Buffer) {
	input.WriteByte(c)

	// deal with all characters we can have
	for input.Len() > 0 {
		inputBytes := input.Bytes()
		klass := qpclass[inputBytes[0]]

		switch klass {

		// might be the CR of a CRLF sequence
		case QP_CR:
			if len(inputBytes) < 2 {
				return
			}
			if inputBytes[1] == '\n' {
				buffer.WriteString(marker)
				input.Next(2)
				return
			} else {
				qpquote(inputBytes[0], buffer)
			}
			break
			// might be a space and that has to be quoted if last in line
		case QP_IF_LAST:
			if len(inputBytes) < 3 {
				return
			}
			// if it is the last, quote it and we are done
			if inputBytes[1] == '\r' && inputBytes[2] == '\n' {
				qpquote(inputBytes[0], buffer)
				buffer.WriteString(marker)
				input.Next(2)
				return
			} else {
				buffer.WriteByte(inputBytes[0])
			}
			break
			// might have to be quoted always
		case QP_QUOTED:
			qpquote(inputBytes[0], buffer)
			break
			// might never have to be quoted
		default:
			buffer.WriteByte(inputBytes[0])
			break
		}
		input.Next(1)
	}
}

/*-------------------------------------------------------------------------*\
* Deal with the final characters
\*-------------------------------------------------------------------------*/
func qppad(input bytes.Buffer, buffer *bytes.Buffer) int {
	for _, c := range input.Bytes() {
		if qpclass[c] == QP_PLAIN {
			buffer.WriteByte(c)
		} else {
			qpquote(c, buffer)
		}
	}
	if input.Len() > 0 {
		buffer.WriteString("=\r\n")
	}
	return 0
}

/*-------------------------------------------------------------------------*\
* Output one character in form =XX
\*-------------------------------------------------------------------------*/
func qpquote(c byte, buffer *bytes.Buffer) {
	buffer.WriteRune('=')
	buffer.WriteByte(qpbase[c>>4])
	buffer.WriteByte(qpbase[c&0x0f])
}

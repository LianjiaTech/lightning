/*
 * Copyright(c)  2019 Lianjia, Inc.  All Rights Reserved
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *     http://www.apache.org/licenses/LICENSE-2.0
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package event

import (
	"errors"
	"math/bits"

	"github.com/go-mysql-org/go-mysql/mysql"
	"github.com/go-mysql-org/go-mysql/replication"
)

var errRowCount = errors.New("cannot count rows without decoding")

// countRowsEvent is the rows event decoder used by the stat plugin. It walks
// the row images and only counts them instead of materializing every column
// value. Anything the walker does not understand (partial JSON updates,
// MariaDB compressed rows, unknown column types, truncated data) falls back to
// the regular decoder, so the count is always correct.
func countRowsEvent(re *replication.RowsEvent, data []byte) error {
	switch re.Type() {
	case replication.EnumRowsEventTypeInsert, replication.EnumRowsEventTypeUpdate, replication.EnumRowsEventTypeDelete:
		if n, err := countRows(re, data); err == nil && n > 0 {
			// Rows is consumed as len(Rows); keep the slice headers only.
			re.Rows = make([][]any, n)
			return nil
		}
	}
	return re.Decode(data)
}

func countRows(re *replication.RowsEvent, data []byte) (int, error) {
	pos, err := re.DecodeHeader(data)
	if err != nil {
		return 0, err
	}
	if pos >= len(data) || len(re.ColumnBitmap1) == 0 || re.Table == nil {
		return 0, errRowCount
	}

	bitmap1 := re.ColumnBitmap1
	bitmap2 := re.ColumnBitmap2
	// UPDATE events carry a before and an after image, and Rows holds one
	// entry per image (matching the full decoder and the stat counters).
	imagesPerRow := 1
	if len(bitmap2) > 0 {
		imagesPerRow = 2
	}
	rows := 0
	for pos < len(data) {
		n, err := skipRowImage(re, data[pos:], bitmap1)
		if err != nil {
			return 0, err
		}
		if n <= 0 {
			return 0, errRowCount
		}
		pos += n

		if len(bitmap2) > 0 {
			n, err = skipRowImage(re, data[pos:], bitmap2)
			if err != nil {
				return 0, err
			}
			if n <= 0 {
				return 0, errRowCount
			}
			pos += n
		}
		rows += imagesPerRow
	}
	if pos != len(data) {
		return 0, errRowCount
	}
	return rows, nil
}

// skipRowImage returns the number of bytes of one row image.
func skipRowImage(re *replication.RowsEvent, data []byte, bitmap []byte) (int, error) {
	columns := int(re.ColumnCount)
	present := 0
	for i := 0; i+8 <= columns; i += 8 {
		present += bits.OnesCount8(bitmap[i>>3])
	}
	if rem := columns & 7; rem > 0 {
		present += bits.OnesCount8(bitmap[columns>>3] & byte((1<<rem)-1))
	}

	// the null bitmap only covers the columns present in this image
	nullLen := (present + 7) / 8
	if len(data) < nullLen {
		return 0, errRowCount
	}
	nullBitmap := data[:nullLen]
	pos := nullLen
	nullIndex := 0

	for i := 0; i < columns; i++ {
		if !isBitSet(bitmap, i) {
			continue
		}
		isNull := isBitSet(nullBitmap, nullIndex)
		nullIndex++
		if isNull {
			continue
		}
		if i >= len(re.Table.ColumnType) || i >= len(re.Table.ColumnMeta) {
			return 0, errRowCount
		}
		size, err := skipValueSize(data[pos:], re.Table.ColumnType[i], re.Table.ColumnMeta[i])
		if err != nil || pos+size > len(data) {
			return 0, errRowCount
		}
		pos += size
	}
	return pos, nil
}

func isBitSet(bitmap []byte, i int) bool {
	return bitmap[i>>3]&(1<<(uint(i)&7)) > 0
}

// decimalBytes is MySQL's dig2bytes table: storage bytes for a partial group.
var decimalBytes = [10]int{0, 1, 1, 2, 2, 3, 3, 4, 4, 4}

// skipValueSize returns the encoded size of one column value.
func skipValueSize(data []byte, tp byte, meta uint16) (int, error) {
	length := 0
	if tp == mysql.MYSQL_TYPE_STRING {
		if meta >= 256 {
			b0 := uint8(meta >> 8)
			b1 := uint8(meta & 0xFF)
			if b0&0x30 != 0x30 {
				length = int(uint16(b1) | (uint16((b0&0x30)^0x30) << 4))
				tp = b0 | 0x30
			} else {
				length = int(meta & 0xFF)
				tp = b0
			}
		} else {
			length = int(meta)
		}
	}

	switch tp {
	case mysql.MYSQL_TYPE_NULL:
		return 0, nil
	case mysql.MYSQL_TYPE_TINY:
		return needBytes(data, 1)
	case mysql.MYSQL_TYPE_SHORT:
		return needBytes(data, 2)
	case mysql.MYSQL_TYPE_INT24:
		return needBytes(data, 3)
	case mysql.MYSQL_TYPE_LONG, mysql.MYSQL_TYPE_FLOAT, mysql.MYSQL_TYPE_TIMESTAMP:
		return needBytes(data, 4)
	case mysql.MYSQL_TYPE_LONGLONG, mysql.MYSQL_TYPE_DOUBLE, mysql.MYSQL_TYPE_DATETIME:
		return needBytes(data, 8)
	case mysql.MYSQL_TYPE_NEWDECIMAL:
		precision := int(meta >> 8)
		scale := int(meta & 0xFF)
		integral := precision - scale
		uncompIntegral := integral / 9
		uncompFractional := scale / 9
		compIntegral := integral - uncompIntegral*9
		compFractional := scale - uncompFractional*9
		size := uncompIntegral*4 + decimalBytes[compIntegral] +
			uncompFractional*4 + decimalBytes[compFractional]
		return needBytes(data, size)
	case mysql.MYSQL_TYPE_BIT:
		nbits := (meta>>8)*8 + (meta & 0xFF)
		return needBytes(data, int(nbits+7)/8)
	case mysql.MYSQL_TYPE_TIMESTAMP2:
		return needBytes(data, 4+int((meta+1)/2))
	case mysql.MYSQL_TYPE_DATETIME2:
		return needBytes(data, 5+int((meta+1)/2))
	case mysql.MYSQL_TYPE_TIME2:
		return needBytes(data, 3+int((meta+1)/2))
	case mysql.MYSQL_TYPE_TIME, mysql.MYSQL_TYPE_DATE:
		return needBytes(data, 3)
	case mysql.MYSQL_TYPE_YEAR:
		return needBytes(data, 1)
	case mysql.MYSQL_TYPE_ENUM, mysql.MYSQL_TYPE_SET:
		return needBytes(data, int(meta&0xFF))
	case mysql.MYSQL_TYPE_BLOB, mysql.MYSQL_TYPE_GEOMETRY, mysql.MYSQL_TYPE_VECTOR:
		return blobSize(data, meta)
	case mysql.MYSQL_TYPE_VARCHAR, mysql.MYSQL_TYPE_VAR_STRING:
		return stringSize(data, int(meta))
	case mysql.MYSQL_TYPE_STRING:
		return stringSize(data, length)
	case mysql.MYSQL_TYPE_JSON:
		if meta == 0 || int(meta) > 4 || len(data) < int(meta) {
			return 0, errRowCount
		}
		length = int(mysql.FixedLengthInt(data[:meta]))
		return needBytes(data, length+int(meta))
	default:
		return 0, errRowCount
	}
}

func needBytes(data []byte, n int) (int, error) {
	if n < 0 || n > len(data) {
		return 0, errRowCount
	}
	return n, nil
}

func blobSize(data []byte, meta uint16) (int, error) {
	if meta == 0 || meta > 4 || int(meta) > len(data) {
		return 0, errRowCount
	}
	return needBytes(data, int(mysql.FixedLengthInt(data[:meta]))+int(meta))
}

func stringSize(data []byte, length int) (int, error) {
	if length < 256 {
		if len(data) < 1 {
			return 0, errRowCount
		}
		return needBytes(data, int(data[0])+1)
	}
	if len(data) < 2 {
		return 0, errRowCount
	}
	return needBytes(data, int(uint16(data[0])|uint16(data[1])<<8)+2)
}

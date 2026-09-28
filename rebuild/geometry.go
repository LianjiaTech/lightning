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

package rebuild

import "encoding/binary"

// MySQL 8.0+ prefixes the binlog geometry value with a 4-byte little-endian
// SRID followed by the WKB. MySQL 5.7 and earlier store the bare WKB.
// Because an SRID of 0 makes the value start with 0x00 (also a valid WKB byte
// order marker), splitGeometry validates the WKB length to disambiguate.

// splitGeometry splits a geometry binlog value into WKB and SRID.
// hasSRID reports whether the 4-byte SRID prefix was detected.
func splitGeometry(b []byte) (wkb []byte, srid uint32, hasSRID bool) {
	if len(b) >= 5 && wkbLength(b[4:]) == len(b)-4 {
		return b[4:], binary.LittleEndian.Uint32(b[:4]), true
	}
	return b, 0, false
}

// wkbLength returns the total byte length of the WKB value starting at b,
// or -1 if b is not a valid/complete WKB.
func wkbLength(b []byte) int {
	if len(b) < 5 {
		return -1
	}
	var order binary.ByteOrder
	switch b[0] {
	case 0:
		order = binary.BigEndian
	case 1:
		order = binary.LittleEndian
	default:
		return -1
	}
	return wkbElemLength(b, order)
}

func wkbElemLength(b []byte, order binary.ByteOrder) int {
	if len(b) < 5 {
		return -1
	}
	geomType := order.Uint32(b[1:5])
	body := 5
	switch geomType {
	case 1: // Point
		return body + 16
	case 2: // LineString
		if len(b) < body+4 {
			return -1
		}
		n := int(order.Uint32(b[body : body+4]))
		return body + 4 + n*16
	case 3: // Polygon
		if len(b) < body+4 {
			return -1
		}
		rings := int(order.Uint32(b[body : body+4]))
		pos := body + 4
		for i := 0; i < rings; i++ {
			if len(b) < pos+4 {
				return -1
			}
			points := int(order.Uint32(b[pos : pos+4]))
			pos += 4 + points*16
		}
		return pos
	case 4, 5, 6, 7: // MultiPoint / MultiLineString / MultiPolygon / GeometryCollection
		if len(b) < body+4 {
			return -1
		}
		n := int(order.Uint32(b[body : body+4]))
		pos := body + 4
		for i := 0; i < n; i++ {
			inner := wkbLength(b[pos:])
			if inner <= 0 || pos+inner > len(b) {
				return -1
			}
			pos += inner
		}
		return pos
	default:
		return -1
	}
}

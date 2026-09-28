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

import (
	"encoding/hex"
	"testing"
)

func mustHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestSplitGeometry(t *testing.T) {
	cases := []struct {
		name    string
		hex     string
		hasSRID bool
		srid    uint32
		wkb     string
	}{
		{
			name:    "srid0_point_with_prefix",
			hex:     "000000000101000000000000000000f03f0000000000000040",
			hasSRID: true,
			srid:    0,
			wkb:     "0101000000000000000000f03f0000000000000040",
		},
		{
			name:    "srid4326_point_with_prefix",
			hex:     "e6100000010100000000000000000010400000000000000840",
			hasSRID: true,
			srid:    4326,
			wkb:     "010100000000000000000010400000000000000840",
		},
		{
			name:    "raw_wkb_without_prefix",
			hex:     "0101000000000000000000f03f0000000000000040",
			hasSRID: false,
			srid:    0,
			wkb:     "0101000000000000000000f03f0000000000000040",
		},
		{
			name:    "srid0_linestring_with_prefix",
			hex:     "0000000001020000000300000000000000000000000000000000000000000000000000f03f000000000000f03f00000000000000400000000000000040",
			hasSRID: true,
			srid:    0,
			wkb:     "01020000000300000000000000000000000000000000000000000000000000f03f000000000000f03f00000000000000400000000000000040",
		},
	}
	for _, c := range cases {
		wkb, srid, hasSRID := splitGeometry(mustHex(t, c.hex))
		if hasSRID != c.hasSRID {
			t.Errorf("%s: hasSRID = %v, want %v", c.name, hasSRID, c.hasSRID)
		}
		if srid != c.srid {
			t.Errorf("%s: srid = %d, want %d", c.name, srid, c.srid)
		}
		if hex.EncodeToString(wkb) != c.wkb {
			t.Errorf("%s: wkb = %s, want %s", c.name, hex.EncodeToString(wkb), c.wkb)
		}
	}
}

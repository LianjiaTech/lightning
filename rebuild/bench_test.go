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
	"strings"
	"testing"

	"github.com/go-mysql-org/go-mysql/mysql"
	"github.com/go-mysql-org/go-mysql/replication"
)

// BenchmarkBuildValues measures SQL literal generation for a wide table, the
// shape that dominates real binlogs (many columns and long text values).
func BenchmarkBuildValues(b *testing.B) {
	ev := &replication.RowsEvent{
		Table: &replication.TableMapEvent{
			Schema: []byte("test"),
			Table:  []byte("wide"),
			ColumnType: []byte{
				mysql.MYSQL_TYPE_LONG,
				mysql.MYSQL_TYPE_LONGLONG,
				mysql.MYSQL_TYPE_VARCHAR,
				mysql.MYSQL_TYPE_VAR_STRING,
				mysql.MYSQL_TYPE_DATETIME2,
				mysql.MYSQL_TYPE_DOUBLE,
				mysql.MYSQL_TYPE_NEWDECIMAL,
				mysql.MYSQL_TYPE_BLOB,
				mysql.MYSQL_TYPE_JSON,
				mysql.MYSQL_TYPE_TINY,
			},
		},
	}
	rows := make([][]any, 50)
	for i := range rows {
		rows[i] = []any{
			int64(i),
			int64(1) << 40,
			strings.Repeat("varchar-value-", 4),
			[]byte(strings.Repeat("var-string-", 3)),
			"2024-01-02 03:04:05",
			float64(i) + 0.5,
			"12345.6789",
			[]byte("blob payload"),
			[]byte(`{"key":"value"}`),
			int64(1),
		}
	}
	ev.Rows = rows

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = BuildValues(ev)
	}
}

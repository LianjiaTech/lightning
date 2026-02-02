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
	"fmt"
	"strconv"
	"testing"

	"github.com/go-mysql-org/go-mysql/mysql"
	"github.com/go-mysql-org/go-mysql/replication"
)

func TestLastStatus(t *testing.T) {
	LastStatus()
}

func TestPrintBinlogStat(t *testing.T) {
	printBinlogStat()
}

func TestLoadLuaScript(t *testing.T) {
	LoadLuaScript()
}

func TestStrconvQuote(t *testing.T) {
	fmt.Println(strconv.Quote(`'"space `))
	fmt.Printf(`"%s"`, escape(`'"space `))
}

// TestBuildValuesDataTypes tests MySQL 8.0/8.4 data types support
func TestBuildValuesDataTypes(t *testing.T) {
	testCases := []struct {
		name        string
		event       *replication.RowsEvent
		expectedSQL []string
	}{
		{
			name: "ENUM type",
			event: &replication.RowsEvent{
				Table: &replication.TableMapEvent{
					Schema: []byte("test"),
					Table:  []byte("enum_test"),
					ColumnType: []byte{
						mysql.MYSQL_TYPE_ENUM,
					},
				},
				Rows: [][]interface{}{
					{1}, // 'red' in ENUM('red','green','blue')
					{2}, // 'green'
					{3}, // 'blue'
				},
			},
			expectedSQL: []string{"1", "2", "3"},
		},
		{
			name: "SET type",
			event: &replication.RowsEvent{
				Table: &replication.TableMapEvent{
					Schema: []byte("test"),
					Table:  []byte("set_test"),
					ColumnType: []byte{
						mysql.MYSQL_TYPE_SET,
					},
				},
				Rows: [][]interface{}{
					{1}, // 'bold' only
					{3}, // 'bold' + 'italic' = 1 + 2 = 3
					{7}, // 'bold' + 'italic' + 'underline' = 1 + 2 + 4 = 7
				},
			},
			expectedSQL: []string{"1", "3", "7"},
		},
		{
			name: "GEOMETRY type (POINT)",
			event: &replication.RowsEvent{
				Table: &replication.TableMapEvent{
					Schema: []byte("test"),
					Table:  []byte("geometry_test"),
					ColumnType: []byte{
						mysql.MYSQL_TYPE_GEOMETRY,
					},
				},
				Rows: [][]interface{}{
					{[]byte{0x01, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x24, 0x40, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x2e, 0x40}}, // POINT(10, 15) in WKB
				},
			},
			expectedSQL: []string{"ST_GeomFromWKB(X'010100000000000000000024400000000000002e40')"},
		},
		{
			name: "VECTOR type",
			event: &replication.RowsEvent{
				Table: &replication.TableMapEvent{
					Schema: []byte("test"),
					Table:  []byte("vector_test"),
					ColumnType: []byte{
						mysql.MYSQL_TYPE_VECTOR,
					},
				},
				Rows: [][]interface{}{
					{[]byte{0x00, 0x00, 0x40, 0x00, 0x00, 0x00, 0x40, 0x00, 0x00, 0x00, 0x40, 0x00}}, // VECTOR as binary float array
				},
			},
			expectedSQL: []string{"STRING_TO_VECTOR(X'000040000000400000004000')"},
		},
		{
			name: "BLOB types",
			event: &replication.RowsEvent{
				Table: &replication.TableMapEvent{
					Schema: []byte("test"),
					Table:  []byte("blob_test"),
					ColumnType: []byte{
						mysql.MYSQL_TYPE_BLOB,
						mysql.MYSQL_TYPE_TINY_BLOB,
						mysql.MYSQL_TYPE_MEDIUM_BLOB,
						mysql.MYSQL_TYPE_LONG_BLOB,
					},
				},
				Rows: [][]interface{}{
					{[]byte("test blob data"), []byte("tiny blob"), []byte("medium blob"), []byte("long blob")},
				},
			},
			expectedSQL: []string{
				"X'7465737420626c6f622064617461'",
				"X'74696e7920626c6f62'",
				"X'6d656469756d20626c6f62'",
				"X'6c6f6e6720626c6f62'",
			},
		},
		{
			name: "Mixed types (JSON, BIT, GEOMETRY)",
			event: &replication.RowsEvent{
				Table: &replication.TableMapEvent{
					Schema: []byte("test"),
					Table:  []byte("mixed_test"),
					ColumnType: []byte{
						mysql.MYSQL_TYPE_JSON,
						mysql.MYSQL_TYPE_BIT,
						mysql.MYSQL_TYPE_GEOMETRY,
					},
				},
				Rows: [][]interface{}{
					{[]byte(`{"key": "value"}`), int64(255), []byte{0x01, 0x01, 0x00, 0x00, 0x00}},
				},
			},
			expectedSQL: []string{
				"'{\"key\": \"value\"}'",
				"255",
				"ST_GeomFromWKB(X'0101000000')",
			},
		},
		{
			name: "NULL values",
			event: &replication.RowsEvent{
				Table: &replication.TableMapEvent{
					Schema: []byte("test"),
					Table:  []byte("null_test"),
					ColumnType: []byte{
						mysql.MYSQL_TYPE_ENUM,
						mysql.MYSQL_TYPE_GEOMETRY,
						mysql.MYSQL_TYPE_BLOB,
					},
				},
				Rows: [][]interface{}{
					{nil, nil, nil},
				},
			},
			expectedSQL: []string{"NULL", "NULL", "NULL"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			values := BuildValues(tc.event)

			for rowIdx, row := range values {
				for colIdx, col := range row {
					expectedIdx := rowIdx*len(row) + colIdx
					if expectedIdx >= len(tc.expectedSQL) {
						t.Fatalf("Unexpected output at row %d, col %d", rowIdx, colIdx)
					}
					if col != tc.expectedSQL[expectedIdx] {
						t.Errorf("Expected %s, got %s at row %d, col %d",
							tc.expectedSQL[expectedIdx], col, rowIdx, colIdx)
					}
				}
			}
		})
	}
}

// TestBuildValuesUnsignedInts tests unsigned integer max values
func TestBuildValuesUnsignedInts(t *testing.T) {
	// Note: This test requires schema to be set up with unsigned flags
	// For now, we test without schema (no unsigned flag)
	event := &replication.RowsEvent{
		Table: &replication.TableMapEvent{
			Schema: []byte("test"),
			Table:  []byte("uint_test"),
			ColumnType: []byte{
				mysql.MYSQL_TYPE_TINY,
				mysql.MYSQL_TYPE_SHORT,
				mysql.MYSQL_TYPE_INT24,
				mysql.MYSQL_TYPE_LONG,
				mysql.MYSQL_TYPE_LONGLONG,
			},
		},
		Rows: [][]interface{}{
			{-1, -1, -1, -1, -1}, // -1 in binlog represents max value for unsigned
		},
	}

	values := BuildValues(event)

	// Without schema (unsigned flag), -1 should remain as -1
	expected := []string{"-1", "-1", "-1", "-1", "-1"}

	for i, col := range values[0] {
		if col != expected[i] {
			t.Errorf("Expected %s, got %s for column %d", expected[i], col, i)
		}
	}
}

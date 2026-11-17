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
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/LianjiaTech/lightning/common"
	"github.com/pingcap/tidb/pkg/parser/ast"

	"github.com/BixData/gluabit32"
	"github.com/BixData/gluasocket"
	"github.com/go-mysql-org/go-mysql/mysql"
	"github.com/go-mysql-org/go-mysql/replication"
	"github.com/montanaflynn/stats"

	uuid "github.com/satori/go.uuid"
	lua "github.com/yuin/gopher-lua"
	"github.com/zhu327/gluadb"
	lfs "layeh.com/gopher-lfs"
)

// for -plugin stat
// TableStats
var TableStats map[string]map[string]int64

// RowsStats
var RowsStats map[string]map[string]int64

// QueryStats ...
var QueryStats map[string]int64

// TransactionStartPos ...
var TransactionStartPos float64

// TransactionStartTimeStamp ...
var TransactionStartTimeStamp float64

// MaxTransactionTime ...
var MaxTransactionTime float64

// MaxTransactionSize ...
var MaxTransactionSize float64

// MaxTransactionSizeStartPos ..
var MaxTransactionSizeStartPos float64

// MaxTransactionSizeStopPos = MaxTransactionSizeStartPos + Max(TransactionSizeStats)

// MaxTransactionTimeStartPos ...
var MaxTransactionTimeStartPos float64

// MaxTransactionTimeStopPos ...
var MaxTransactionTimeStopPos float64

// TransactionSizeStats ...
var TransactionSizeStats []float64

// TransactionTimeStats ...
var TransactionTimeStats []float64

type Stats struct {
	Table           map[string]map[string]int64  `json:"TableStats"`
	Rows            map[string]map[string]int64  `json:"RowsStats"`
	Query           map[string]int64             `json:"QueryStats"`
	Transaction     map[string]map[string]string `json:"TransactionStats"`
	TransactionSize []float64                    `json:"-"` // take from end_log_pos between begin and commit
	TransactionTime []float64                    `json:"-"` // take from timestamp between begin and commit
}

// BinlogStats ...
var BinlogStats Stats

// Lua ...
var Lua *lua.LState

// InsertValuesMerge INSERT values merge
var InsertValuesMerge []string

func init() {
	TableStats = make(map[string]map[string]int64)
	RowsStats = make(map[string]map[string]int64)
	Schemas = make(map[string]*ast.CreateTableStmt)
}

// RowEventTable returns the `schema`.`table` name of a row event. Row events
// of the same table (a table map plus all its following row events) repeat the
// lookup many times per event, so cache the formatted name per table id.
func RowEventTable(event *replication.BinlogEvent) string {
	if event == nil {
		return ""
	}
	switch event.Header.EventType {
	case replication.WRITE_ROWS_EVENTv0, replication.WRITE_ROWS_EVENTv1, replication.WRITE_ROWS_EVENTv2,
		replication.UPDATE_ROWS_EVENTv0, replication.UPDATE_ROWS_EVENTv1, replication.UPDATE_ROWS_EVENTv2,
		replication.PARTIAL_UPDATE_ROWS_EVENT,
		replication.DELETE_ROWS_EVENTv0, replication.DELETE_ROWS_EVENTv1, replication.DELETE_ROWS_EVENTv2:
		ev := event.Event.(*replication.RowsEvent)
		if ev.Table == nil {
			return ""
		}
		if cached, ok := rowEventTableCache[ev.TableID]; ok &&
			bytes.Equal(cached.schema, ev.Table.Schema) && bytes.Equal(cached.table, ev.Table.Table) {
			return cached.name
		}
		name := fmt.Sprintf("`%s`.`%s`", ev.Table.Schema, ev.Table.Table)
		rowEventTableCache[ev.TableID] = &rowEventTable{
			schema: append([]byte(nil), ev.Table.Schema...),
			table:  append([]byte(nil), ev.Table.Table...),
			name:   name,
		}
		return name
	}
	return ""
}

type rowEventTable struct {
	schema []byte
	table  []byte
	name   string
}

// rowEventTableCache maps a table id to its formatted name. Table ids can be
// reused after a table map rewrite, so the cached schema/table bytes are
// compared before reuse.
var rowEventTableCache = make(map[uint64]*rowEventTable)

// BuildValues build values list
func BuildValues(event *replication.RowsEvent) [][]string {
	table := fmt.Sprintf("`%s`.`%s`", event.Table.Schema, event.Table.Table)

	// The schema lookup and the unsigned flag used to be resolved for every
	// column of every row; resolve them once per event instead.
	var cols []*ast.ColumnDef
	if schema, ok := Schemas[table]; ok && schema != nil {
		cols = schema.Cols
	}
	columnTypes := event.Table.ColumnType

	var buf []byte
	values := make([][]string, 0, len(event.Rows))
	for _, row := range event.Rows {
		columns := make([]string, 0, len(columnTypes))
		for i, t := range columnTypes {
			v := row[i]
			if v == nil {
				columns = append(columns, "NULL")
				continue
			}
			unsigned := i < len(cols) && cols[i] != nil &&
				cols[i].Tp.GetFlag()&mysql.UNSIGNED_FLAG > 0
			buf = appendColumnValue(buf[:0], v, t, unsigned)
			columns = append(columns, string(buf))
		}
		values = append(values, columns)
	}
	return values
}

// appendColumnValue renders one binlog value as an SQL literal. Values are
// formatted into the caller's scratch buffer with strconv/hex instead of
// fmt.Sprintf to avoid reflection on every column of every row.
func appendColumnValue(buf []byte, v any, t byte, unsigned bool) []byte {
	switch t {
	case mysql.MYSQL_TYPE_DECIMAL, mysql.MYSQL_TYPE_NEWDECIMAL, mysql.MYSQL_TYPE_FLOAT, mysql.MYSQL_TYPE_DOUBLE, mysql.MYSQL_TYPE_NULL,
		mysql.MYSQL_TYPE_TIMESTAMP:
		return appendAnyValue(buf, v)
	// binlog use -1 for unsigned int max value
	case mysql.MYSQL_TYPE_TINY:
		if unsigned && isMinusOne(v) {
			return append(buf, "255"...)
		}
		return appendAnyValue(buf, v)
	case mysql.MYSQL_TYPE_SHORT:
		if unsigned && isMinusOne(v) {
			return append(buf, "65535"...)
		}
		return appendAnyValue(buf, v)
	case mysql.MYSQL_TYPE_INT24:
		if unsigned && isMinusOne(v) {
			return append(buf, "16777215"...)
		}
		return appendAnyValue(buf, v)
	case mysql.MYSQL_TYPE_LONG:
		if unsigned && isMinusOne(v) {
			return append(buf, "4294967295"...)
		}
		return appendAnyValue(buf, v)
	case mysql.MYSQL_TYPE_LONGLONG:
		if unsigned && isMinusOne(v) {
			return append(buf, "18446744073709551615"...)
		}
		return appendAnyValue(buf, v)
	case mysql.MYSQL_TYPE_DATE, mysql.MYSQL_TYPE_TIME, mysql.MYSQL_TYPE_DATETIME, mysql.MYSQL_TYPE_YEAR,
		mysql.MYSQL_TYPE_NEWDATE, mysql.MYSQL_TYPE_TIMESTAMP2, mysql.MYSQL_TYPE_DATETIME2, mysql.MYSQL_TYPE_TIME2:
		buf = append(buf, '\'')
		buf = appendAnyValue(buf, v)
		return append(buf, '\'')
	case mysql.MYSQL_TYPE_VARCHAR, mysql.MYSQL_TYPE_VAR_STRING, mysql.MYSQL_TYPE_STRING:
		switch x := v.(type) {
		case string:
			if common.Config.Global.HexString {
				return appendHexLiteral(buf, []byte(x))
			}
			// strconv.Quote will escape unicode \u0100
			// escape function maybe not correct with multi byte charset
			buf = append(buf, '"')
			buf = appendEscapedString(buf, x)
			return append(buf, '"')
		case []byte:
			if common.Config.Global.HexString {
				return appendHexLiteral(buf, x)
			}
			buf = append(buf, '"')
			buf = appendEscapedBytes(buf, x)
			return append(buf, '"')
		case int, int64, int32, int16, int8, uint64, uint32, uint16, uint8:
			// SET ENUM
			return appendAnyValue(buf, x)
		default:
			return appendSingleQuoted(buf, v)
		}

	// MySQL 8.0/8.4: ENUM and SET types - stored as integers in binlog
	case mysql.MYSQL_TYPE_ENUM, mysql.MYSQL_TYPE_SET:
		return appendAnyValue(buf, v)

	case mysql.MYSQL_TYPE_JSON:
		return appendSingleQuoted(buf, v)
	case mysql.MYSQL_TYPE_BIT:
		switch x := v.(type) {
		case int64, int, uint64:
			return appendAnyValue(buf, x)
		default:
			return appendSingleQuoted(buf, v)
		}

	// MySQL 8.0/8.4: GEOMETRY and Spatial types - stored as binary: 4-byte
	// little-endian SRID (8.0+) followed by WKB
	// Supports: GEOMETRY, POINT, LINESTRING, POLYGON, MULTIPOINT,
	//           MULTILINESTRING, MULTIPOLYGON, GEOMETRYCOLLECTION
	case mysql.MYSQL_TYPE_GEOMETRY:
		var b []byte
		switch x := v.(type) {
		case []byte:
			b = x
		default:
			b = []byte(fmt.Sprint(x))
		}
		wkb, srid, hasSRID := splitGeometry(b)
		buf = append(buf, "ST_GeomFromWKB(X'"...)
		buf = hex.AppendEncode(buf, wkb)
		if hasSRID {
			buf = append(buf, '\'', ',', ' ')
			buf = strconv.AppendInt(buf, int64(srid), 10)
			return append(buf, ')')
		}
		return append(buf, '\'', ')')

	// MySQL 9.0: VECTOR type - little-endian float32 array without any prefix;
	// a binary literal is accepted directly when inserting into a VECTOR column
	case mysql.MYSQL_TYPE_VECTOR:
		switch x := v.(type) {
		case []byte:
			return appendHexLiteral(buf, x)
		default:
			return appendHexLiteral(buf, []byte(fmt.Sprint(x)))
		}

	// MySQL 8.0/8.4: BLOB types - stored as binary data
	case mysql.MYSQL_TYPE_TINY_BLOB, mysql.MYSQL_TYPE_BLOB, mysql.MYSQL_TYPE_MEDIUM_BLOB, mysql.MYSQL_TYPE_LONG_BLOB:
		switch x := v.(type) {
		case []byte:
			return appendHexLiteral(buf, x)
		default:
			return appendHexLiteral(buf, []byte(fmt.Sprint(x)))
		}

	default:
		switch x := v.(type) {
		case []byte:
			return appendHexLiteral(buf, x)
		default:
			return appendSingleQuoted(buf, v)
		}
	}
}

// appendAnyValue appends the decimal/text form of a decoded binlog value.
func appendAnyValue(buf []byte, v any) []byte {
	switch x := v.(type) {
	case string:
		return append(buf, x...)
	case []byte:
		return append(buf, x...)
	case int:
		return strconv.AppendInt(buf, int64(x), 10)
	case int8:
		return strconv.AppendInt(buf, int64(x), 10)
	case int16:
		return strconv.AppendInt(buf, int64(x), 10)
	case int32:
		return strconv.AppendInt(buf, int64(x), 10)
	case int64:
		return strconv.AppendInt(buf, x, 10)
	case uint:
		return strconv.AppendUint(buf, uint64(x), 10)
	case uint8:
		return strconv.AppendUint(buf, uint64(x), 10)
	case uint16:
		return strconv.AppendUint(buf, uint64(x), 10)
	case uint32:
		return strconv.AppendUint(buf, uint64(x), 10)
	case uint64:
		return strconv.AppendUint(buf, x, 10)
	case float32:
		return strconv.AppendFloat(buf, float64(x), 'g', -1, 32)
	case float64:
		return strconv.AppendFloat(buf, x, 'g', -1, 64)
	default:
		return append(buf, fmt.Sprint(x)...)
	}
}

func appendSingleQuoted(buf []byte, v any) []byte {
	buf = append(buf, '\'')
	buf = appendAnyValue(buf, v)
	return append(buf, '\'')
}

func appendHexLiteral(buf []byte, b []byte) []byte {
	buf = append(buf, 'X', '\'')
	buf = hex.AppendEncode(buf, b)
	return append(buf, '\'')
}

// isMinusOne reports whether a decoded integer is binlog's -1 placeholder for
// an unsigned maximum value.
func isMinusOne(v any) bool {
	switch x := v.(type) {
	case int:
		return x == -1
	case int8:
		return x == -1
	case int16:
		return x == -1
	case int32:
		return x == -1
	case int64:
		return x == -1
	default:
		return false
	}
}

// GTIDRebuild ...
func GTIDRebuild(event *replication.GTIDEvent) {
	serverID, _ := uuid.FromBytes(event.SID)
	tag := ""
	if s := event.Tag.String(); s != "" {
		tag = fmt.Sprintf(", Tag: %s", s)
	}
	common.Verbose("-- [DEBUG] GTID_NEXT: %s:%d%s, LastCommitted: %d, SequenceNumber: %d, CommitFlag: %d\n", serverID, event.GNO, tag, event.LastCommitted, event.SequenceNumber, event.CommitFlag)
}

// EventHeaderRebuild ...
func EventHeaderRebuild(event *replication.BinlogEvent) {
	header := event.Header
	common.Verbose("-- [DEBUG] EventType: %s, ServerID: %d, Timestamp: %d, LogPos: %d, EventSize: %d, Flags: %d\n",
		header.EventType.String(), header.ServerID, header.Timestamp, header.LogPos, header.EventSize, header.Flags)

	if common.Config.Rebuild.ForeachTime {
		common.Config.Rebuild.CurrentEventTime = fmt.Sprint(time.Unix(int64(header.Timestamp), 0).Format("2006-01-02 15:04:05"))
	}
}

// LastStatus ...
func LastStatus() {
	switch common.Config.Rebuild.Plugin {
	case "stat":
		printBinlogStat()
	}
	defer common.FlushOut()
	if Lua != nil {
		if err := Lua.CallByParam(lua.P{
			Fn:      Lua.GetGlobal("Finalizer"),
			NRet:    1,
			Protect: true,
		}); err != nil {
			common.Log.Error(err.Error())
			return
		}
		defer Lua.Close()
	}
}

// printBinlogStat ...
func printBinlogStat() {
	// TransactionTimeStats
	medianTime, _ := stats.Median(TransactionTimeStats)
	maxTime, _ := stats.Max(TransactionTimeStats)
	meanTime, _ := stats.Mean(TransactionTimeStats)
	p99Time, _ := stats.Percentile(TransactionTimeStats, 99)
	p95Time, _ := stats.Percentile(TransactionTimeStats, 95)
	// TransactionSizeStats
	medianSize, _ := stats.Median(TransactionSizeStats)
	maxSize, _ := stats.Max(TransactionSizeStats)
	meanSize, _ := stats.Mean(TransactionSizeStats)
	p99Size, _ := stats.Percentile(TransactionSizeStats, 99)
	p95Size, _ := stats.Percentile(TransactionSizeStats, 95)

	BinlogStats = Stats{
		Table: TableStats,
		Rows:  RowsStats,
		Query: QueryStats,
		Transaction: map[string]map[string]string{
			"TimeSeconds": {
				"MaxTransactionPos": fmt.Sprintf("-start-position %d -stop-position %d", int64(MaxTransactionTimeStartPos), int64(MaxTransactionTimeStopPos)),
				"Median":            fmt.Sprintf("%0.2f", medianTime),
				"Max":               fmt.Sprintf("%0.2f", maxTime),
				"Mean":              fmt.Sprintf("%0.2f", meanTime),
				"P99":               fmt.Sprintf("%0.2f", p99Time),
				"P95":               fmt.Sprintf("%0.2f", p95Time),
			},
			"SizeBytes": {
				"MaxTransactionPos": fmt.Sprintf("-start-position %d -stop-position %d", int64(MaxTransactionSizeStartPos), int64(MaxTransactionSizeStartPos+MaxTransactionSize)),
				"Median":            fmt.Sprintf("%0.1f", medianSize),
				"Max":               fmt.Sprintf("%0.1f", maxSize),
				"Mean":              fmt.Sprintf("%0.1f", meanSize),
				"P99":               fmt.Sprintf("%0.1f", p99Size),
				"P95":               fmt.Sprintf("%0.1f", p95Size),
			},
		},
	}

	buf, err := json.MarshalIndent(BinlogStats, "", "  ")
	if err != nil {
		common.Outln(err)
	}
	common.Outln(string(buf))
}

// LuaStringList ...
func LuaStringList(name string, values []string) {
	t := Lua.NewTable()
	for k, v := range values {
		Lua.SetTable(t, lua.LNumber(k+1), lua.LString(v))
	}
	Lua.SetGlobal(name, t)
}

// LuaMapStringList ...
func LuaMapStringList(name string, values map[string][]string) {
	t := Lua.NewTable()
	for k, cols := range values {
		l := Lua.NewTable()
		for i, col := range cols {
			Lua.SetTable(l, lua.LNumber(i+1), lua.LString(col))
		}
		Lua.SetTable(t, lua.LString(k), l)
	}
	Lua.SetGlobal(name, t)
}

// LoadLuaScript ...
func LoadLuaScript() {
	if common.Config.Rebuild.LuaScript == "" || common.Config.Rebuild.Plugin != "lua" {
		return
	}
	Lua = lua.NewState()
	gluasocket.Preload(Lua)
	gluabit32.Preload(Lua)
	gluadb.Preload(Lua) // lua package require "mysql", "redis"
	lfs.Preload(Lua)    // lfs.currentdir() for package loading

	if err := Lua.DoFile(common.Config.Rebuild.LuaScript); err != nil {
		common.Log.Error(err.Error())
		return
	}

	LuaMapStringList("GoPrimaryKeys", PrimaryKeys)
	LuaMapStringList("GoColumns", Columns)

	if err := Lua.CallByParam(lua.P{
		Fn:      Lua.GetGlobal("Init"),
		NRet:    1,
		Protect: true,
	}); err != nil {
		common.Log.Error(err.Error())
		return
	}
}

func escape(sql string) string {
	dest := make([]byte, 0, 2*len(sql))
	return string(appendEscapedString(dest, sql))
}

// appendEscapedString appends MySQL-escaped text to dst.
func appendEscapedString(dst []byte, sql string) []byte {
	var esc byte
	for i := 0; i < len(sql); i++ {
		c := sql[i]
		esc = 0
		switch c {
		case 0: /* Must be escaped for 'mysql' */
			esc = '0'
		case '\n': /* Must be escaped for logs */
			esc = 'n'
		case '\r':
			esc = 'r'
		case '\\':
			esc = '\\'
		case '\'':
			esc = '\''
		case '"': /* Better safe than sorry */
			esc = '"'
		case '\032': /* This gives problems on Win32 */
			esc = 'Z'
		}
		if esc != 0 {
			dst = append(dst, '\\', esc)
		} else {
			dst = append(dst, c)
		}
	}
	return dst
}

// appendEscapedBytes appends MySQL-escaped bytes to dst.
func appendEscapedBytes(dst []byte, sql []byte) []byte {
	var esc byte
	for _, c := range sql {
		esc = 0
		switch c {
		case 0:
			esc = '0'
		case '\n':
			esc = 'n'
		case '\r':
			esc = 'r'
		case '\\':
			esc = '\\'
		case '\'':
			esc = '\''
		case '"':
			esc = '"'
		case '\032':
			esc = 'Z'
		}
		if esc != 0 {
			dst = append(dst, '\\', esc)
		} else {
			dst = append(dst, c)
		}
	}
	return dst
}

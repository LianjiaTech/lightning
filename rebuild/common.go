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
	"encoding/json"
	"fmt"
	"time"

	"github.com/LianjiaTech/lightning/common"
	"github.com/pingcap/parser/ast"

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

// RowEventTable ...
func RowEventTable(event *replication.BinlogEvent) string {
	if event == nil {
		return ""
	}
	switch event.Header.EventType {
	case replication.WRITE_ROWS_EVENTv0, replication.WRITE_ROWS_EVENTv1, replication.WRITE_ROWS_EVENTv2,
		replication.UPDATE_ROWS_EVENTv0, replication.UPDATE_ROWS_EVENTv1, replication.UPDATE_ROWS_EVENTv2,
		replication.DELETE_ROWS_EVENTv0, replication.DELETE_ROWS_EVENTv1, replication.DELETE_ROWS_EVENTv2:
		return fmt.Sprintf("`%s`.`%s`",
			string(event.Event.(*replication.RowsEvent).Table.Schema),
			string(event.Event.(*replication.RowsEvent).Table.Table))
	}
	return ""
}

// BuildValues build values list
func BuildValues(event *replication.RowsEvent) [][]string {
	table := fmt.Sprintf("`%s`.`%s`", string(event.Table.Schema), event.Table.Table)
	var values [][]string
	for _, row := range event.Rows {
		var columns []string
		for i, t := range event.Table.ColumnType {
			if row[i] == nil {
				columns = append(columns, "NULL")
				continue
			}
			var unsigned bool
			if ok := Schemas[table]; ok != nil {
				if (Schemas[table].Cols[i].Tp.Flag & mysql.UNSIGNED_FLAG) > 0 {
					unsigned = true
				}
			}
			switch t {
			case mysql.MYSQL_TYPE_DECIMAL, mysql.MYSQL_TYPE_NEWDECIMAL, mysql.MYSQL_TYPE_FLOAT, mysql.MYSQL_TYPE_DOUBLE, mysql.MYSQL_TYPE_NULL,
				mysql.MYSQL_TYPE_TIMESTAMP:
				columns = append(columns, fmt.Sprint(row[i]))
			// binlog use -1 for unsigned int max value
			case mysql.MYSQL_TYPE_TINY:
				if unsigned && fmt.Sprint(row[i]) == "-1" {
					columns = append(columns, "255")
				} else {
					columns = append(columns, fmt.Sprint(row[i]))
				}
			case mysql.MYSQL_TYPE_SHORT:
				if unsigned && fmt.Sprint(row[i]) == "-1" {
					columns = append(columns, "65535")
				} else {
					columns = append(columns, fmt.Sprint(row[i]))
				}
			case mysql.MYSQL_TYPE_INT24:
				if unsigned && fmt.Sprint(row[i]) == "-1" {
					columns = append(columns, "16777215")
				} else {
					columns = append(columns, fmt.Sprint(row[i]))
				}
			case mysql.MYSQL_TYPE_LONG:
				if unsigned && fmt.Sprint(row[i]) == "-1" {
					columns = append(columns, "4294967295")
				} else {
					columns = append(columns, fmt.Sprint(row[i]))
				}
			case mysql.MYSQL_TYPE_LONGLONG:
				if unsigned && fmt.Sprint(row[i]) == "-1" {
					columns = append(columns, "18446744073709551615")
				} else {
					columns = append(columns, fmt.Sprint(row[i]))
				}
			case mysql.MYSQL_TYPE_DATE, mysql.MYSQL_TYPE_TIME, mysql.MYSQL_TYPE_DATETIME, mysql.MYSQL_TYPE_YEAR,
				mysql.MYSQL_TYPE_NEWDATE, mysql.MYSQL_TYPE_TIMESTAMP2, mysql.MYSQL_TYPE_DATETIME2, mysql.MYSQL_TYPE_TIME2:
				columns = append(columns, fmt.Sprint("'", row[i], "'"))
			case mysql.MYSQL_TYPE_VARCHAR, mysql.MYSQL_TYPE_VAR_STRING, mysql.MYSQL_TYPE_STRING:
				switch row[i].(type) {
				case string:
					if common.Config.Global.HexString {
						columns = append(columns, fmt.Sprintf(`X'%s'`, hex.EncodeToString(row[i].([]byte))))
					} else {
						// strconv.Quote will escape unicode \u0100
						// escape function maybe not correct with multi byte charset
						// columns = append(columns, strconv.Quote(row[i].(string)))
						columns = append(columns, fmt.Sprintf(`"%s"`, escape(row[i].(string))))
					}
				case int, int64, int32, int16, int8, uint64, uint32, uint16, uint8:
					// SET ENUM
					columns = append(columns, fmt.Sprint(row[i]))
				default:
					columns = append(columns, fmt.Sprintf(`'%s'`, fmt.Sprint(row[i])))
				}

			case mysql.MYSQL_TYPE_JSON:
				columns = append(columns, fmt.Sprintf(`'%s'`, row[i].([]byte)))
			case mysql.MYSQL_TYPE_BIT:
				columns = append(columns, fmt.Sprintf(`%d`, row[i].(int64)))
			default:
				// mysql.MYSQL_TYPE_TINY_BLOB, mysql.MYSQL_TYPE_BLOB, mysql.MYSQL_TYPE_MEDIUM_BLOB, mysql.MYSQL_TYPE_LONG_BLOB
				// mysql.MYSQL_TYPE_GEOMETRY
				columns = append(columns, fmt.Sprintf(`X'%s'`, hex.EncodeToString(row[i].([]byte))))
			}
		}
		values = append(values, columns)
	}
	return values
}

// GTIDRebuild ...
func GTIDRebuild(event *replication.GTIDEvent) {
	serverID, _ := uuid.FromBytes(event.SID)
	common.Verbose("-- [DEBUG] GTID_NEXT: %s:%d, LastCommitted: %d, SequenceNumber: %d, CommitFlag: %d\n", serverID, event.GNO, event.LastCommitted, event.SequenceNumber, event.CommitFlag)
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
		fmt.Println(err)
	}
	fmt.Println(string(buf))
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
	var escape byte
	for i := 0; i < len(sql); i++ {
		c := sql[i]

		escape = 0

		switch c {
		case 0: /* Must be escaped for 'mysql' */
			escape = '0'
			break
		case '\n': /* Must be escaped for logs */
			escape = 'n'
			break
		case '\r':
			escape = 'r'
			break
		case '\\':
			escape = '\\'
			break
		case '\'':
			escape = '\''
			break
		case '"': /* Better safe than sorry */
			escape = '"'
			break
		case '\032': /* This gives problems on Win32 */
			escape = 'Z'
		}

		if escape != 0 {
			dest = append(dest, '\\', escape)
		} else {
			dest = append(dest, c)
		}
	}

	return string(dest)
}

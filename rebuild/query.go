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
	"strings"

	"github.com/LianjiaTech/lightning/common"
	"github.com/siddontang/go-mysql/replication"
	lua "github.com/yuin/gopher-lua"

	"github.com/pingcap/parser"
	"github.com/pingcap/parser/ast"
	"github.com/pingcap/parser/mysql"

	// pingcap/parser
	_ "github.com/pingcap/tidb/types/parser_driver"
)

// QueryRebuild rebuild sql, need pingcap/parser
func QueryRebuild(queryEvent *replication.BinlogEvent) string {
	switch queryEvent.Header.EventType {
	case replication.QUERY_EVENT:
	default:
		common.Log.Error("QueryRebuild get wrong event: %s", queryEvent.Header.EventType.String())
		return ""
	}

	event := queryEvent.Event.(*replication.QueryEvent)

	common.Verbose("-- [DEBUG] ThreadID: %d, Schema: %s, ErrorCode: %d, ExecutionTime: %d, GSet: %v\n",
		event.SlaveProxyID, event.Schema, event.ErrorCode, event.ExecutionTime, event.GSet)

	sql := string(event.Query)
	switch common.Config.Rebuild.Plugin {
	case "sql":
		// fmt.Printf("SET TIMESTAMP=%d;\n", queryEvent.Header.Timestamp)
		QueryFormat(sql)
	case "flashback":
		QueryRollback(sql)
	case "stat":
		if sql == "BEGIN" {
			TransactionStartPos = float64(queryEvent.Header.LogPos)
			TransactionStartTimeStamp = float64(queryEvent.Header.Timestamp)
		}
		QueryStat(sql)
	case "lua":
		QueryLua(sql)
	default:
	}

	// stat transaction time, exec_time on slave it's replication lag time.
	// https://dev.mysql.com/doc/refman/5.6/en/mysqlbinlog.html
	transactionTime := float64(event.ExecutionTime)
	if transactionTime > MaxTransactionTime {
		MaxTransactionTime = transactionTime
		MaxTransactionTimeStartPos = TransactionStartPos
	}
	TransactionTimeStats = append(TransactionTimeStats, transactionTime)

	return ""
}

// RowsQueryRebuild ...
func RowsQueryRebuild(rowsQueryEvent *replication.BinlogEvent) string {
	switch rowsQueryEvent.Header.EventType {
	case replication.ROWS_QUERY_EVENT:
	default:
		common.Log.Error("RowsQueryRebuild get wrong event: %s", rowsQueryEvent.Header.EventType.String())
		return ""
	}

	event := rowsQueryEvent.Event.(*replication.RowsQueryEvent)
	common.VerboseVerbose("-- [DEBUG] RowsQuery Event, Query: %s\n", string(event.Query))
	return ""
}

// XidRebuild ...
func XidRebuild(event *replication.BinlogEvent) string {
	// stat transaction size
	transactionSize := float64(event.Header.LogPos) - TransactionStartPos
	if transactionSize > MaxTransactionSize {
		MaxTransactionSizeStartPos = TransactionStartPos
		MaxTransactionSize = transactionSize
	}
	TransactionSizeStats = append(TransactionSizeStats, transactionSize)

	if MaxTransactionTimeStartPos == TransactionStartPos {
		MaxTransactionTimeStopPos = float64(event.Header.LogPos)
	}

	common.Verbose("-- [DEBUG] XID_EVENT TransactionSizeBytes: %s, Xid: %d, GSet: %v\n",
		fmt.Sprintf("%0.0f", transactionSize), event.Event.(*replication.XIDEvent).XID, event.Event.(*replication.XIDEvent).GSet)
	return ""
}

// TiParse TiDB 语法解析
func TiParse(sql, charset, collation string) ([]ast.StmtNode, error) {
	p := parser.New()
	stmt, _, err := p.Parse(sql, charset, collation)
	return stmt, err
}

// QueryFormat ...
func QueryFormat(sql string) {
	if strings.HasPrefix(sql, "BEGIN") {
		common.Verbose("-- [DEBUG] BEGIN;")
		return
	}

	if strings.HasSuffix(sql, ";") {
		fmt.Println(sql)
	} else {
		fmt.Println(sql, ";")
	}
}

func QueryRollback(sql string) {
	stmts, err := TiParse(sql, common.Config.Global.Charset, mysql.Charsets[common.Config.Global.Charset])
	if err == nil {
		for _, stmt := range stmts {
			switch node := stmt.(type) {
			case *ast.CreateTableStmt:
				CreateTableRollback(node)
			case *ast.CreateDatabaseStmt:
				CreateDatabaseRollback(node)
			case *ast.CreateIndexStmt:
				CreateIndexRollback(node)
			case *ast.CreateViewStmt:
				CreateViewRollback(node)
			// case *ast.AlterTableStmt:
			// TODO: ALTER TABLE tb ADD col int;
			case *ast.BeginStmt:
				common.Verbose("-- [DEBUG] BEGIN;")
			default:
				common.VerboseVerbose("-- [DEBUG] can't rollback: %s;", sql)
			}
		}
	} else {
		common.Log.Error(err.Error())
	}
}

// CreateTableRollback ...
func CreateTableRollback(stmt *ast.CreateTableStmt) {
	if stmt.Table.Schema.String() == "" {
		fmt.Printf("DROP TABLE IF EXISTS `%s`;\n", stmt.Table.Name)
	} else {
		fmt.Printf("DROP TABLE IF EXISTS `%s`.`%s`;\n", stmt.Table.Schema, stmt.Table.Name)
	}
}

// CreateDatabaseRollback ...
func CreateDatabaseRollback(stmt *ast.CreateDatabaseStmt) {
	fmt.Printf("DROP DATABASE IF EXISTS `%s`;\n", stmt.Name)
}

// CreateIndexRollback ...
func CreateIndexRollback(stmt *ast.CreateIndexStmt) {
	if stmt.Table.Schema.String() == "" {
		fmt.Printf("DROP INDEX `%s` ON `%s`;\n", stmt.IndexName, stmt.Table.Name)
	} else {
		fmt.Printf("DROP INDEX `%s` ON `%s`.`%s`;\n", stmt.IndexName, stmt.Table.Schema, stmt.Table.Name)
	}
}

// CreateViewRollback ...
func CreateViewRollback(stmt *ast.CreateViewStmt) {
	if stmt.ViewName.Schema.String() == "" {
		fmt.Printf("DROP VIEW IF EXISTS `%s`;\n", stmt.ViewName.Name)
	} else {
		fmt.Printf("DROP VIEW IF EXISTS `%s`.`%s`;\n", stmt.ViewName.Schema, stmt.ViewName.Name)
	}
}

// QueryStat ...
func QueryStat(sql string) {
	// TODO: statement base table stat
	// stmt, err := TiParse(sql, common.Config.Global.Charset, mysql.Charsets[common.Config.Global.Charset])
	t := strings.ToLower(strings.Fields(sql)[0])
	if QueryStats != nil {
		QueryStats[t]++
	} else {
		QueryStats = map[string]int64{t: 1}
	}
}

// QueryLua ...
func QueryLua(sql string) {
	if common.Config.Rebuild.LuaScript == "" || sql == "" {
		return
	}

	if err := Lua.CallByParam(lua.P{
		Fn:      Lua.GetGlobal("QueryRewrite"),
		NRet:    1,
		Protect: true,
	}, lua.LString(sql)); err != nil {
		common.Log.Error(err.Error())
		return
	}
}

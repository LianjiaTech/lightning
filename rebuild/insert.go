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
)

// InsertRebuild ...
func InsertRebuild(event *replication.BinlogEvent) string {
	switch common.Config.Rebuild.Plugin {
	case "sql":
		InsertQuery(event)
	case "flashback":
		InsertRollbackQuery(event)
	case "stat":
		InsertStat(event)
	case "lua":
		InsertLua(event)
	default:
	}
	return ""
}

// InsertQuery ...
func InsertQuery(event *replication.BinlogEvent) {
	table := RowEventTable(event)
	ev := event.Event.(*replication.RowsEvent)
	values := BuildValues(ev)

	var insertPrefix string
	if common.Config.Rebuild.Replace {
		insertPrefix = "REPLACE INTO"
	} else {
		insertPrefix = "INSERT INTO"
	}

	colStr := ""
	for row, v := range values {
		valStr := ""
		if common.Config.Rebuild.CompleteInsert {
			if ok := Columns[table]; ok != nil {
				if len(common.Config.Rebuild.IgnoreColumns) > 0 {
					var truncValues, truncColumns []string
					for i, col := range Columns[table] {
						ignore := false
						for _, c := range common.Config.Rebuild.IgnoreColumns {
							if c == strings.Trim(col, "`") {
								ignore = true
							}
						}
						if !ignore {
							truncColumns = append(truncColumns, col)
							truncValues = append(truncValues, v[i])
						}
					}
					colStr = fmt.Sprintf("(%s)", strings.Join(truncColumns, ", "))
					valStr = strings.Join(truncValues, ", ")
				} else {
					colStr = fmt.Sprintf("(%s)", strings.Join(Columns[table], ", "))
					valStr = strings.Join(v, ", ")
				}
			} else {
				valStr = strings.Join(v, ", ")
			}
		} else {
			valStr = strings.Join(v, ", ")
		}

		if common.Config.Rebuild.ExtendedInsertCount > 1 {
			InsertValuesMerge = append(InsertValuesMerge, fmt.Sprintf("(%s)", valStr))
		} else {
			fmt.Printf("%s %s %s VALUES (%s);\n", insertPrefix, table, colStr, valStr)
		}

		// INSERT VALUES merge
		if row != 0 && common.Config.Rebuild.ExtendedInsertCount > 1 &&
			(row+1)%common.Config.Rebuild.ExtendedInsertCount == 0 {
			fmt.Printf("%s %s %s VALUES %s;\n", insertPrefix, table, colStr, strings.Join(InsertValuesMerge, ", "))
			InsertValuesMerge = []string{}
		}
	}
	if len(InsertValuesMerge) > 0 {
		fmt.Printf("%s %s %s VALUES %s;\n", insertPrefix, table, colStr, strings.Join(InsertValuesMerge, ", "))
		InsertValuesMerge = []string{}
	}
}

// InsertRollbackQuery ...
func InsertRollbackQuery(event *replication.BinlogEvent) {
	table := RowEventTable(event)
	ev := event.Event.(*replication.RowsEvent)
	values := BuildValues(ev)

	if ok := PrimaryKeys[table]; ok != nil {
		for _, value := range values {
			var where []string
			for _, col := range PrimaryKeys[table] {
				for i, c := range Columns[table] {
					if c == col {
						if value[i] == "NULL" {
							where = append(where, fmt.Sprintf("%s IS NULL", col))
						} else {
							where = append(where, fmt.Sprintf("%s = %s", col, value[i]))
						}
					}
				}
			}
			fmt.Printf("DELETE FROM %s WHERE %s LIMIT 1;\n", table, strings.Join(where, " AND "))
		}
	} else {
		for _, value := range values {
			var where []string
			for i, v := range value {
				col := fmt.Sprintf("@%d", i)
				if v == "NULL" {
					where = append(where, fmt.Sprintf("%s IS NULL", col))
				} else {
					where = append(where, fmt.Sprintf("%s = %s", col, v))
				}
			}
			fmt.Printf("-- DELETE FROM %s WHERE %s LIMIT 1;\n", table, strings.Join(where, " AND "))
		}
	}
}

// InsertStat ...
func InsertStat(event *replication.BinlogEvent) {
	table := RowEventTable(event)
	if TableStats[table] != nil {
		TableStats[table]["insert"]++
	} else {
		TableStats[table] = map[string]int64{"insert": 1}
	}
}

// InsertLua ...
func InsertLua(event *replication.BinlogEvent) {
	if common.Config.Rebuild.LuaScript == "" || event == nil {
		return
	}

	table := RowEventTable(event)
	ev := event.Event.(*replication.RowsEvent)
	values := BuildValues(ev)

	// lua function
	f := lua.P{
		Fn:      Lua.GetGlobal("InsertRewrite"),
		NRet:    0,
		Protect: true,
	}
	// lua value
	v := lua.LString(table)
	for _, value := range values {
		LuaStringList("GoValues", value)
		if err := Lua.CallByParam(f, v); err != nil {
			common.Log.Error(err.Error())
			return
		}
	}
}

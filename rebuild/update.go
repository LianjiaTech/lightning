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

	"github.com/go-mysql-org/go-mysql/replication"
	lua "github.com/yuin/gopher-lua"
)

// UpdateRebuild ...
func UpdateRebuild(event *replication.BinlogEvent) string {
	switch common.Config.Rebuild.Plugin {
	case "sql":
		UpdateQuery(event)
	case "flashback":
		UpdateRollbackQuery(event)
	case "stat":
		UpdateStat(event)
	case "lua":
		UpdateLua(event)
	default:
	}
	return ""
}

// UpdateQuery ...
func UpdateQuery(event *replication.BinlogEvent) {
	var table string
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("-- Table: %s, Error: %s\n", table, strings.Split(fmt.Sprint(r), "\n")[0])
		}
	}()
	table = RowEventTable(event)
	ev := event.Event.(*replication.RowsEvent)
	values := BuildValues(ev)

	common.Verbose("-- [DEBUG] event: update, table: %s, rows: %d\n", table, len(values))

	if common.Config.Rebuild.Replace {
		var insertValues [][]string
		for odd, value := range values {
			if odd%2 == 1 {
				insertValues = append(insertValues, value)
			}
		}
		insertQuery(table, insertValues)
	} else {
		updateQuery(table, values)
	}
}

func updateQuery(table string, values [][]string) {
	var where []string
	var set []string

	var updatePrefix = "UPDATE"
	if common.Config.Rebuild.ForeachTime && common.Config.Rebuild.CurrentEventTime != "" {
		updatePrefix = fmt.Sprintf(`/* %s */%s`, common.Config.Rebuild.CurrentEventTime, updatePrefix)
	}

	// for common.Config.Rebuild.WithoutDBName
	shortTableName := onlyTable(table)

	if ok := PrimaryKeys[table]; ok != nil {
		// 0 是 where 条件， 1 是 set 值
		for odd, value := range values {
			if odd%2 == 0 {
				where = []string{}
				set = []string{}
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
			} else {
				if len(common.Config.Rebuild.IgnoreColumns) > 0 {
					for i, col := range Columns[table] {
						ignore := false
						for _, c := range common.Config.Rebuild.IgnoreColumns {
							if c == strings.Trim(col, "`") {
								ignore = true
							}
						}
						if !ignore {
							set = append(set, fmt.Sprintf("%s = %s", col, value[i]))
						}
					}
				} else {
					for i, c := range Columns[table] {
						set = append(set, fmt.Sprintf("%s = %s", c, value[i]))
					}
				}

				if common.Config.Rebuild.WithoutDBName {
					fmt.Printf("%s %s SET %s WHERE %s LIMIT 1;\n", updatePrefix, shortTableName, strings.Join(set, ", "), strings.Join(where, " AND "))
				} else {
					fmt.Printf("%s %s SET %s WHERE %s LIMIT 1;\n", updatePrefix, table, strings.Join(set, ", "), strings.Join(where, " AND "))
				}
			}
		}
	} else {
		for odd, value := range values {
			if odd%2 == 0 {
				where = []string{}
				set = []string{}
				for i, v := range value {
					if v == "NULL" {
						where = append(where, fmt.Sprintf("@%d IS NULL", i))
					} else {
						where = append(where, fmt.Sprintf("@%d = %s", i, v))
					}
				}
			} else {
				for i, v := range value {
					set = append(set, fmt.Sprintf("@%d = %s", i, v))
				}
				if common.Config.Rebuild.WithoutDBName {
					fmt.Printf("-- %s %s SET %s WHERE %s LIMIT 1;\n", updatePrefix, shortTableName, strings.Join(set, ", "), strings.Join(where, " AND "))
				} else {
					fmt.Printf("-- %s %s SET %s WHERE %s LIMIT 1;\n", updatePrefix, table, strings.Join(set, ", "), strings.Join(where, " AND "))
				}
			}
		}
	}
}

// UpdateRollbackQuery ...
func UpdateRollbackQuery(event *replication.BinlogEvent) {
	var table string
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("-- Table: %s, Error: %s\n", table, strings.Split(fmt.Sprint(r), "\n")[0])
		}
	}()
	table = RowEventTable(event)
	ev := event.Event.(*replication.RowsEvent)
	values := BuildValues(ev)

	if common.Config.Rebuild.Replace {
		var insertValues [][]string
		for odd, value := range values {
			if odd%2 == 0 {
				insertValues = append(insertValues, value)
			}
		}
		insertQuery(table, insertValues)
	} else {
		updateRollbackQuery(table, values)
	}
}

func updateRollbackQuery(table string, values [][]string) {
	var where []string
	var set []string

	if ok := PrimaryKeys[table]; ok != nil {
		for odd, value := range values {
			if odd%2 == 0 {
				where = []string{}
				set = []string{}
				if len(common.Config.Rebuild.IgnoreColumns) > 0 {
					for i, col := range Columns[table] {
						ignore := false
						for _, c := range common.Config.Rebuild.IgnoreColumns {
							if c == strings.Trim(col, "`") {
								ignore = true
							}
						}
						if !ignore {
							set = append(set, fmt.Sprintf("%s = %s", col, value[i]))
						}
					}
				} else {
					for i, c := range Columns[table] {
						set = append(set, fmt.Sprintf("%s = %s", c, value[i]))
					}
				}
			} else {
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
				fmt.Printf("UPDATE %s SET %s WHERE %s LIMIT 1;\n", table, strings.Join(set, ", "), strings.Join(where, " AND "))
			}
		}
	} else {
		for odd, value := range values {
			if odd%2 == 0 {
				where = []string{}
				set = []string{}
				for i, v := range value {
					set = append(set, fmt.Sprintf("@%d = %s", i, v))
				}
			} else {
				for i, v := range value {
					if v == "NULL" {
						where = append(where, fmt.Sprintf("@%d IS NULL", i))
					} else {
						where = append(where, fmt.Sprintf("@%d = %s", i, v))
					}
				}
				fmt.Printf("-- UPDATE %s SET %s WHERE %s  LIMIT 1;\n", table, strings.Join(set, ", "), strings.Join(where, " AND "))
			}
		}
	}
}

// UpdateStat ...
func UpdateStat(event *replication.BinlogEvent) {
	table := RowEventTable(event)
	if TableStats[table] != nil {
		TableStats[table]["update"]++
	} else {
		TableStats[table] = map[string]int64{"update": 1}
	}

	ev := event.Event.(*replication.RowsEvent)
	values := BuildValues(ev)
	if RowsStats[table] != nil {
		RowsStats[table]["update"] += int64(len(values))
	} else {
		RowsStats[table] = map[string]int64{"update": int64(len(values))}
	}
}

// UpdateLua ...
func UpdateLua(event *replication.BinlogEvent) {
	if common.Config.Rebuild.LuaScript == "" || event == nil {
		return
	}

	table := RowEventTable(event)
	ev := event.Event.(*replication.RowsEvent)
	values := BuildValues(ev)

	// lua function
	f := lua.P{
		Fn:      Lua.GetGlobal("UpdateRewrite"),
		NRet:    0,
		Protect: true,
	}
	// lua value
	v := lua.LString(table)
	var where, set []string
	for odd, value := range values {
		if odd%2 == 0 {
			where = []string{}
			set = []string{}
			for _, v := range value {
				where = append(where, v)
			}
		} else {
			for _, v := range value {
				set = append(set, v)
			}

			LuaStringList("GoValuesWhere", where)
			LuaStringList("GoValuesSet", set)

			if err := Lua.CallByParam(f, v); err != nil {
				common.Log.Error(err.Error())
				return
			}
		}
	}
}

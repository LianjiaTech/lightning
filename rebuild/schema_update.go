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

	"github.com/pingcap/tidb/pkg/parser/ast"
)

// UpdateSchemaFromQuery keeps the in-memory table schemas in sync with the DDL
// statements found in binlog QUERY_EVENT, so that row events after a schema
// change are mapped to the correct columns. db is the current database of the
// event, used for unqualified table names.
func UpdateSchemaFromQuery(db, sql string) {
	sql = strings.TrimSpace(sql)
	if !isDDL(sql) {
		return
	}
	stmts, err := TiParse(sql, common.Config.Global.Charset, defaultCollation(common.Config.Global.Charset))
	if err != nil {
		common.VerboseVerbose("-- [DEBUG] UpdateSchemaFromQuery parse error: %s, sql: %s", err.Error(), sql)
		return
	}
	changed := false
	for _, stmt := range stmts {
		switch node := stmt.(type) {
		case *ast.CreateTableStmt:
			changed = addTableSchema(db, node) || changed
		case *ast.DropTableStmt:
			for _, t := range node.Tables {
				changed = dropTableSchema(db, t) || changed
			}
		case *ast.AlterTableStmt:
			changed = alterTableSchema(db, node) || changed
		case *ast.RenameTableStmt:
			for _, t := range node.TableToTables {
				changed = renameTableSchema(db, t) || changed
			}
		}
	}
	if changed {
		buildColumns()
		buildPrimaryKeys()
	}
}

// isDDL is a cheap pre-check to avoid parsing ordinary DML statements.
func isDDL(sql string) bool {
	fields := strings.Fields(sql)
	if len(fields) == 0 {
		return false
	}
	switch strings.ToUpper(fields[0]) {
	case "CREATE", "ALTER", "DROP", "RENAME":
		return true
	default:
		return false
	}
}

func schemaKey(schema, table string) string {
	return fmt.Sprintf("`%s`.`%s`", schema, table)
}

func tableSchemaKey(db string, t *ast.TableName) string {
	schema := t.Schema.String()
	if schema == "" {
		schema = db
	}
	return schemaKey(schema, t.Name.String())
}

func addTableSchema(db string, node *ast.CreateTableStmt) bool {
	if node.Table.Schema.String() == "" {
		node.Table.Schema = ast.NewCIStr(db)
	}
	Schemas[schemaKey(node.Table.Schema.String(), node.Table.Name.String())] = node
	return true
}

func dropTableSchema(db string, t *ast.TableName) bool {
	key := tableSchemaKey(db, t)
	if _, ok := Schemas[key]; ok {
		delete(Schemas, key)
		return true
	}
	return false
}

func renameTableSchema(db string, t *ast.TableToTable) bool {
	key := tableSchemaKey(db, t.OldTable)
	node, ok := Schemas[key]
	if !ok {
		return false
	}
	delete(Schemas, key)
	newSchema := t.NewTable.Schema.String()
	if newSchema == "" {
		newSchema = t.OldTable.Schema.String()
	}
	if newSchema == "" {
		newSchema = db
	}
	t.NewTable.Schema = ast.NewCIStr(newSchema)
	node.Table = t.NewTable
	Schemas[schemaKey(newSchema, t.NewTable.Name.String())] = node
	return true
}

func alterTableSchema(db string, node *ast.AlterTableStmt) bool {
	key := tableSchemaKey(db, node.Table)
	table, ok := Schemas[key]
	if !ok {
		return false
	}
	changed := false
	for _, spec := range node.Specs {
		switch spec.Tp {
		case ast.AlterTableAddColumns:
			table.Cols = append(table.Cols, spec.NewColumns...)
			changed = true
		case ast.AlterTableDropColumn:
			if spec.OldColumnName != nil {
				changed = dropColumn(table, spec.OldColumnName.Name.String()) || changed
			}
		case ast.AlterTableModifyColumn:
			if len(spec.NewColumns) == 1 {
				changed = replaceColumn(table, spec.NewColumns[0].Name.Name.String(), spec.NewColumns[0]) || changed
			}
		case ast.AlterTableChangeColumn:
			if spec.OldColumnName != nil && len(spec.NewColumns) == 1 {
				oldName := spec.OldColumnName.Name.String()
				if dropColumn(table, oldName) {
					table.Cols = append(table.Cols, spec.NewColumns[0])
					changed = true
				}
			}
		case ast.AlterTableRenameColumn:
			if spec.OldColumnName != nil && spec.NewColumnName != nil {
				changed = renameColumn(table, spec.OldColumnName.Name.String(), spec.NewColumnName.Name.String()) || changed
			}
		case ast.AlterTableAddConstraint:
			switch {
			case spec.Constraint != nil:
				table.Constraints = append(table.Constraints, spec.Constraint)
				changed = true
			case len(spec.NewConstraints) > 0:
				table.Constraints = append(table.Constraints, spec.NewConstraints...)
				changed = true
			}
		case ast.AlterTableDropPrimaryKey:
			changed = dropConstraint(table, ast.ConstraintPrimaryKey, "") || changed
		case ast.AlterTableDropIndex:
			changed = dropConstraint(table, ast.ConstraintKey, spec.Name) || changed
		case ast.AlterTableRenameTable:
			if spec.NewTable != nil {
				delete(Schemas, key)
				newSchema := spec.NewTable.Schema.String()
				if newSchema == "" {
					newSchema = table.Table.Schema.String()
				}
				if newSchema == "" {
					newSchema = db
				}
				spec.NewTable.Schema = ast.NewCIStr(newSchema)
				table.Table = spec.NewTable
				key = schemaKey(newSchema, spec.NewTable.Name.String())
				Schemas[key] = table
				changed = true
			}
		}
	}
	return changed
}

func dropColumn(table *ast.CreateTableStmt, name string) bool {
	for i, col := range table.Cols {
		if col.Name.Name.String() == name {
			table.Cols = append(table.Cols[:i], table.Cols[i+1:]...)
			return true
		}
	}
	return false
}

func replaceColumn(table *ast.CreateTableStmt, name string, newCol *ast.ColumnDef) bool {
	for i, col := range table.Cols {
		if col.Name.Name.String() == name {
			table.Cols[i] = newCol
			return true
		}
	}
	return false
}

func renameColumn(table *ast.CreateTableStmt, oldName, newName string) bool {
	for _, col := range table.Cols {
		if col.Name.Name.String() == oldName {
			col.Name.Name = ast.NewCIStr(newName)
			return true
		}
	}
	return false
}

func dropConstraint(table *ast.CreateTableStmt, tp ast.ConstraintType, name string) bool {
	for i, con := range table.Constraints {
		if con.Tp != tp {
			continue
		}
		if name != "" && con.Name != name {
			continue
		}
		table.Constraints = append(table.Constraints[:i], table.Constraints[i+1:]...)
		return true
	}
	return false
}

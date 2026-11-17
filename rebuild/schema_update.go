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
	// Only tables touched by this DDL need their column/PK indexes rebuilt.
	dirty := make(map[string]struct{})
	for _, stmt := range stmts {
		switch node := stmt.(type) {
		case *ast.CreateTableStmt:
			dirty[addTableSchema(db, node)] = struct{}{}
		case *ast.DropTableStmt:
			for _, t := range node.Tables {
				dropTableSchema(db, t, dirty)
			}
		case *ast.AlterTableStmt:
			alterTableSchema(db, node, dirty)
		case *ast.RenameTableStmt:
			for _, t := range node.TableToTables {
				renameTableSchema(db, t, dirty)
			}
		}
	}
	if len(dirty) == 0 {
		return
	}
	ensureSchemaIndexes()
	for key := range dirty {
		rebuildTableIndexes(key)
	}
}

// ensureSchemaIndexes allocates the derived indexes when LoadSchemaInfo did not
// populate them (e.g. schema loading failed).
func ensureSchemaIndexes() {
	if Columns == nil {
		Columns = make(map[string][]string)
	}
	if PrimaryKeys == nil {
		PrimaryKeys = make(map[string][]string)
	}
}

// rebuildTableIndexes refreshes the derived columns/primary keys of one table.
func rebuildTableIndexes(key string) {
	schema, ok := Schemas[key]
	if !ok {
		delete(Columns, key)
		delete(PrimaryKeys, key)
		return
	}
	buildTableColumns(key, schema)
	buildTablePrimaryKeys(key, schema)
}

// isDDL is a cheap pre-check to avoid parsing ordinary DML statements.
func isDDL(sql string) bool {
	kw := FirstKeyword(sql)
	switch {
	case strings.EqualFold(kw, "CREATE"),
		strings.EqualFold(kw, "ALTER"),
		strings.EqualFold(kw, "DROP"),
		strings.EqualFold(kw, "RENAME"):
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

func addTableSchema(db string, node *ast.CreateTableStmt) string {
	if node.Table.Schema.String() == "" {
		node.Table.Schema = ast.NewCIStr(db)
	}
	key := schemaKey(node.Table.Schema.String(), node.Table.Name.String())
	Schemas[key] = node
	return key
}

func dropTableSchema(db string, t *ast.TableName, dirty map[string]struct{}) {
	key := tableSchemaKey(db, t)
	if _, ok := Schemas[key]; ok {
		delete(Schemas, key)
		dirty[key] = struct{}{}
	}
}

func renameTableSchema(db string, t *ast.TableToTable, dirty map[string]struct{}) {
	key := tableSchemaKey(db, t.OldTable)
	node, ok := Schemas[key]
	if !ok {
		return
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
	newKey := schemaKey(newSchema, t.NewTable.Name.String())
	Schemas[newKey] = node
	dirty[key] = struct{}{}
	dirty[newKey] = struct{}{}
}

func alterTableSchema(db string, node *ast.AlterTableStmt, dirty map[string]struct{}) {
	key := tableSchemaKey(db, node.Table)
	table, ok := Schemas[key]
	if !ok {
		return
	}
	for _, spec := range node.Specs {
		switch spec.Tp {
		case ast.AlterTableAddColumns:
			table.Cols = append(table.Cols, spec.NewColumns...)
			dirty[key] = struct{}{}
		case ast.AlterTableDropColumn:
			if spec.OldColumnName != nil {
				if dropColumn(table, spec.OldColumnName.Name.String()) {
					dirty[key] = struct{}{}
				}
			}
		case ast.AlterTableModifyColumn:
			if len(spec.NewColumns) == 1 {
				if replaceColumn(table, spec.NewColumns[0].Name.Name.String(), spec.NewColumns[0]) {
					dirty[key] = struct{}{}
				}
			}
		case ast.AlterTableChangeColumn:
			if spec.OldColumnName != nil && len(spec.NewColumns) == 1 {
				oldName := spec.OldColumnName.Name.String()
				if dropColumn(table, oldName) {
					table.Cols = append(table.Cols, spec.NewColumns[0])
					dirty[key] = struct{}{}
				}
			}
		case ast.AlterTableRenameColumn:
			if spec.OldColumnName != nil && spec.NewColumnName != nil {
				if renameColumn(table, spec.OldColumnName.Name.String(), spec.NewColumnName.Name.String()) {
					dirty[key] = struct{}{}
				}
			}
		case ast.AlterTableAddConstraint:
			switch {
			case spec.Constraint != nil:
				table.Constraints = append(table.Constraints, spec.Constraint)
				dirty[key] = struct{}{}
			case len(spec.NewConstraints) > 0:
				table.Constraints = append(table.Constraints, spec.NewConstraints...)
				dirty[key] = struct{}{}
			}
		case ast.AlterTableDropPrimaryKey:
			if dropConstraint(table, ast.ConstraintPrimaryKey, "") {
				dirty[key] = struct{}{}
			}
		case ast.AlterTableDropIndex:
			if dropConstraint(table, ast.ConstraintKey, spec.Name) {
				dirty[key] = struct{}{}
			}
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
				newKey := schemaKey(newSchema, spec.NewTable.Name.String())
				Schemas[newKey] = table
				dirty[key] = struct{}{}
				dirty[newKey] = struct{}{}
				key = newKey
			}
		}
	}
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

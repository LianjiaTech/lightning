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
	"testing"

	"github.com/LianjiaTech/lightning/common"

	"github.com/pingcap/tidb/pkg/parser/ast"
)

func TestUpdateSchemaFromQuery(t *testing.T) {
	oldSchemas := Schemas
	oldCharset := common.Config.Global.Charset
	defer func() {
		Schemas = oldSchemas
		common.Config.Global.Charset = oldCharset
		buildColumns()
		buildPrimaryKeys()
	}()
	Schemas = map[string]*ast.CreateTableStmt{}
	common.Config.Global.Charset = "utf8mb4"

	key := "`test`.`t`"
	UpdateSchemaFromQuery("test", "CREATE TABLE t (a int primary key, b varchar(10))")
	if _, ok := Schemas[key]; !ok {
		t.Fatalf("CREATE TABLE not tracked, keys=%v", Schemas)
	}
	if len(Columns[key]) != 2 {
		t.Fatalf("after CREATE, columns = %v, want 2", Columns[key])
	}
	if len(PrimaryKeys[key]) != 1 || PrimaryKeys[key][0] != "`a`" {
		t.Fatalf("after CREATE, primary keys = %v, want [`a`]", PrimaryKeys[key])
	}

	UpdateSchemaFromQuery("test", "ALTER TABLE t ADD COLUMN c int")
	if len(Columns[key]) != 3 || Columns[key][2] != "`c`" {
		t.Fatalf("after ADD COLUMN, columns = %v", Columns[key])
	}

	UpdateSchemaFromQuery("test", "ALTER TABLE t RENAME COLUMN b TO bb")
	if len(Columns[key]) != 3 || Columns[key][1] != "`bb`" {
		t.Fatalf("after RENAME COLUMN, columns = %v", Columns[key])
	}

	UpdateSchemaFromQuery("test", "ALTER TABLE t MODIFY COLUMN c bigint")
	// column name stays, type change is not asserted beyond name
	if len(Columns[key]) != 3 || Columns[key][2] != "`c`" {
		t.Fatalf("after MODIFY COLUMN, columns = %v", Columns[key])
	}

	UpdateSchemaFromQuery("test", "ALTER TABLE t DROP COLUMN a")
	if len(Columns[key]) != 2 || Columns[key][0] != "`bb`" {
		t.Fatalf("after DROP COLUMN, columns = %v", Columns[key])
	}
	// no primary key left -> all columns become the key
	if len(PrimaryKeys[key]) != 2 {
		t.Fatalf("after DROP PRIMARY KEY, primary keys = %v", PrimaryKeys[key])
	}

	key2 := "`test`.`t2`"
	UpdateSchemaFromQuery("test", "RENAME TABLE t TO t2")
	if _, ok := Schemas[key]; ok {
		t.Fatalf("old table key still present after RENAME TABLE")
	}
	if _, ok := Schemas[key2]; !ok {
		t.Fatalf("renamed table not tracked, keys=%v", Schemas)
	}

	UpdateSchemaFromQuery("test", "DROP TABLE t2")
	if _, ok := Schemas[key2]; ok {
		t.Fatalf("table still present after DROP TABLE")
	}

	// DML must not touch the schema
	UpdateSchemaFromQuery("test", "INSERT INTO t VALUES (1)")
}

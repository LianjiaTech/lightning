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
	"os"
	"strings"
	"testing"

	"github.com/LianjiaTech/lightning/common"
	"github.com/kr/pretty"
	"github.com/pingcap/tidb/pkg/parser/ast"
)

func TestLoadSchemaInfo(t *testing.T) {
	TestLoadSchemaFromFile(t)

	TestLoadSchemaFromMySQL(t)
}

func TestLoadSchemaFromFile(t *testing.T) {
	schemaFileOrg := common.Config.MySQL.SchemaFile

	common.Config.MySQL.SchemaFile = common.DevPath + "/test/schema.sql"
	err := loadSchemaFromFile()
	pretty.Println(err, Schemas)

	common.Config.MySQL.SchemaFile = schemaFileOrg
}

func TestLoadSchemaFromMySQL(t *testing.T) {
	master := common.Config.MySQL.MasterInfo

	common.Config.MySQL.MasterInfo = common.DevPath + "/etc/master.info"
	common.LoadMasterInfo()
	err := loadSchemaFromMySQL()
	pretty.Println(err, Schemas)

	common.Config.MySQL.MasterInfo = master
}

func TestOnlyTable(t *testing.T) {
	tables := []string{
		"`db`.`tb`",
		"tb",
	}

	err := common.GoldenDiff(func() {
		for _, table := range tables {
			fmt.Println(onlyTable(table))
		}
	}, t.Name(), update)
	if nil != err {
		t.Fatal(err)
	}
}

func TestSaveAndLoadSchemaFile(t *testing.T) {
	oldSchemas, oldColumns, oldPrimaryKeys := Schemas, Columns, PrimaryKeys
	oldStatements := schemaStatements
	oldCharset := common.Config.Global.Charset
	defer func() {
		Schemas, Columns, PrimaryKeys = oldSchemas, oldColumns, oldPrimaryKeys
		schemaStatements = oldStatements
		common.Config.Global.Charset = oldCharset
		buildColumns()
		buildPrimaryKeys()
	}()
	common.Config.Global.Charset = "utf8mb4"

	schemaStatements = []schemaStatement{
		{database: "save_db", sql: "CREATE TABLE `t1` (`id` int primary key, `v` varchar(10))"},
		{database: "save_db", sql: "CREATE TABLE `t2` (`a` int, `b` int)"},
	}
	path := t.TempDir() + "/schema.sql"
	if err := saveSchemaFile(path); err != nil {
		t.Fatal(err.Error())
	}

	Schemas = map[string]*ast.CreateTableStmt{}
	Columns = nil
	PrimaryKeys = nil
	if err := loadSchemaFile(path); err != nil {
		t.Fatal(err.Error())
	}

	key := "`save_db`.`t1`"
	if len(Columns[key]) != 2 || Columns[key][0] != "`id`" {
		t.Fatalf("saved columns = %v", Columns[key])
	}
	if len(PrimaryKeys[key]) != 1 || PrimaryKeys[key][0] != "`id`" {
		t.Fatalf("saved primary keys = %v", PrimaryKeys[key])
	}
	if len(PrimaryKeys["`save_db`.`t2`"]) != 2 {
		t.Fatalf("saved primary keys = %v", PrimaryKeys["`save_db`.`t2`"])
	}
}

func TestEnsureTableSchemaLazyLoad(t *testing.T) {
	oldSchemas, oldColumns, oldPrimaryKeys := Schemas, Columns, PrimaryKeys
	oldFailed := schemaFailed
	oldLoader := loadTableDDL
	oldSchemaFile := common.Config.MySQL.SchemaFile
	oldCharset := common.Config.Global.Charset
	oldHost := common.MasterInfo.MasterHost
	defer func() {
		Schemas, Columns, PrimaryKeys = oldSchemas, oldColumns, oldPrimaryKeys
		schemaFailed = oldFailed
		loadTableDDL = oldLoader
		common.Config.MySQL.SchemaFile = oldSchemaFile
		common.Config.Global.Charset = oldCharset
		common.MasterInfo.MasterHost = oldHost
		buildColumns()
		buildPrimaryKeys()
	}()
	common.Config.Global.Charset = "utf8mb4"
	common.MasterInfo.MasterHost = "127.0.0.1"
	schemaFailed = make(map[string]bool)

	path := t.TempDir() + "/schema.sql"
	if err := os.WriteFile(path, []byte("USE `lazy_db`;\nCREATE TABLE `known` (`id` int primary key);\n"), 0644); err != nil {
		t.Fatal(err.Error())
	}
	common.Config.MySQL.SchemaFile = path
	Schemas = map[string]*ast.CreateTableStmt{}
	Columns = nil
	PrimaryKeys = nil
	if err := loadSchemaFile(path); err != nil {
		t.Fatal(err.Error())
	}

	calls := 0
	loadTableDDL = func(database, table string) (string, error) {
		calls++
		if database != "lazy_db" || table != "missing" {
			t.Errorf("unexpected lookup %s.%s", database, table)
		}
		return "CREATE TABLE `missing` (`id` bigint primary key, `v` int)", nil
	}

	key := "`lazy_db`.`missing`"
	EnsureTableSchema(key)
	if calls != 1 {
		t.Fatalf("load calls = %d, want 1", calls)
	}
	if len(Columns[key]) != 2 || Columns[key][1] != "`v`" {
		t.Fatalf("lazy columns = %v", Columns[key])
	}
	if len(PrimaryKeys[key]) != 1 || PrimaryKeys[key][0] != "`id`" {
		t.Fatalf("lazy primary keys = %v", PrimaryKeys[key])
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err.Error())
	}
	if !strings.Contains(string(content), "`missing`") {
		t.Fatalf("schema-file was not updated:\n%s", content)
	}

	// cached in memory: no second lookup, and known tables are never looked up
	EnsureTableSchema(key)
	EnsureTableSchema("`lazy_db`.`known`")
	if calls != 1 {
		t.Fatalf("unexpected extra lookups: %d", calls)
	}
}

func TestEnsureTableSchemaFailureNotRetried(t *testing.T) {
	oldSchemas, oldColumns, oldPrimaryKeys := Schemas, Columns, PrimaryKeys
	oldFailed := schemaFailed
	oldLoader := loadTableDDL
	oldSchemaFile := common.Config.MySQL.SchemaFile
	oldHost := common.MasterInfo.MasterHost
	defer func() {
		Schemas, Columns, PrimaryKeys = oldSchemas, oldColumns, oldPrimaryKeys
		schemaFailed = oldFailed
		loadTableDDL = oldLoader
		common.Config.MySQL.SchemaFile = oldSchemaFile
		common.MasterInfo.MasterHost = oldHost
		buildColumns()
		buildPrimaryKeys()
	}()
	schemaFailed = make(map[string]bool)
	common.MasterInfo.MasterHost = "127.0.0.1"
	common.Config.MySQL.SchemaFile = t.TempDir() + "/schema.sql"
	Schemas = map[string]*ast.CreateTableStmt{}

	calls := 0
	loadTableDDL = func(string, string) (string, error) {
		calls++
		return "", fmt.Errorf("no such table")
	}
	EnsureTableSchema("`lazy_db`.`gone`")
	EnsureTableSchema("`lazy_db`.`gone`")
	if calls != 1 {
		t.Fatalf("failed lookups retried: %d", calls)
	}
}

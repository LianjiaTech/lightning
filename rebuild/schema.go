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
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strings"

	"github.com/juju/errors"

	"github.com/LianjiaTech/lightning/common"

	// database/sql
	_ "github.com/go-sql-driver/mysql"
	"github.com/pingcap/parser/ast"
	"github.com/pingcap/parser/model"
	"github.com/pingcap/parser/mysql"
)

// Schemas ...
var Schemas map[string]*ast.CreateTableStmt

// Columns ...
var Columns map[string][]string

// PrimaryKeys ...
var PrimaryKeys map[string][]string

// LoadSchemaInfo load schema info from file or mysql
func LoadSchemaInfo() {
	if common.Config.MySQL.SchemaFile != "" {
		// load from file
		err := loadSchemaFromFile()
		if err != nil {
			common.Log.Error(errors.Trace(err).Error())
		}
		return
	} else {
		// load from mysql server
		err := loadSchemaFromMySQL()
		if err != nil {
			common.Log.Error(errors.Trace(err).Error())
		}
	}
}

func loadSchemaFromFile() error {
	common.Log.Debug("loadSchemaFromFile %s", common.Config.MySQL.SchemaFile)
	if _, err := os.Stat(common.Config.MySQL.SchemaFile); err != nil {
		return err
	}
	buf, err := ioutil.ReadFile(common.Config.MySQL.SchemaFile)
	if err != nil {
		return err
	}
	err = schemaAppend("", string(buf))
	buildColumns()
	buildPrimaryKeys()
	return err
}

func loadSchemaFromMySQL() error {
	var databases []string
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/?charset=%s&timeout=5s",
		common.MasterInfo.MasterUser,
		common.MasterInfo.MasterPassword,
		common.MasterInfo.MasterHost,
		common.MasterInfo.MasterPort,
		common.Config.Global.Charset,
	)
	common.Log.Debug("loadSchemaFromMySQL %s", dsn)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return err
	}
	defer db.Close()

	res, err := db.Query("SHOW DATABASES;")
	if err != nil {
		return err
	}
	for res.Next() {
		var database string
		res.Scan(&database)
		switch database {
		case "information_schema", "sys", "mysql", "performance_schema":
		default:
			databases = append(databases, database)
		}
	}
	res.Close()

	for _, database := range databases {
		res, err := db.Query(fmt.Sprintf("SHOW TABLES FROM `%s`", database))
		if err != nil {
			common.Log.Error(errors.Trace(err).Error())
			continue
		}

		// SHOW TABLES
		var tables []string
		for res.Next() {
			var table string
			err = res.Scan(&table)
			if err != nil {
				common.Log.Error(errors.Trace(err).Error())
				continue
			}
			tables = append(tables, table)
		}

		// SHOW CREATE TABLE
		for _, table := range tables {

			tableRes, err := db.Query(fmt.Sprintf("SHOW CREATE TABLE `%s`.`%s`;", database, table))
			if err != nil {
				common.Log.Error(errors.Trace(err).Error())
				continue
			}

			cols, err := tableRes.Columns()
			if err != nil {
				common.Log.Error(errors.Trace(err).Error())
				continue
			}
			// SHOW CREATE VIEW WILL GET 4 COLUMNS
			if len(cols) != 2 {
				common.Log.Info("by pass host: %s, port: %d, database: %s, table: %s",
					common.MasterInfo.MasterHost,
					common.MasterInfo.MasterPort,
					database, table)
				continue
			}

			for tableRes.Next() {
				var name, schema string
				err = tableRes.Scan(&name, &schema)
				if err != nil {
					common.Log.Error("host: %s, port: %d, database: %s, table: %s, error: %s",
						common.MasterInfo.MasterHost,
						common.MasterInfo.MasterPort,
						database, table, errors.Trace(err).Error())
					continue
				}
				err = schemaAppend(database, schema)
				if err != nil {
					common.Log.Error("host: %s, port: %d, database: %s, table: %s, sql: %s, error: %s",
						common.MasterInfo.MasterHost,
						common.MasterInfo.MasterPort,
						database, table,
						schema,
						errors.Trace(err).Error())
					schemaAppend(database, buildFakeTable(db, fmt.Sprintf("`%s`.`%s`", database, table)))
				}
			}
			tableRes.Close()
		}
		res.Close()
	}
	buildColumns()
	buildPrimaryKeys()
	return nil
}

func schemaAppend(database, sql string) error {
	Schemas = make(map[string]*ast.CreateTableStmt)
	sql = removeIncompatibleWords(sql)
	stmts, err := TiParse(sql, common.Config.Global.Charset, mysql.Charsets[common.Config.Global.Charset])
	if err != nil {
		return err
	}
	if database == "" {
		database = "%"
	}
	for _, stmt := range stmts {
		switch node := stmt.(type) {
		case *ast.CreateTableStmt:
			if node.Table.Schema.String() == "" {
				node.Table.Schema = model.NewCIStr(database)
			}
			Schemas[fmt.Sprintf("`%s`.`%s`", database, node.Table.Name)] = node
		case *ast.UseStmt:
			database = node.DBName
		}
	}
	return nil
}

// removeIncompatibleWords remove pingcap/parser not support words from schema
// Note: only for MySQL `SHOW CREATE TABLE` hand-writing SQL not compatible
func removeIncompatibleWords(sql string) string {
	// CONSTRAINT col_fk FOREIGN KEY (col) REFERENCES tb (id) ON UPDATE CASCADE
	re := regexp.MustCompile(` ON UPDATE CASCADE`)
	sql = re.ReplaceAllString(sql, "")

	// FULLTEXT KEY col_fk (col) /*!50100 WITH PARSER `ngram` */
	// /*!50100 PARTITION BY LIST (col)
	re = regexp.MustCompile(`/\*!5`)
	sql = re.ReplaceAllString(sql, "/* 5")

	// col varchar(10) CHARACTER SET gbk DEFAULT NULL
	re = regexp.MustCompile(`CHARACTER SET [a-z_0-9]* `)
	sql = re.ReplaceAllString(sql, "")

	return sql
}

// buildColumns build column name list
func buildColumns() {
	Columns = make(map[string][]string)
	for _, schema := range Schemas {
		table := fmt.Sprintf("`%s`.`%s`", schema.Table.Schema.String(), schema.Table.Name.String())
		for _, col := range schema.Cols {
			Columns[table] = append(Columns[table], fmt.Sprintf("`%s`", col.Name.String()))
		}
	}
}

// buildPrimaryKeys build primary key list
func buildPrimaryKeys() {
	PrimaryKeys = make(map[string][]string)
	for _, schema := range Schemas {
		table := fmt.Sprintf("`%s`.`%s`", schema.Table.Schema.String(), schema.Table.Name.String())
		for _, con := range schema.Constraints {
			if con.Tp == ast.ConstraintPrimaryKey {
				for _, col := range con.Keys {
					PrimaryKeys[table] = append(PrimaryKeys[table], fmt.Sprintf("`%s`", col.Column.String()))
				}
			}
		}
		// 如果表没有主键，把表的所有列合起来当主键
		if len(PrimaryKeys[table]) == 0 {
			PrimaryKeys[table] = Columns[table]
		}
	}
}

// buildFakeTable ...
func buildFakeTable(db *sql.DB, table string) string {
	var col, key string
	var t []byte
	var columns, primary []string
	res, err := db.Query(fmt.Sprintf("SHOW COLUMNS FROM %s", table))
	if err != nil {
		common.Log.Error(err.Error())
		return ""
	}
	defer res.Close()
	for res.Next() {
		res.Scan(&col, &t, &t, &key, &t, &t)
		columns = append(columns, fmt.Sprintf("`%s` INT", col))
		if key == "PRI" {
			primary = append(primary, fmt.Sprintf("`%s`", col))
		}
	}
	return fmt.Sprintf("CREATE TABLE %s (%s %s);", table, strings.Join(columns, ","), fmt.Sprintf(", PRIMARY KEY (%s)", strings.Join(primary, ",")))
}

func onlyTable(table string) string {
	tup := strings.Split(strings.Trim(table, "`"), "`.`")
	length := len(tup)
	if length <= 0 {
		return ""
	}
	return fmt.Sprint("`", tup[length-1], "`")
}

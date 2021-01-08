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
	"testing"

	"github.com/LianjiaTech/lightning/common"
	"github.com/kr/pretty"
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

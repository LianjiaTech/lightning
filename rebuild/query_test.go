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
	"flag"
	"testing"

	"github.com/LianjiaTech/lightning/common"
)

var update = flag.Bool("update", false, "update .golden files")

func TestQueryRollback(t *testing.T) {
	sqls := []string{
		`CREATE TABLE tb (a int)`,
		`create database db`,
		// "create index on tb idx_col (`col`)",
	}

	err := common.GoldenDiff(func() {
		for _, sql := range sqls {
			QueryRollback(sql)
		}
	}, t.Name(), update)
	if nil != err {
		t.Fatal(err)
	}

}

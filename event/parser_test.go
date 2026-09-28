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

package event

import (
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/LianjiaTech/lightning/common"
	"github.com/LianjiaTech/lightning/rebuild"
)

func init() {
	common.Config.MySQL.SchemaFile = common.DevPath + "/test/schema.sql"
	rebuild.LoadSchemaInfo()
}

func ExampleCheckBinlogFileHeader() {
	headers := [][]byte{
		{0xfe, 'b', 'i', 'n'}, // not encrypted
		{0xfd, 'b', 'i', 'n'}, // encrypted
		{0xfe, 'g', 'i', 'f'}, // wrong file header
	}
	for _, header := range headers {
		fmt.Println("CheckBinlogFileHeader", header, CheckBinlogFileHeader(header))
		fmt.Println("CheckBinlogFileEncrypt", header, CheckBinlogFileEncrypt(header))
	}
	// Output:
	// CheckBinlogFileHeader [254 98 105 110] true
	// CheckBinlogFileEncrypt [254 98 105 110] false
	// CheckBinlogFileHeader [253 98 105 110] true
	// CheckBinlogFileEncrypt [253 98 105 110] true
	// CheckBinlogFileHeader [254 103 105 102] false
	// CheckBinlogFileEncrypt [254 103 105 102] false
}

func TestBinlogFileParser(t *testing.T) {
	err := BinlogFileParser([]string{common.DevPath + "/test/binlog.000002"})
	if err != nil {
		t.Error(err.Error())
	}
}

// TestBinlogFileParserCompressed verifies that a compressed transaction
// (binlog_transaction_compression=ON, MySQL 8.0.20+) delivered as a single
// TRANSACTION_PAYLOAD_EVENT is expanded and every inner row event is rebuilt.
func TestBinlogFileParserCompressed(t *testing.T) {
	schemaOrg := common.Config.MySQL.SchemaFile
	pluginOrg := common.Config.Rebuild.Plugin
	common.Config.MySQL.SchemaFile = common.DevPath + "/test/schema.compressed.sql"
	common.Config.Rebuild.Plugin = "sql"
	rebuild.LoadSchemaInfo()

	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err.Error())
	}
	os.Stdout = w

	parseErr := BinlogFileParser([]string{common.DevPath + "/test/binlog.compressed"})

	w.Close()
	os.Stdout = old
	out, _ := io.ReadAll(r)

	common.Config.MySQL.SchemaFile = schemaOrg
	common.Config.Rebuild.Plugin = pluginOrg

	if parseErr != nil {
		t.Fatal(parseErr.Error())
	}
	for _, want := range []string{
		"INSERT INTO `test`.`t_compress`  VALUES (1, \"compressed-a\")",
		"INSERT INTO `test`.`t_compress`  VALUES (2, \"compressed-b\")",
		"UPDATE `test`.`t_compress` SET `id` = 1, `v` = \"compressed-upd\" WHERE `id` = 1 LIMIT 1",
		"DELETE FROM `test`.`t_compress` WHERE `id` = 2 LIMIT 1",
	} {
		if !strings.Contains(string(out), want) {
			t.Errorf("compressed binlog output missing: %q\n--- got ---\n%s", want, out)
		}
	}
}

func TestBinlogStreamParser(t *testing.T) {
	masterInfoOrg := common.Config.MySQL.MasterInfo
	stopPositionOrg := common.Config.Filters.StopPosition
	replicateFromCurrentOrg := common.Config.MySQL.ReplicateFromCurrentPosition
	common.Config.MySQL.MasterInfo = common.DevPath + "/etc/master.info"
	common.Config.Filters.StopPosition = 190
	// 从当前位点开始，避免 binlog 文件不存在的问题
	common.Config.MySQL.ReplicateFromCurrentPosition = true
	common.LoadMasterInfo()
	// 清空 binlog 文件名，强制从当前位点开始
	common.MasterInfo.MasterLogFile = ""
	err := BinlogStreamParser()
	if err != nil {
		t.Error(err.Error())
	}
	common.Config.MySQL.MasterInfo = masterInfoOrg
	common.Config.Filters.StopPosition = stopPositionOrg
	common.Config.MySQL.ReplicateFromCurrentPosition = replicateFromCurrentOrg
}

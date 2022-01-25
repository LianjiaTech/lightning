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
	"testing"

	"github.com/LianjiaTech/lightning/common"
	"github.com/LianjiaTech/lightning/rebuild"
)

func init() {
	common.Config.MySQL.SchemaFile = common.DevPath + "/test/schema.sql"
	rebuild.LoadSchemaInfo()
}

func ExampleBinlogFileValidator() {
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

func TestBinlogStreamParser(t *testing.T) {
	masterInfoOrg := common.Config.MySQL.MasterInfo
	stopPositionOrg := common.Config.Filters.StopPosition
	common.Config.MySQL.MasterInfo = common.DevPath + "/etc/master.info"
	common.Config.Filters.StopPosition = 190
	common.LoadMasterInfo()
	err := BinlogStreamParser()
	if err != nil {
		t.Error(err.Error())
	}
	common.Config.MySQL.MasterInfo = masterInfoOrg
	common.Config.Filters.StopPosition = stopPositionOrg
}

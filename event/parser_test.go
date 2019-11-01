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
	"testing"

	"github.com/LianjiaTech/lightning/common"
	"github.com/LianjiaTech/lightning/rebuild"
)

func init() {
	common.Config.MySQL.SchemaFile = common.DevPath + "/test/schema.sql"
	rebuild.LoadSchemaInfo()
}

func TestBinlogFileValidator(t *testing.T) {
	headersRight := [][]byte{
		{0xfe, 'b', 'i', 'n'},
	}
	headersWrong := [][]byte{
		{0xfe, 'g', 'i', 'f'},
	}
	for _, head := range headersRight {
		if !CheckBinlogFileHeader(head) {
			t.Error("CheckBinlogFileHeader should true")
		}
	}

	for _, head := range headersWrong {
		if CheckBinlogFileHeader(head) {
			t.Error("CheckBinlogFileHeader should false")
		}
	}
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

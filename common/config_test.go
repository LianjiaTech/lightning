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

package common

import (
	"flag"
	"fmt"
	"testing"
	"time"

	"github.com/kr/pretty"
)

var update = flag.Bool("update", false, "update .golden files")

func TestLoadReplicationInfo(t *testing.T) {
	masterInfoOrg := Config.MySQL.MasterInfo
	Config.MySQL.MasterInfo = DevPath + "/etc/master.info"
	LoadMasterInfo()
	pretty.Println(MasterInfo)
	Config.MySQL.MasterInfo = masterInfoOrg
}

func TestPrintConfiguration(t *testing.T) {
	err := GoldenDiff(func() {
		PrintConfiguration()
	}, t.Name(), update)
	if nil != err {
		t.Fatal(err)
	}
}

func TestListPlugin(t *testing.T) {
	err := GoldenDiff(func() {
		ListPlugin()
	}, t.Name(), update)
	if nil != err {
		t.Fatal(err)
	}
}

func TestTimeOffset(t *testing.T) {
	tzs := []string{
		"UTC",
		"Asia/Shanghai",
		"Europe/London",
		"America/Los_Angeles",
	}

	err := GoldenDiff(func() {
		for _, tz := range tzs {
			fmt.Println(TimeOffset(tz))
		}
	}, t.Name(), update)
	if nil != err {
		t.Fatal(err)
	}
}

func TestFlushReplicationInfo(t *testing.T) {
	masterInfoOrg := Config.MySQL.MasterInfo
	Config.MySQL.MasterInfo = DevPath + "/common/fixture/master.info"
	FlushReplicationInfo()
	Config.MySQL.MasterInfo = masterInfoOrg
}

func TestSyncReplicationInfo(t *testing.T) {
	durationOrg := Config.MySQL.SyncDuration
	Config.MySQL.SyncDuration = time.Duration(0 * time.Second)
	SyncReplicationInfo()
	Config.MySQL.SyncDuration = durationOrg
}

func TestParseConfig(t *testing.T) {
	ParseConfig()
	pretty.Println(Config, MasterInfo)
}

func TestParseConfigFile(t *testing.T) {
	var cfg *Configuration
	cfg.parseConfigFile(DevPath + "/etc/lightning.yaml")
	pretty.Println(Config)
}

func TestVersion(t *testing.T) {
	version()
}

func TestPrintMasterInfo(t *testing.T) {
	PrintMasterInfo()
}

func TestShowMasterStatus(t *testing.T) {
	// dsn := `root:******@tcp(127.0.0.1:3306)/`
	masterInfo := ChangeMaster{
		MasterUser:     "root",
		MasterPassword: "******",
		MasterHost:     "127.0.0.1",
		MasterPort:     3306,
	}
	pretty.Println(ShowMasterStatus(masterInfo))
}

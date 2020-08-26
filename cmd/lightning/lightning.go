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

package main

import (
	"github.com/LianjiaTech/lightning/common"
	"github.com/LianjiaTech/lightning/event"
	"github.com/LianjiaTech/lightning/rebuild"
	// "github.com/pkg/profile"
)

func main() {
	// defer profile.Start(profile.CPUProfile).Stop()

	// load config from lightning.yaml, master.info, relay.info, command lines
	common.ParseConfig()

	// load table schema info from mysql or create table SQL file
	rebuild.LoadSchemaInfo()

	// load lua script
	rebuild.LoadLuaScript()

	go common.SyncReplicationInfo()

	// binlog parser file || stream
	event.BinlogParser()

	// query stat info which need print at last
	rebuild.LastStatus()
}

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
	"os"
	"runtime"
	"runtime/pprof"

	"github.com/LianjiaTech/lightning/common"
	"github.com/LianjiaTech/lightning/event"
	"github.com/LianjiaTech/lightning/rebuild"
)

func main() {
	// load config from lightning.yaml, master.info, relay.info, command lines
	common.ParseConfig()
	// generated SQL goes through a buffered writer
	defer common.FlushOut()

	if path := common.Config.Global.CPUProfile; path != "" {
		f, err := os.Create(path)
		if err != nil {
			println("create cpu profile:", err.Error())
		} else {
			if err := pprof.StartCPUProfile(f); err != nil {
				println("start cpu profile:", err.Error())
			}
			defer func() {
				pprof.StopCPUProfile()
				f.Close()
			}()
		}
	}
	if path := common.Config.Global.MemProfile; path != "" {
		defer func() {
			runtime.GC()
			f, err := os.Create(path)
			if err != nil {
				println("create mem profile:", err.Error())
				return
			}
			defer f.Close()
			if err := pprof.WriteHeapProfile(f); err != nil {
				println("write mem profile:", err.Error())
			}
		}()
	}

	// load table schema info from mysql or create table SQL file.
	// The stat plugin only needs table names and row counts, so loading and
	// maintaining schemas is pure overhead there.
	if common.Config.Rebuild.Plugin != "stat" {
		rebuild.LoadSchemaInfo()
	}

	// load lua script
	rebuild.LoadLuaScript()

	go common.SyncReplicationInfo()

	// binlog parser file || stream
	event.BinlogParser()

	// query stat info which need print at last
	rebuild.LastStatus()
}

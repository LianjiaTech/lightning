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
	"strings"
	"time"

	"github.com/LianjiaTech/lightning/common"
	"github.com/LianjiaTech/lightning/rebuild"

	"github.com/go-mysql-org/go-mysql/replication"
	uuid "github.com/satori/go.uuid"
)

var FollowGTID bool
var FollowThreadID bool
var Ending bool
var Starting bool

// FilterThreadID ...
func FilterThreadID(event *replication.BinlogEvent) bool {
	var do bool
	if common.Config.Filters.ThreadID == 0 {
		return true
	}
	var threadId uint32
	switch event.Header.EventType {
	case replication.QUERY_EVENT:
		threadId = event.Event.(*replication.QueryEvent).SlaveProxyID
		if threadId == uint32(common.Config.Filters.ThreadID) {
			do = true
			FollowThreadID = do
		} else {
			FollowThreadID = false
		}
	default:
		common.VerboseVerbose("-- [DEBUG] FilterThreadID do: %v, Table: %s", FollowThreadID, threadId)
		return FollowThreadID
	}
	common.VerboseVerbose("-- [DEBUG] FilterThreadID do: %v, Table: %s", do, threadId)
	return do
}

// FilterTables ...
func FilterTables(event *replication.BinlogEvent) bool {
	var do bool
	if len(common.Config.Filters.Tables) == 0 {
		do = true
	}
	table := rebuild.RowEventTable(event)
	for _, filter := range common.Config.Filters.Tables {
		if tableFilterMatch(table, filter) {
			do = true
			break
		}
	}
	common.VerboseVerbose("-- [DEBUG] FilterTables do: %v, Table: %s", do, table)
	return do
}

// FilterIgnoreTables ...
func FilterIgnoreTables(event *replication.BinlogEvent) bool {
	do := true
	if len(common.Config.Filters.IgnoreTables) == 0 {
		return true
	}
	table := rebuild.RowEventTable(event)
	for _, filter := range common.Config.Filters.IgnoreTables {
		if tableFilterMatch(table, filter) {
			do = false
			break
		}
	}
	common.VerboseVerbose("-- [DEBUG] FilterIgnoreTables do: %v, Table: %s", do, table)
	return do
}

// FilterStartDatetime ...
func FilterStartDatetime(event *replication.BinlogEvent) bool {
	var do bool
	if common.Config.Filters.StartTimestamp == 0 {
		do = true
	}
	if int64(event.Header.Timestamp) >= common.Config.Filters.StartTimestamp {
		do = true
	}
	return do
}

// FilterStopDatetime ...
func FilterStopDatetime(event *replication.BinlogEvent) bool {
	var do bool
	if common.Config.Filters.StopTimestamp == 0 {
		do = true
		return do
	}
	if int64(event.Header.Timestamp) <= common.Config.Filters.StopTimestamp {
		do = true
	} else {
		Ending = true
	}
	return do
}

// FilterServerID ...
func FilterServerID(event *replication.BinlogEvent) bool {
	var do bool
	if common.Config.Filters.ServerID == 0 {
		do = true
	}
	if event.Header.ServerID == uint32(common.Config.Filters.ServerID) {
		do = true
	}
	return do
}

// FilterIncludeGTIDs ...
func FilterIncludeGTIDs(event *replication.BinlogEvent) bool {
	var do bool
	if common.Config.Filters.IncludeGTIDSet == "" {
		return true
	}
	switch event.Header.EventType {
	case replication.GTID_EVENT:
		do = InGTIDSet(event.Event.(*replication.GTIDEvent).SID, event.Event.(*replication.GTIDEvent).GNO, common.Config.Filters.IncludeGTIDSet)
		if FollowGTID && !do {
			Ending = true
		}
		FollowGTID = do
	default:
		do = FollowGTID
	}
	return do
}

// FilterExcludeGTIDs ...
func FilterExcludeGTIDs(event *replication.BinlogEvent) bool {
	var do bool
	if common.Config.Filters.ExcludeGTIDSet == "" {
		return true
	}
	switch event.Header.EventType {
	case replication.GTID_EVENT:
		do = !InGTIDSet(event.Event.(*replication.GTIDEvent).SID, event.Event.(*replication.GTIDEvent).GNO, common.Config.Filters.ExcludeGTIDSet)
		FollowGTID = do
	default:
		do = FollowGTID
	}
	return do
}

// FilterStartPos ...
func FilterStartPos(event *replication.BinlogEvent) bool {
	var do bool
	if common.Config.Filters.StartPosition == 0 || Starting {
		do = true
	}
	if event.Header.LogPos >= common.Config.Filters.StartPosition {
		do = true
		Starting = true
	}
	return do
}

// FilterStopPos ...
func FilterStopPos(event *replication.BinlogEvent) bool {
	var do bool
	if common.Config.Filters.StopPosition == 0 {
		do = true
		return do
	}
	if event.Header.LogPos <= common.Config.Filters.StopPosition {
		do = true
	} else {
		Ending = true
	}
	return do
}

// FilterQueryType ...
func FilterQueryType(event *replication.BinlogEvent) bool {
	var do bool
	if len(common.Config.Filters.EventType) == 0 {
		return true
	}

	for _, t := range common.Config.Filters.EventType {
		switch event.Header.EventType {
		case replication.WRITE_ROWS_EVENTv2, replication.WRITE_ROWS_EVENTv1, replication.WRITE_ROWS_EVENTv0:
			if strings.ToLower(t) == "insert" {
				do = true
			}
		case replication.UPDATE_ROWS_EVENTv2, replication.UPDATE_ROWS_EVENTv1, replication.UPDATE_ROWS_EVENTv0:
			if strings.ToLower(t) == "update" {
				do = true
			}
		case replication.DELETE_ROWS_EVENTv2, replication.DELETE_ROWS_EVENTv1, replication.DELETE_ROWS_EVENTv0:
			if strings.ToLower(t) == "delete" {
				do = true
			}
		case replication.QUERY_EVENT:
			prefix := strings.Fields(string(event.Event.(*replication.QueryEvent).Query))[0]
			if strings.ToLower(t) == prefix {
				do = true
			}
		default:
		}
		if do {
			break
		}
	}
	return do
}

// UpdateMasterInfo ...
func UpdateMasterInfo(event *replication.BinlogEvent) {
	switch event.Header.EventType {
	case replication.ROTATE_EVENT:
		nextFile := string(event.Event.(*replication.RotateEvent).NextLogName)
		if nextFile != common.MasterInfo.MasterLogFile {
			common.MasterInfo.MasterLogFile = nextFile
			common.MasterInfo.MasterLogPos = 4
		}
	case replication.QUERY_EVENT:
		common.MasterInfo.MasterLogPos = int64(event.Header.LogPos)
	case replication.XID_EVENT:
		common.MasterInfo.MasterLogPos = int64(event.Header.LogPos)
		executedGTIDSet := fmt.Sprint(event.Event.(*replication.XIDEvent).GSet)
		if executedGTIDSet != "<nil>" {
			common.MasterInfo.ExecutedGTIDSet = executedGTIDSet
		}
	default:
	}
	common.MasterInfo.SecondsBehindMaster = time.Now().Unix() - int64(event.Header.Timestamp)
	if common.Config.MySQL.SyncDuration.Seconds() == 0 {
		common.FlushReplicationInfo()
	}
}

// BinlogFilter check if event will do
func BinlogFilter(event *replication.BinlogEvent) bool {
	if !FilterStopPos(event) {
		return false
	}
	if !FilterStartPos(event) {
		return false
	}
	if !FilterThreadID(event) {
		return false
	}
	if !FilterExcludeGTIDs(event) {
		return false
	}
	if !FilterIncludeGTIDs(event) {
		return false
	}
	if !FilterServerID(event) {
		return false
	}
	if !FilterStopDatetime(event) {
		return false
	}
	if !FilterStartDatetime(event) {
		return false
	}
	if !FilterTables(event) {
		return false
	}
	if !FilterIgnoreTables(event) {
		return false
	}
	if !FilterQueryType(event) {
		return false
	}
	return true
}

func tableFilterMatch(table, filter string) bool {
	var match, dbMatch, tbMatch bool
	table = strings.Replace(table, "`", "", -1)
	schema := strings.Split(table, ".")
	if len(schema) < 2 {
		return match
	}
	sep := strings.Split(filter, ".")
	if len(sep) < 2 {
		common.Log.Error("tableFilterMatch, -tables: '%s' filter format error", filter)
		return match
	}

	// 库表名大小写不敏感
	if sep[0] == "%" {
		dbMatch = true
	}
	// 当 -schema 指定的文件中只有 CREATE TABLE 忘了写 USE db 的时候，schema[0] 为 %
	if schema[0] == "%" {
		dbMatch = true
	}
	if i := strings.Index(sep[0], "%"); i > 0 {
		if strings.HasPrefix(schema[0], sep[0][0:i]) {
			dbMatch = true
		}
	}
	if strings.ToLower(schema[0]) == strings.ToLower(sep[0]) {
		dbMatch = true
	}

	if sep[1] == "%" {
		tbMatch = true
	}
	if i := strings.Index(sep[1], "%"); i > 0 {
		if strings.HasPrefix(strings.ToLower(schema[1]), strings.ToLower(sep[1][0:i])) {
			tbMatch = true
		}
	}
	if schema[1] == sep[1] {
		tbMatch = true
	}
	match = dbMatch && tbMatch
	return match
}

// InGTIDSet ...
func InGTIDSet(sid []byte, gno int64, gtidSet string) bool {
	var gtidSets [][]string
	for _, set := range strings.Split(gtidSet, ",") {
		var couple []string
		tmp := strings.Split(strings.TrimSpace(set), ":")
		if len(tmp) != 2 {
			return true
		}
		couple = append(couple, tmp[0])
		couple = append(couple, strings.Split(tmp[1], "-")...)
		gtidSets = append(gtidSets, couple)
	}
	for _, set := range gtidSets {
		s, _ := uuid.FromBytes(sid)
		if len(set) != 3 {
			continue
		}
		if set[0] == s.String() {
			if strings.Compare(set[1], fmt.Sprint(gno)) <= 0 &&
				strings.Compare(set[2], fmt.Sprint(gno)) >= 0 {
				return true
			}
		}
	}
	return false
}

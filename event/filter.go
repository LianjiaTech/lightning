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
	"bytes"
	"fmt"
	"strconv"
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
	if len(common.Config.Filters.Tables) == 0 {
		return true
	}
	if !tableFiltersReady {
		prepareTableFilters()
	}
	var do bool
	table := rebuild.RowEventTable(event)
	for i := range includeTableFilters {
		if includeTableFilters[i].match(table) {
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
	case replication.GTID_TAGGED_LOG_EVENT:
		tagged := event.Event.(*replication.GtidTaggedLogEvent)
		do = InGTIDSet(tagged.SID, tagged.GNO, common.Config.Filters.IncludeGTIDSet)
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
	case replication.GTID_TAGGED_LOG_EVENT:
		tagged := event.Event.(*replication.GtidTaggedLogEvent)
		do = !InGTIDSet(tagged.SID, tagged.GNO, common.Config.Filters.ExcludeGTIDSet)
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
	if len(common.Config.Filters.EventType) == 0 {
		return true
	}

	var eventType string
	switch event.Header.EventType {
	case replication.WRITE_ROWS_EVENTv2, replication.WRITE_ROWS_EVENTv1, replication.WRITE_ROWS_EVENTv0:
		eventType = "insert"
	case replication.UPDATE_ROWS_EVENTv2, replication.UPDATE_ROWS_EVENTv1, replication.UPDATE_ROWS_EVENTv0, replication.PARTIAL_UPDATE_ROWS_EVENT:
		eventType = "update"
	case replication.DELETE_ROWS_EVENTv2, replication.DELETE_ROWS_EVENTv1, replication.DELETE_ROWS_EVENTv0:
		eventType = "delete"
	case replication.QUERY_EVENT:
		eventType = rebuild.FirstKeyword(string(event.Event.(*replication.QueryEvent).Query))
	default:
		return false
	}

	// Config.Filters.EventType is normalized to lower case at startup, which
	// keeps the historical case-sensitive comparison with the raw query
	// keyword without lower-casing the filter list for every event.
	for _, t := range common.Config.Filters.EventType {
		if t == eventType {
			return true
		}
	}
	return false
}

const masterInfoSyncMinInterval = 100 * time.Millisecond

var lastMasterInfoSync time.Time

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
	case replication.XA_PREPARE_LOG_EVENT:
		// XA transaction prepare phase (MySQL 8.0.30+, decoded by go-mysql as
		// GenericEvent); only track the replication position.
		common.MasterInfo.MasterLogPos = int64(event.Header.LogPos)
	default:
	}

	// START SLAVE UNTIL MASTER_LOG_FILE = 'log_name', MASTER_LOG_POS = log_pos
	if common.MasterInfo.MasterLogFile == common.MasterInfo.UntilLogFile &&
		common.MasterInfo.UntilLogPos <= common.MasterInfo.MasterLogPos {
		Ending = true
	}
	common.MasterInfo.SecondsBehindMaster = time.Now().Unix() - int64(event.Header.Timestamp)
	if common.Config.MySQL.SyncDuration.Seconds() == 0 {
		// sync-interval 0 means "flush as soon as possible", but rewriting
		// master.info for every event costs ~60us and dominates a streaming
		// run. Flush at transaction boundaries, throttled to at most 10 times
		// per second; the in-memory position still advances on every event.
		switch event.Header.EventType {
		case replication.XID_EVENT, replication.QUERY_EVENT, replication.ROTATE_EVENT, replication.TRANSACTION_PAYLOAD_EVENT:
			if now := time.Now(); now.Sub(lastMasterInfoSync) >= masterInfoSyncMinInterval {
				lastMasterInfoSync = now
				common.FlushReplicationInfo()
			}
		}
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

// tableFilter is a pre-compiled -tables/-ignore-tables entry.
type tableFilter struct {
	dbAny    bool
	dbExact  string
	dbPrefix string
	tbAny    bool
	tbExact  string
	tbPrefix string // lower case
}

var (
	includeTableFilters []tableFilter
	ignoreTableFilters  []tableFilter
	tableFiltersReady   bool
)

// prepareTableFilters compiles the configured table filters once instead of
// splitting and lower-casing them for every event.
func prepareTableFilters() {
	includeTableFilters = compileTableFilters(common.Config.Filters.Tables)
	ignoreTableFilters = compileTableFilters(common.Config.Filters.IgnoreTables)
	tableFiltersReady = true
}

func compileTableFilters(filters []string) []tableFilter {
	compiled := make([]tableFilter, 0, len(filters))
	for _, filter := range filters {
		if f, ok := compileTableFilter(filter); ok {
			compiled = append(compiled, f)
		}
	}
	return compiled
}

func compileTableFilter(filter string) (tableFilter, bool) {
	// keep the historical behaviour of ignoring anything after the table part
	parts := strings.SplitN(filter, ".", 3)
	if len(parts) < 2 {
		common.Log.Error("tableFilterMatch, -tables: '%s' filter format error", filter)
		return tableFilter{}, false
	}
	f := tableFilter{dbExact: parts[0], tbExact: parts[1]}
	if f.dbExact == "%" {
		f.dbAny = true
	}
	if i := strings.Index(f.dbExact, "%"); i > 0 {
		f.dbPrefix = f.dbExact[:i]
	}
	if f.tbExact == "%" {
		f.tbAny = true
	}
	if i := strings.Index(f.tbExact, "%"); i > 0 {
		f.tbPrefix = strings.ToLower(f.tbExact[:i])
	}
	return f, true
}

// match reports whether a `schema`.`table` name matches the filter.
func (f *tableFilter) match(table string) bool {
	db, tb, ok := splitTableName(table)
	if !ok {
		return false
	}

	dbMatch := f.dbAny || db == "%"
	if !dbMatch && f.dbPrefix != "" && strings.HasPrefix(db, f.dbPrefix) {
		dbMatch = true
	}
	if !dbMatch && strings.EqualFold(db, f.dbExact) {
		dbMatch = true
	}

	tbMatch := f.tbAny
	if !tbMatch && f.tbPrefix != "" && strings.HasPrefix(strings.ToLower(tb), f.tbPrefix) {
		tbMatch = true
	}
	if !tbMatch && tb == f.tbExact {
		tbMatch = true
	}
	return dbMatch && tbMatch
}

// splitTableName splits a table name (with or without backticks) without
// allocating.
func splitTableName(table string) (db, tb string, ok bool) {
	db, rest, ok := strings.Cut(table, ".")
	if !ok {
		return "", "", false
	}
	tb, _, _ = strings.Cut(rest, ".")
	return strings.Trim(db, "`"), strings.Trim(tb, "`"), true
}

func tableFilterMatch(table, filter string) bool {
	f, ok := compileTableFilter(filter)
	if !ok {
		return false
	}
	return f.match(table)
}

// InGTIDSet ...
func InGTIDSet(sid []byte, gno int64, gtidSet string) bool {
	m := compileGTIDSet(gtidSet)
	if m.all {
		return true
	}
	gnoStr := strconv.FormatInt(gno, 10)
	for i := range m.ranges {
		r := &m.ranges[i]
		if !bytes.Equal(sid, r.sid[:]) {
			continue
		}
		if strings.Compare(r.min, gnoStr) <= 0 && strings.Compare(r.max, gnoStr) >= 0 {
			return true
		}
	}
	return false
}

type gtidRange struct {
	sid uuid.UUID
	min string
	max string
}

type gtidMatcher struct {
	// all mirrors the historical behaviour: a malformed set matches everything
	all    bool
	ranges []gtidRange
}

var gtidMatcherCache = make(map[string]*gtidMatcher)

// compileGTIDSet parses a GTID set once instead of for every event.
func compileGTIDSet(gtidSet string) *gtidMatcher {
	if m, ok := gtidMatcherCache[gtidSet]; ok {
		return m
	}
	m := &gtidMatcher{}
	for _, set := range strings.Split(gtidSet, ",") {
		tmp := strings.Split(strings.TrimSpace(set), ":")
		if len(tmp) != 2 {
			m.all = true
			break
		}
		parts := strings.Split(tmp[1], "-")
		if len(parts) != 2 {
			continue
		}
		u, err := uuid.FromString(tmp[0])
		// the historical implementation compared the raw text with the
		// canonical lower-case UUID, so any other spelling never matched
		if err != nil || u.String() != tmp[0] {
			continue
		}
		m.ranges = append(m.ranges, gtidRange{sid: u, min: parts[0], max: parts[1]})
	}
	if len(gtidMatcherCache) > 8 {
		gtidMatcherCache = make(map[string]*gtidMatcher)
	}
	gtidMatcherCache[gtidSet] = m
	return m
}

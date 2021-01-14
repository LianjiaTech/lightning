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
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"sort"
	"time"

	"github.com/LianjiaTech/lightning/common"
	"github.com/LianjiaTech/lightning/rebuild"

	// database/sql
	_ "github.com/go-sql-driver/mysql"
	"github.com/juju/errors"
	"github.com/siddontang/go-mysql/mysql"
	"github.com/siddontang/go-mysql/replication"
)

// https://dev.mysql.com/doc/internals/en/binary-log-structure-and-contents.html

const (
	FileHeaderLength  = 4  // binlog file magic header 0XFE bin
	EventHeaderLength = 19 // event header length
)

// BinlogParser ...
func BinlogParser() {
	if len(common.Config.MySQL.BinlogFile) > 0 {
		// check each binlog file start time for event time filter
		err := CheckBinlogFileTime(common.Config.MySQL.BinlogFile)
		if err != nil {
			fmt.Println(err.Error())
			return
		}
		if common.Config.Rebuild.Plugin == "find" {
			fmt.Println(common.Config.MySQL.BinlogFile)
			return
		}

		// parse each binlog file
		err = BinlogFileParser(common.Config.MySQL.BinlogFile)
		if err != nil {
			fmt.Println(err.Error())
		}
		return
	}
	if common.Config.MySQL.MasterInfo != "" {
		err := BinlogStreamParser()
		if err != nil {
			fmt.Println(err.Error())
		}
	}
}

// CheckBinlogFileHeader check file is binary log
func CheckBinlogFileHeader(buf []byte) bool {
	return bytes.Equal(buf, []byte{0xfe, 'b', 'i', 'n'})
}

// CheckBinlogFormat check binlog format
func CheckBinlogFormat(dsn string) string {
	format := "unknown"
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return format
	}
	defer db.Close()
	res, err := db.Query("SELECT @@binlog_format;")
	for res.Next() {
		res.Scan(&format)
	}
	return format
}

// CheckBinlogFileTime ...
func CheckBinlogFileTime(files []string) error {
	var err error
	var filteredBinlogs []string

	// no binlog, or only one, by pass check
	if len(files) < 2 {
		return err
	}

	// if no time filter, no need to check binlog files time
	if common.Config.Filters.StartDatetime == "" &&
		common.Config.Filters.StopDatetime == "" {
		return err
	}

	// file sort by index
	sort.Strings(common.Config.MySQL.BinlogFile)

	// each file only check first event
	for idx, filename := range files {
		do := true
		fd, err := os.Open(filename)
		if err != nil {
			return err
		}

		bufFileHeader := make([]byte, FileHeaderLength)
		if _, err := io.ReadFull(fd, bufFileHeader); err != nil {
			return errors.Trace(err)
		}
		if !CheckBinlogFileHeader(bufFileHeader) {
			err = errors.Errorf("invalid file type, not binlog")
			return err
		}

		p := replication.NewBinlogParser()
		event, err := FileNextEvent(p, fd)
		if err == io.EOF {
			continue
		}
		if err != nil {
			return errors.Trace(err)
		}

		if !FilterStartDatetime(event) {
			do = false
		}
		if !FilterStopDatetime(event) {
			if len(filteredBinlogs) == 0 && idx > 0 {
				filteredBinlogs = append(filteredBinlogs, files[idx-1])
			}
			do = false
		}
		fd.Close()
		if do {
			if len(filteredBinlogs) == 0 && idx > 0 {
				filteredBinlogs = append(filteredBinlogs, files[idx-1])
			}
			filteredBinlogs = append(filteredBinlogs, filename)
		}
	}
	common.Config.MySQL.BinlogFile = filteredBinlogs
	return err
}

// BinlogFileParser parser binary log file
func BinlogFileParser(files []string) error {
	for _, filename := range files {
		fd, err := os.Open(filename)
		if err != nil {
			return err
		}
		bufFileHeader := make([]byte, FileHeaderLength)
		if _, err := io.ReadFull(fd, bufFileHeader); err != nil {
			return errors.Trace(err)
		}
		if !CheckBinlogFileHeader(bufFileHeader) {
			err = errors.Errorf("invalid file type, not binlog")
			return err
		}

		p := replication.NewBinlogParser()
		for {
			event, err := FileNextEvent(p, fd)
			if err == io.EOF {
				break
			}
			if err != nil {
				return errors.Trace(err)
			}
			if BinlogFilter(event) {
				TypeSwitcher(event)
			} else {
				common.VerboseVerbose("-- [DEBUG] BinlogFilter ignore, EventType: %s, Position: %d, ServerID: %d, TimeStamp: %d",
					event.Header.EventType.String(),
					event.Header.LogPos,
					event.Header.ServerID,
					event.Header.Timestamp,
				)
			}
			if Ending {
				break
			}
		}
		fd.Close()
	}
	return nil
}

// FileNextEvent ...
func FileNextEvent(p *replication.BinlogParser, r io.Reader) (*replication.BinlogEvent, error) {
	var err error
	var head *replication.EventHeader
	var event *replication.BinlogEvent

	bufHead := make([]byte, EventHeaderLength)
	if _, err = io.ReadFull(r, bufHead); err != nil {
		return event, err
	}
	head, err = ParseEventHeader(bufHead)
	if err != nil {
		return event, errors.Trace(err)
	}

	eventLength := head.EventSize - replication.EventHeaderSize
	bufBody := make([]byte, eventLength)
	if n, err := io.ReadFull(r, bufBody); err != nil {
		err = errors.Errorf("get event body err %v, need %d - %d, but got %d", err, head.EventSize, replication.EventHeaderSize, n)
		return event, err
	}

	var rawData []byte
	rawData = append(rawData, bufHead...)
	rawData = append(rawData, bufBody...)
	return p.Parse(rawData)
}

// BinlogStreamParser parser mysql connection replication event
func BinlogStreamParser() error {
	readTimeout, err := time.ParseDuration(common.Config.MySQL.ReadTimeout)
	if err != nil {
		common.Log.Error("BinlogStreamParser Error: %s", err.Error())
		return err
	}

	changeMaster := replication.BinlogSyncerConfig{
		ServerID:             common.MasterInfo.ServerID,
		Flavor:               common.MasterInfo.ServerType,
		Host:                 common.MasterInfo.MasterHost,
		Port:                 uint16(common.MasterInfo.MasterPort),
		User:                 common.MasterInfo.MasterUser,
		Password:             common.MasterInfo.MasterPassword,
		Charset:              common.Config.Global.Charset,
		ReadTimeout:          readTimeout,
		MaxReconnectAttempts: common.Config.MySQL.RetryCount,
		SemiSyncEnabled:      false,

		ParseTime:               false,                         // parse mysql datetime/time as string
		TimestampStringLocation: common.Config.Global.Location, // If ParseTime is false, convert TIMESTAMP into this specified timezone.
		UseDecimal:              false,                         // parse decimal type as float64
	}
	syncer := replication.NewBinlogSyncer(changeMaster)
	defer syncer.Close()
	var streamer *replication.BinlogStreamer
	if common.MasterInfo.AutoPosition {
		streamer, err = binlogDumpGTIDSyncer(syncer)
	} else {
		streamer, err = binlogDumpSyncer(syncer)
	}
	if err != nil {
		return err
	}

	for {
		event, err := getEvent(streamer, readTimeout)
		if err != nil {
			return errors.Trace(err)
		}
		if BinlogFilter(event) {
			TypeSwitcher(event)
		} else {
			common.VerboseVerbose("-- [DEBUG] BinlogFilter ignore, EventType: %s, Position: %d, ServerID: %d, TimeStamp: %d",
				event.Header.EventType.String(),
				event.Header.LogPos,
				event.Header.ServerID,
				event.Header.Timestamp,
			)
		}
		UpdateMasterInfo(event)
		if Ending {
			break
		}
	}
	return nil
}

func getEvent(streamer *replication.BinlogStreamer, readTimeout time.Duration) (*replication.BinlogEvent, error) {
	var ctx context.Context
	var cancel context.CancelFunc
	if common.Config.Global.Daemon {
		ctx = context.Background()
	} else {
		ctx, cancel = context.WithTimeout(context.Background(), readTimeout)
		defer cancel()
	}
	return streamer.GetEvent(ctx)
}

// TypeSwitcher event router by type
func TypeSwitcher(event *replication.BinlogEvent) {
	rebuild.EventHeaderRebuild(event)
	switch event.Header.EventType {
	case replication.GTID_EVENT:
		rebuild.GTIDRebuild(event.Event.(*replication.GTIDEvent))
	case replication.WRITE_ROWS_EVENTv0, replication.WRITE_ROWS_EVENTv1, replication.WRITE_ROWS_EVENTv2:
		rebuild.InsertRebuild(event)
	case replication.UPDATE_ROWS_EVENTv0, replication.UPDATE_ROWS_EVENTv1, replication.UPDATE_ROWS_EVENTv2:
		rebuild.UpdateRebuild(event)
	case replication.DELETE_ROWS_EVENTv0, replication.DELETE_ROWS_EVENTv1, replication.DELETE_ROWS_EVENTv2:
		rebuild.DeleteRebuild(event)
	case replication.QUERY_EVENT:
		rebuild.QueryRebuild(event)
	case replication.ROWS_QUERY_EVENT:
		rebuild.RowsQueryRebuild(event)
	case replication.XID_EVENT:
		rebuild.XidRebuild(event)
	case replication.ROTATE_EVENT:
		common.VerboseVerbose("-- [DEBUG] EventType: %s, NextLogName: %s", event.Header.EventType.String(), string(event.Event.(*replication.RotateEvent).NextLogName))
	// case replication.ANONYMOUS_GTID_EVENT, replication.PREVIOUS_GTIDS_EVENT, replication.TABLE_MAP_EVENT:
	default:
		common.VerboseVerbose("-- [DEBUG] TypeSwitcher EventType: %s bypass", event.Header.EventType.String())
	}
	sleepInterval(event)
}

func binlogDumpSyncer(syncer *replication.BinlogSyncer) (*replication.BinlogStreamer, error) {
	if common.MasterInfo.MasterLogFile == "" && common.Config.MySQL.ReplicateFromCurrentPosition {
		masterInfo := common.ShowMasterStatus(common.MasterInfo)
		common.MasterInfo.MasterLogFile = masterInfo.MasterLogFile
		common.MasterInfo.MasterLogPos = masterInfo.MasterLogPos
	}
	position := mysql.Position{Name: common.MasterInfo.MasterLogFile, Pos: uint32(common.MasterInfo.MasterLogPos)}
	return syncer.StartSync(position)
}

func binlogDumpGTIDSyncer(syncer *replication.BinlogSyncer) (*replication.BinlogStreamer, error) {
	gtid, err := mysql.ParseGTIDSet(common.MasterInfo.ServerType, common.MasterInfo.ExecutedGTIDSet)
	if err != nil {
		return nil, err
	}
	return syncer.StartSyncGTID(gtid)
}

// ParseEventHeader parser event header, in go-mysql it's internal func, make it public
func ParseEventHeader(buf []byte) (*replication.EventHeader, error) {
	head := new(replication.EventHeader)
	err := head.Decode(buf)
	if err != nil {
		return nil, err
	}

	if head.EventSize <= uint32(replication.EventHeaderSize) {
		err = errors.Errorf("invalid event header, event size is %d, too small", head.EventSize)
		return nil, err
	}
	return head, nil
}

// sleepInterval ...
func sleepInterval(event *replication.BinlogEvent) {
	switch common.Config.Rebuild.Plugin {
	case "sql", "flashback":
	default:
		return
	}
	interval := common.Config.Rebuild.SleepDuration.Seconds()
	if interval > 0 {
		switch event.Header.EventType {
		case replication.WRITE_ROWS_EVENTv0, replication.WRITE_ROWS_EVENTv1, replication.WRITE_ROWS_EVENTv2,
			replication.UPDATE_ROWS_EVENTv0, replication.UPDATE_ROWS_EVENTv1, replication.UPDATE_ROWS_EVENTv2,
			replication.DELETE_ROWS_EVENTv0, replication.DELETE_ROWS_EVENTv1, replication.DELETE_ROWS_EVENTv2:
			fmt.Printf("SELECT sleep(%f);\n", interval)
		case replication.QUERY_EVENT:
			switch string(event.Event.(*replication.QueryEvent).Query) {
			case "BEGIN", "COMMIT":
			default:
				fmt.Printf("SELECT sleep(%f);\n", interval)
			}
		}
	}
}

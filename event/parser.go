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
	"bufio"
	"bytes"
	"context"
	"crypto/cipher"
	"database/sql"
	"fmt"
	"io"
	"os"
	"sort"
	"time"

	"github.com/LianjiaTech/lightning/common"
	"github.com/LianjiaTech/lightning/rebuild"

	// database/sql
	"github.com/go-mysql-org/go-mysql/mysql"
	"github.com/go-mysql-org/go-mysql/replication"
	_ "github.com/go-sql-driver/mysql"
	"github.com/juju/errors"
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
			println(err.Error())
			return
		}
		switch common.Config.Rebuild.Plugin {
		case "find":
			fmt.Println(common.Config.MySQL.BinlogFile)
			return
		case "decrypt":
			for _, binlog := range common.Config.MySQL.BinlogFile {
				err = DecryptBinlog(binlog, common.Config.MySQL.Keyring)
				if err != nil {
					println(err.Error())
				}
			}
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
			println(err.Error())
		}
	}
}

// CheckBinlogFileHeader check file is binary log
func CheckBinlogFileHeader(buf []byte) bool {
	// 0xFE62696E not encrypted
	// 0xFD62696E encrypted
	return bytes.Equal(buf, []byte{0xfe, 'b', 'i', 'n'}) || bytes.Equal(buf, []byte{0xfd, 'b', 'i', 'n'})
}

// CheckBinlogFileEncrypt check file is encrypted
func CheckBinlogFileEncrypt(buf []byte) bool {
	return bytes.Equal(buf, []byte{0xfd, 'b', 'i', 'n'})
}

// CheckBinlogFormat check binlog format
func CheckBinlogFormat(dsn string) string {
	format := "unknown"
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return format
	}
	defer db.Close()
	res, err := db.Query("SELECT @@binlog_format")
	if err != nil {
		fmt.Println("CheckBinlogFormat:", err.Error())
		return format
	}
	defer res.Close()
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
		var stream cipher.Stream
		if CheckBinlogFileEncrypt(bufFileHeader) {
			stream, err = initAESCTRStream(filename, common.Config.MySQL.Keyring)
			if err != nil {
				return err
			}
			fd.Seek(EncryptFileHeaderOffset+FileHeaderLength, 0)
		} else {
			stream = nil
		}

		p := replication.NewBinlogParser()
		event, err := FileNextEvent(p, fd, stream)
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
	defer common.FlushOut()
	for _, filename := range files {
		var fd *os.File
		var err error
		switch filename {
		case "-":
			fd = os.Stdin
		default:
			fd, err = os.Open(filename)
		}
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
		var stream cipher.Stream
		if CheckBinlogFileEncrypt(bufFileHeader) {
			stream, err = initAESCTRStream(filename, common.Config.MySQL.Keyring)
			if err != nil {
				return err
			}
			fd.Seek(EncryptFileHeaderOffset, 0)
			if _, err := io.ReadFull(fd, bufFileHeader); err != nil {
				return errors.Trace(err)
			}
			if !CheckBinlogFileHeader(decryptAESCTR(stream, bufFileHeader)) {
				err = errors.Errorf("invalid file type, not binlog")
				return err
			}
		} else {
			stream = nil
		}

		p := replication.NewBinlogParser()
		p.SetUseDecimal(true) // support Decimal type
		if common.Config.Rebuild.Plugin == "stat" {
			// stat only needs row counts, not the decoded values
			p.SetRowsEventDecodeFunc(countRowsEvent)
		}

		// Binlog events are tiny (often < 200 bytes) but very numerous. Reading
		// them straight from the file costs two read(2) syscalls per event, so
		// put a large buffer in front of it.
		reader := bufio.NewReaderSize(fd, 1<<20)
		for {
			event, err := FileNextEvent(p, reader, stream)
			if err == io.EOF {
				break
			}
			if err != nil {
				return errors.Trace(err)
			}
			handleBinlogEvent(event, false)
			if Ending {
				break
			}
		}
		fd.Close()
	}
	return nil
}

// FileNextEvent ...
func FileNextEvent(p *replication.BinlogParser, r io.Reader, stream cipher.Stream) (*replication.BinlogEvent, error) {
	var err error

	// Read the 19-byte header into a stack buffer, then allocate the whole
	// event once: the header is copied in and the body is read straight into
	// the same slice. This replaces three allocations plus a full copy per
	// event with a single allocation.
	var headBuf [EventHeaderLength]byte
	bufHead := headBuf[:]
	if _, err = io.ReadFull(r, bufHead); err != nil {
		return nil, err
	}
	if stream != nil {
		stream.XORKeyStream(bufHead, bufHead)
	}

	head, err := ParseEventHeader(bufHead)
	if err != nil {
		return nil, errors.Trace(err)
	}

	rawData := make([]byte, head.EventSize)
	copy(rawData, bufHead)
	body := rawData[EventHeaderLength:]
	if n, err := io.ReadFull(r, body); err != nil {
		err = errors.Errorf("get event body err %v, need %d - %d, but got %d", err, head.EventSize, replication.EventHeaderSize, n)
		return nil, err
	}
	if stream != nil {
		// decrypt in place; CTR keeps the keystream position across calls
		stream.XORKeyStream(body, body)
	}
	return p.Parse(rawData)
}

// flushStreamOutput flushes buffered SQL after an event that completes a
// statement or transaction.
func flushStreamOutput(event *replication.BinlogEvent) {
	switch event.Header.EventType {
	case replication.XID_EVENT, replication.QUERY_EVENT, replication.ROTATE_EVENT, replication.TRANSACTION_PAYLOAD_EVENT:
		common.FlushOut()
	}
}

// BinlogStreamParser parser mysql connection replication event
func BinlogStreamParser() error {
	defer common.FlushOut()
	if common.Config.MySQL.MasterInfo != "" {
		// persist the final replication position
		defer common.FlushReplicationInfo()
	}
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
		UseDecimal:              true,                          // support Decimal type
	}
	if common.Config.Rebuild.Plugin == "stat" {
		// stat only needs row counts, not the decoded values
		changeMaster.RowsEventDecodeFunc = countRowsEvent
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
		handleBinlogEvent(event, true)
		// Generated SQL is buffered; flush at transaction boundaries so a
		// streaming consumer still sees output promptly, without paying one
		// write syscall per event.
		flushStreamOutput(event)
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

// handleBinlogEvent filters and dispatches a single binlog event.
// MySQL 8.0.20+ with binlog_transaction_compression=ON wraps a whole
// transaction into one TRANSACTION_PAYLOAD_EVENT, so expand it and dispatch
// each inner event individually. updateMaster controls whether to track the
// replication position (stream parser only).
func handleBinlogEvent(event *replication.BinlogEvent, updateMaster bool) {
	if event.Header.EventType == replication.TRANSACTION_PAYLOAD_EVENT {
		if payload, ok := event.Event.(*replication.TransactionPayloadEvent); ok {
			for _, inner := range payload.Events {
				handleBinlogEvent(inner, updateMaster)
			}
		}
		return
	}
	// DDL changes the in-memory schema globally, so keep it in sync even when
	// the DDL event itself is filtered out (e.g. by a table filter).
	// The stat plugin does not use schemas, so it can skip the SQL parsing
	// and column index rebuilds entirely.
	if event.Header.EventType == replication.QUERY_EVENT && common.Config.Rebuild.Plugin != "stat" {
		query := event.Event.(*replication.QueryEvent)
		rebuild.UpdateSchemaFromQuery(string(query.Schema), string(query.Query))
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
	if updateMaster {
		UpdateMasterInfo(event)
	}
}

// isRowEvent reports whether the event carries row images.
func isRowEvent(eventType replication.EventType) bool {
	switch eventType {
	case replication.WRITE_ROWS_EVENTv0, replication.WRITE_ROWS_EVENTv1, replication.WRITE_ROWS_EVENTv2,
		replication.UPDATE_ROWS_EVENTv0, replication.UPDATE_ROWS_EVENTv1, replication.UPDATE_ROWS_EVENTv2,
		replication.PARTIAL_UPDATE_ROWS_EVENT,
		replication.DELETE_ROWS_EVENTv0, replication.DELETE_ROWS_EVENTv1, replication.DELETE_ROWS_EVENTv2:
		return true
	}
	return false
}

// TypeSwitcher event router by type
func TypeSwitcher(event *replication.BinlogEvent) {
	rebuild.EventHeaderRebuild(event)
	// A -schema-file snapshot may not cover every table in the binlog; fetch
	// missing tables on demand (stat never uses schemas).
	if common.Config.Rebuild.Plugin != "stat" && isRowEvent(event.Header.EventType) {
		rebuild.EnsureTableSchema(rebuild.RowEventTable(event))
	}
	switch event.Header.EventType {
	case replication.GTID_EVENT:
		rebuild.GTIDRebuild(event.Event.(*replication.GTIDEvent))
	case replication.GTID_TAGGED_LOG_EVENT:
		tagged := event.Event.(*replication.GtidTaggedLogEvent)
		rebuild.GTIDRebuild(&tagged.GTIDEvent)
	case replication.WRITE_ROWS_EVENTv0, replication.WRITE_ROWS_EVENTv1, replication.WRITE_ROWS_EVENTv2:
		rebuild.InsertRebuild(event)
	case replication.UPDATE_ROWS_EVENTv0, replication.UPDATE_ROWS_EVENTv1, replication.UPDATE_ROWS_EVENTv2:
		rebuild.UpdateRebuild(event)
	case replication.PARTIAL_UPDATE_ROWS_EVENT:
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
			replication.PARTIAL_UPDATE_ROWS_EVENT,
			replication.DELETE_ROWS_EVENTv0, replication.DELETE_ROWS_EVENTv1, replication.DELETE_ROWS_EVENTv2:
			common.Outf("SELECT sleep(%f);\n", interval)
		case replication.QUERY_EVENT:
			switch string(event.Event.(*replication.QueryEvent).Query) {
			case "BEGIN", "COMMIT":
			default:
				common.Outf("SELECT sleep(%f);\n", interval)
			}
		}
	}
}

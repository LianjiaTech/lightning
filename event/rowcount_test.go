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
	"io"
	"os"
	"reflect"
	"testing"

	"github.com/LianjiaTech/lightning/common"

	"github.com/go-mysql-org/go-mysql/replication"
)

func TestCountRowsEventMatchesDecode(t *testing.T) {
	// encrypted binlogs need a keyring and are covered by the output fixtures
	for _, name := range []string{"binlog.000002", "binlog.compressed", "binlog.partialjson"} {
		path := common.DevPath + "/test/" + name
		full := collectRowCounts(t, path, false)
		fast := collectRowCounts(t, path, true)
		if !reflect.DeepEqual(full, fast) {
			t.Fatalf("%s: row counts differ, full=%v fast=%v", name, full, fast)
		}
		if sum(full) == 0 {
			t.Fatalf("%s: no rows parsed", name)
		}
	}
}

func TestCountRowsEventUsesFastPath(t *testing.T) {
	events := collectRowEvents(t, common.DevPath+"/test/binlog.000002", true)
	if len(events) == 0 {
		t.Fatal("no row events parsed")
	}
	for _, re := range events {
		// the fast decoder fills Rows but never SkippedColumns
		if len(re.Rows) > 0 && re.SkippedColumns != nil {
			t.Fatal("row event fell back to the full decoder")
		}
	}
}

func collectRowCounts(t *testing.T, path string, fast bool) []int {
	t.Helper()
	counts := make([]int, 0, 64)
	for _, re := range collectRowEvents(t, path, fast) {
		counts = append(counts, len(re.Rows))
	}
	return counts
}

func collectRowEvents(t *testing.T, path string, fast bool) []*replication.RowsEvent {
	t.Helper()
	fd, err := os.Open(path)
	if err != nil {
		t.Fatal(err.Error())
	}
	defer fd.Close()

	header := make([]byte, FileHeaderLength)
	if _, err := io.ReadFull(fd, header); err != nil {
		t.Fatal(err.Error())
	}

	p := replication.NewBinlogParser()
	p.SetUseDecimal(true)
	if fast {
		p.SetRowsEventDecodeFunc(countRowsEvent)
	}

	var events []*replication.RowsEvent
	for {
		event, err := FileNextEvent(p, fd, nil)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err.Error())
		}
		events = append(events, rowEventsOf(event)...)
	}
	return events
}

func rowEventsOf(event *replication.BinlogEvent) []*replication.RowsEvent {
	if payload, ok := event.Event.(*replication.TransactionPayloadEvent); ok {
		var events []*replication.RowsEvent
		for _, inner := range payload.Events {
			events = append(events, rowEventsOf(inner)...)
		}
		return events
	}
	if re, ok := event.Event.(*replication.RowsEvent); ok {
		return []*replication.RowsEvent{re}
	}
	return nil
}

func sum(values []int) int {
	total := 0
	for _, v := range values {
		total += v
	}
	return total
}

// BenchmarkRowEventDecode compares counting rows against the full row decoder
// for a real UPDATE event from the fixture.
func BenchmarkRowEventDecode(b *testing.B) {
	re, data := loadUpdateRowEvent(b)

	b.Run("full", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			re.Rows = nil
			re.SkippedColumns = nil
			if err := re.Decode(data); err != nil {
				b.Fatal(err.Error())
			}
		}
	})

	b.Run("count", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if err := countRowsEvent(re, data); err != nil {
				b.Fatal(err.Error())
			}
		}
	})
}

func loadUpdateRowEvent(tb testing.TB) (*replication.RowsEvent, []byte) {
	tb.Helper()
	fd, err := os.Open(common.DevPath + "/test/binlog.000002")
	if err != nil {
		tb.Fatal(err.Error())
	}
	defer fd.Close()

	header := make([]byte, FileHeaderLength)
	if _, err := io.ReadFull(fd, header); err != nil {
		tb.Fatal(err.Error())
	}
	p := replication.NewBinlogParser()
	p.SetUseDecimal(true)
	for {
		event, err := FileNextEvent(p, fd, nil)
		if err == io.EOF {
			break
		}
		if err != nil {
			tb.Fatal(err.Error())
		}
		re, ok := event.Event.(*replication.RowsEvent)
		if !ok || len(re.Rows) < 2 {
			continue
		}
		// the fixture uses CRC32 checksums, which Decode does not expect
		body := event.RawData[EventHeaderLength : len(event.RawData)-4]
		if _, err := countRows(re, body); err != nil {
			continue
		}
		return re, body
	}
	tb.Fatal("no update row event found")
	return nil, nil
}

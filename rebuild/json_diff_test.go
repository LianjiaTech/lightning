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

package rebuild

import (
	"testing"

	"github.com/go-mysql-org/go-mysql/replication"
)

func TestParseJSONPath(t *testing.T) {
	cases := []struct {
		path  string
		steps []jsonPathStep
	}{
		{"$", nil},
		{"$.a", []jsonPathStep{{key: "a"}}},
		{"$.a.b", []jsonPathStep{{key: "a"}, {key: "b"}}},
		{"$.b.c[1]", []jsonPathStep{{key: "b"}, {key: "c"}, {index: 1, isIdx: true}}},
		{"$[0]", []jsonPathStep{{index: 0, isIdx: true}}},
		{`$."a b"`, []jsonPathStep{{key: "a b"}}},
	}
	for _, c := range cases {
		got, err := parseJSONPath(c.path)
		if err != nil {
			t.Errorf("parseJSONPath(%q) error: %v", c.path, err)
			continue
		}
		if len(got) != len(c.steps) {
			t.Errorf("parseJSONPath(%q) = %v, want %v", c.path, got, c.steps)
			continue
		}
		for i := range got {
			if got[i] != c.steps[i] {
				t.Errorf("parseJSONPath(%q)[%d] = %v, want %v", c.path, i, got[i], c.steps[i])
			}
		}
	}
}

func TestApplyJSONDiff(t *testing.T) {
	before := `{"a":1,"b":{"c":[1,2,3],"d":"x"}}`
	cases := []struct {
		op    replication.JsonDiffOperation
		path  string
		value string
		want  string
	}{
		{replication.JsonDiffOperationReplace, "$.b.c[1]", "99", `{"a":1,"b":{"c":[1,99,3],"d":"x"}}`},
		{replication.JsonDiffOperationRemove, "$.b.d", "", `{"a":1,"b":{"c":[1,2,3]}}`},
		{replication.JsonDiffOperationInsert, "$.e", `"new"`, `{"a":1,"b":{"c":[1,2,3],"d":"x"},"e":"new"}`},
		{replication.JsonDiffOperationReplace, "$.b.c[1]", "99", `{"a":1,"b":{"c":[1,99,3],"d":"x"}}`},
	}
	for _, c := range cases {
		diff := &replication.JsonDiff{Op: c.op, Path: c.path, Value: c.value}
		got, err := applyJSONDiff(before, diff)
		if err != nil {
			t.Errorf("applyJSONDiff(%s %s) error: %v", c.op, c.path, err)
			continue
		}
		if got != c.want {
			t.Errorf("applyJSONDiff(%s %s) = %s, want %s", c.op, c.path, got, c.want)
		}
	}
}

func TestMergePartialJSON(t *testing.T) {
	rows := [][]any{
		{int32(1), `{"a":1,"b":{"c":[1,2,3],"d":"x"}}`},                                                            // before
		{int32(1), &replication.JsonDiff{Op: replication.JsonDiffOperationReplace, Path: "$.b.c[1]", Value: "99"}}, // after
	}
	mergePartialJSON(rows)
	got, ok := rows[1][1].(string)
	if !ok {
		t.Fatalf("merged value type = %T, want string", rows[1][1])
	}
	want := `{"a":1,"b":{"c":[1,99,3],"d":"x"}}`
	if got != want {
		t.Errorf("merged = %s, want %s", got, want)
	}
}

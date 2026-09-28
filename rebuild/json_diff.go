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
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/LianjiaTech/lightning/common"

	"github.com/go-mysql-org/go-mysql/replication"
)

// jsonPathStep is a single segment of a MySQL JSON path.
type jsonPathStep struct {
	key   string
	index int
	isIdx bool
}

// parseJSONPath parses a MySQL JSON path like `$.a.b[0]` or `$."a b"[1]`.
func parseJSONPath(path string) ([]jsonPathStep, error) {
	if !strings.HasPrefix(path, "$") {
		return nil, fmt.Errorf("invalid JSON path: %q", path)
	}
	var steps []jsonPathStep
	i, n := 1, len(path)
	for i < n {
		switch path[i] {
		case '.':
			i++
			if i < n && path[i] == '"' {
				i++
				var sb strings.Builder
				for i < n && path[i] != '"' {
					if path[i] == '\\' && i+1 < n {
						i++
					}
					sb.WriteByte(path[i])
					i++
				}
				i++ // skip closing quote
				steps = append(steps, jsonPathStep{key: sb.String()})
				continue
			}
			start := i
			for i < n && path[i] != '.' && path[i] != '[' {
				i++
			}
			steps = append(steps, jsonPathStep{key: path[start:i]})
		case '[':
			i++
			start := i
			for i < n && path[i] != ']' {
				i++
			}
			idx, err := strconv.Atoi(path[start:i])
			if err != nil {
				return nil, fmt.Errorf("invalid array index in JSON path: %q", path)
			}
			i++ // skip ']'
			steps = append(steps, jsonPathStep{index: idx, isIdx: true})
		default:
			return nil, fmt.Errorf("invalid JSON path: %q", path)
		}
	}
	return steps, nil
}

// applyJSONDiff applies a single partial-JSON diff to the full "before" JSON
// value and returns the resulting JSON text.
func applyJSONDiff(before any, diff *replication.JsonDiff) (string, error) {
	var raw []byte
	switch v := before.(type) {
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		return "", fmt.Errorf("unexpected JSON before value type %T", before)
	}

	var doc any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&doc); err != nil {
		return "", err
	}

	steps, err := parseJSONPath(diff.Path)
	if err != nil {
		return "", err
	}

	switch diff.Op {
	case replication.JsonDiffOperationRemove:
		doc, err = jsonApply(doc, steps, nil, diff.Op)
	case replication.JsonDiffOperationReplace, replication.JsonDiffOperationInsert:
		var val any
		dec := json.NewDecoder(strings.NewReader(diff.Value))
		dec.UseNumber()
		if err := dec.Decode(&val); err != nil {
			return "", err
		}
		doc, err = jsonApply(doc, steps, val, diff.Op)
	default:
		return "", fmt.Errorf("unknown JSON diff op: %v", diff.Op)
	}
	if err != nil {
		return "", err
	}

	out, err := json.Marshal(doc)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// jsonApply sets, inserts or removes a value at the given JSON path.
func jsonApply(doc any, steps []jsonPathStep, val any, op replication.JsonDiffOperation) (any, error) {
	if len(steps) == 0 {
		return val, nil
	}
	step := steps[0]
	last := len(steps) == 1

	if step.isIdx {
		arr, ok := doc.([]any)
		if !ok {
			return nil, fmt.Errorf("JSON path expects array")
		}
		if step.index < 0 || step.index > len(arr) {
			return nil, fmt.Errorf("JSON array index out of range: %d", step.index)
		}
		if last {
			switch op {
			case replication.JsonDiffOperationRemove:
				if step.index == len(arr) {
					return nil, fmt.Errorf("JSON array index out of range: %d", step.index)
				}
				return append(arr[:step.index], arr[step.index+1:]...), nil
			case replication.JsonDiffOperationInsert:
				return append(arr[:step.index], append([]any{val}, arr[step.index:]...)...), nil
			default: // Replace
				if step.index == len(arr) {
					return nil, fmt.Errorf("JSON array index out of range: %d", step.index)
				}
				arr[step.index] = val
				return arr, nil
			}
		}
		if step.index == len(arr) {
			return nil, fmt.Errorf("JSON array index out of range: %d", step.index)
		}
		node, err := jsonApply(arr[step.index], steps[1:], val, op)
		if err != nil {
			return nil, err
		}
		arr[step.index] = node
		return arr, nil
	}

	obj, ok := doc.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("JSON path expects object")
	}
	if last {
		if op == replication.JsonDiffOperationRemove {
			delete(obj, step.key)
		} else {
			obj[step.key] = val
		}
		return obj, nil
	}
	child, exists := obj[step.key]
	if !exists {
		return nil, fmt.Errorf("JSON path key not found: %s", step.key)
	}
	node, err := jsonApply(child, steps[1:], val, op)
	if err != nil {
		return nil, err
	}
	obj[step.key] = node
	return obj, nil
}

// mergePartialJSON merges PARTIAL_JSON diffs into their full "before" value.
// With binlog_row_value_options=PARTIAL_JSON (MySQL 8.0.20+), the after image
// of an UPDATE may carry a *replication.JsonDiff for a JSON column instead of
// the full document. Rows are stored as (before, after) pairs, so the before
// image (available with binlog_row_image=FULL) provides the base document.
func mergePartialJSON(rows [][]any) {
	for i := 1; i < len(rows); i += 2 {
		before := rows[i-1]
		after := rows[i]
		for j, v := range after {
			diff, ok := v.(*replication.JsonDiff)
			if !ok || j >= len(before) {
				continue
			}
			merged, err := applyJSONDiff(before[j], diff)
			if err != nil {
				common.VerboseVerbose("-- [DEBUG] merge partial JSON failed, path: %s, op: %s, error: %s",
					diff.Path, diff.Op.String(), err.Error())
				continue
			}
			after[j] = merged
		}
	}
}

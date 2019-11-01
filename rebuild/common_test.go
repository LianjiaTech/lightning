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
	"fmt"
	"strconv"
	"testing"
)

func TestLastStatus(t *testing.T) {
	LastStatus()
}

func TestPrintBinlogStat(t *testing.T) {
	printBinlogStat()
}

func TestLoadLuaScript(t *testing.T) {
	LoadLuaScript()
}

func TestStrconvQuote(t *testing.T) {
	fmt.Println(strconv.Quote(`'"space `))
	fmt.Printf(`"%s"`, escape(`'"space `))
}

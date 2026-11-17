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

package common

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"runtime"
	"strings"

	"github.com/astaxie/beego/logs"
	disablelog "github.com/siddontang/go-log/log"
)

// Log 使用 beego 的 log 库
var Log *logs.BeeLogger

// output is the buffered writer used for generated SQL. Writing each
// statement straight to os.Stdout costs one write(2) syscall per row, which
// dominates the runtime when generating SQL for large binlogs.
var (
	outFile *os.File
	outBuf  *bufio.Writer
)

const outBufferSize = 1 << 20

// Out returns the buffered writer for generated SQL. It is rebound when
// os.Stdout changes (tests swap os.Stdout to capture output).
func Out() *bufio.Writer {
	if outBuf == nil || outFile != os.Stdout {
		outFile = os.Stdout
		outBuf = bufio.NewWriterSize(outFile, outBufferSize)
	}
	return outBuf
}

// Outf writes formatted output to the buffered stdout writer.
func Outf(format string, a ...interface{}) {
	_, _ = fmt.Fprintf(Out(), format, a...)
}

// Outln writes a line to the buffered stdout writer.
func Outln(a ...interface{}) {
	_, _ = fmt.Fprintln(Out(), a...)
}

// FlushOut flushes pending generated SQL output.
func FlushOut() {
	if outBuf != nil && outFile == os.Stdout {
		_ = outBuf.Flush()
	}
}

func init() {
	Log = logs.NewLogger(0)
	Log.EnableFuncCallDepth(true)
	disablelog.SetLevel(disablelog.LevelFatal)
}

// loggerInit Log配置初始化
func loggerInit() {
	Log.SetLevel(Config.Global.LogLevel)
	func() { _ = Log.DelLogger(logs.AdapterFile) }()
	logConfig := map[string]interface{}{
		"filename": Config.Global.LogOutput,
		"level":    7,
		"maxlines": 0,
		"maxsize":  0,
		"daily":    false,
		"maxdays":  0,
	}
	logConfigJSON, _ := json.Marshal(logConfig)
	err := Log.SetLogger(logs.AdapterFile, string(logConfigJSON))
	if err != nil {
		fmt.Println(err.Error())
	}
}

// Caller returns the caller of the function that called it :)
// https://stackoverflow.com/questions/35212985/is-it-possible-get-information-about-caller-function-in-golang
func Caller() string {
	// we get the callers as uintptrs - but we just need 1
	fpcs := make([]uintptr, 1)

	// skip 3 levels to get to the caller of whoever called Caller()
	n := runtime.Callers(3, fpcs)
	if n == 0 {
		return "n/a" // proper error her would be better
	}

	// get the info of the actual function that's in the pointer
	fun := runtime.FuncForPC(fpcs[0] - 1)
	if fun == nil {
		return "n/a"
	}

	// return its name
	return fun.Name()
}

// GetFunctionName 获取调当前函数名
func GetFunctionName() string {
	// Skip this function, and fetch the PC and file for its parent
	pc, _, _, _ := runtime.Caller(1)
	// Retrieve a Function object this functions parent
	functionObject := runtime.FuncForPC(pc)
	// Regex to extract just the function name (and not the module path)
	extractFnName := regexp.MustCompile(`^.*\.(.*)$`)
	fnName := extractFnName.ReplaceAllString(functionObject.Name(), "$1")
	return fnName
}

// fileName get filename from path
func fileName(original string) string {
	i := strings.LastIndex(original, "/")
	if i == -1 {
		return original
	}
	return original[i+1:]
}

// LogIfError 简化if err != nil 打 Error 日志代码长度
func LogIfError(err error, format string, v ...interface{}) {
	if err != nil {
		_, fn, line, _ := runtime.Caller(1)
		if format == "" {
			format = "[%s:%d] %s"
			Log.Error(format, fileName(fn), line, err.Error())
		} else {
			format = "[%s:%d] " + format + " Error: %s"
			Log.Error(format, fileName(fn), line, v, err.Error())
		}
	}
}

// LogIfWarn 简化if err != nil 打 Warn 日志代码长度
func LogIfWarn(err error, format string, v ...interface{}) {
	if err != nil {
		_, fn, line, _ := runtime.Caller(1)
		if format == "" {
			format = "[%s:%d] %s"
			Log.Warn(format, fileName(fn), line, err.Error())
		} else {
			format = "[%s:%d] " + format + " Error: %s"
			Log.Warn(format, fileName(fn), line, v, err.Error())
		}
	}
}

// Verbose ...
func Verbose(format string, a ...interface{}) {
	if Config.Global.Verbose || Config.Global.VerboseVerbose {
		if !strings.HasSuffix(format, "\n") {
			format += "\n"
		}
		fmt.Printf(format, a...)
	}
}

// VerboseVerbose ...
func VerboseVerbose(format string, a ...interface{}) {
	if Config.Global.VerboseVerbose {
		if !strings.HasSuffix(format, "\n") {
			format += "\n"
		}
		fmt.Printf(format, a...)
	}
}

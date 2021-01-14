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
	"database/sql"
	"flag"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	// database/sql
	_ "github.com/go-sql-driver/mysql"
	"github.com/juju/errors"
	pingcap "github.com/pingcap/parser/mysql"
	yaml "gopkg.in/yaml.v2"
)

// GlobalConfig global config
type GlobalConfig struct {
	// 日志级别，这里使用了 beego 的 log 包
	// [0:Emergency, 1:Alert, 2:Critical, 3:Error, 4:Warning, 5:Notice, 6:Informational, 7:Debug]
	LogLevel int `yaml:"log-level"`
	// 日志输出位置，默认日志输出到控制台
	// 目前只支持['console', 'file']两种形式，如非console形式这里需要指定文件的路径，可以是相对路径
	LogOutput      string         `yaml:"log-output"`
	Daemon         bool           `yaml:"daemon"`
	Charset        string         `yaml:"charset"`
	HexString      bool           `yaml:"hex-string"`      // string, varchar 等数据是否使用 hex 转义，防止数据转换
	CPU            int            `yaml:"cpu"`             // CPU core limit
	Verbose        bool           `yaml:"verbose"`         // more info to print
	VerboseVerbose bool           `yaml:"verbose-verbose"` // more and more info to print
	TimeZone       string         `yaml:"time-zone"`       // "UTC", "Asia/Shanghai"
	Location       *time.Location `yaml:"-"`
}

var gConfig = GlobalConfig{
	LogLevel:  3,
	LogOutput: "lightning.log",
	TimeZone:  "Asia/Shanghai",
	Charset:   "utf8mb4", // MySQL 低版本不支持 utf8mb4, 可能会有报错需要通过修改配置文件避免
}

// MySQL binlog file location or streamer, if streamer use dsn format
type MySQL struct {
	BinlogFile                   []string      `yaml:"binlog-file"`
	SchemaFile                   string        `yaml:"schema-file"`
	MasterInfo                   string        `yaml:"master-info"`
	ReplicateFromCurrentPosition bool          `yaml:"replicate-from-current-position"`
	SyncInterval                 string        `yaml:"sync-interval"`
	SyncDuration                 time.Duration `yaml:"-"`
	ReadTimeout                  string        `yaml:"read-timeout"`
	RetryCount                   int           `yaml:"retry-count"`
}

var mConfig = MySQL{
	BinlogFile:   []string{},
	SchemaFile:   "",
	SyncInterval: "1s",
	ReadTimeout:  "3s",
	RetryCount:   100,
}

// Filters filters about event
type Filters struct {
	Tables         []string `yaml:"tables"`        // replication_wild_do_tables format
	IgnoreTables   []string `yaml:"ignore-tables"` // replicate_wild_ignore_tables format
	EventType      []string `yaml:"event-types"`   // insert, update, delete
	ThreadID       int      `yaml:"thread-id"`
	ServerID       int      `yaml:"server-id"`
	StartPosition  int64    `yaml:"start-position"`
	StopPosition   int64    `yaml:"stop-position"`
	StartDatetime  string   `yaml:"start-datetime"`
	StopDatetime   string   `yaml:"stop-datetime"`
	IncludeGTIDSet string   `yaml:"include-gtid-set"`
	ExcludeGTIDSet string   `yaml:"exclude-gtid-set"`
	StartTimestamp int64    `yaml:"-"`
	StopTimestamp  int64    `yaml:"-"`
}

var fConfig = Filters{
	Tables: []string{},
	IgnoreTables: []string{
		"mysql.%",
		"percona.%",
	},
	StartDatetime: "",
	StopDatetime:  "",
}

// Rebuild rebuild plugins
type Rebuild struct {
	Plugin              string        `yaml:"plugin"` // Plugin name: sql, flashback, stat, lua, find
	CompleteInsert      bool          `yaml:"complete-insert"`
	ExtendedInsertCount int           `yaml:"extended-insert-count"`
	IgnoreColumns       []string      `yaml:"ignore-columns"`
	Replace             bool          `yaml:"replace"`
	SleepInterval       string        `yaml:"sleep-interval"`
	SleepDuration       time.Duration `yaml:"-"`
	LuaScript           string        `yaml:"lua-script"`
	WithoutDBName       bool          `yaml:"without-db-name"`
}

var rConfig = Rebuild{
	Plugin:        "sql",
	SleepInterval: "0s",
	WithoutDBName: false,
}

// Configuration config sections
type Configuration struct {
	Global  GlobalConfig `yaml:"global"`
	MySQL   MySQL        `yaml:"mysql"`
	Filters Filters      `yaml:"filters"`
	Rebuild Rebuild      `yaml:"rebuild"`
}

// Config global config variable
var Config = Configuration{
	gConfig,
	mConfig,
	fConfig,
	rConfig,
}

// ChangeMaster change master info
type ChangeMaster struct {
	MasterHost      string `yaml:"master_host"`
	MasterUser      string `yaml:"master_user"`
	MasterPassword  string `yaml:"master_password"`
	MasterPort      int    `yaml:"master_port"`
	MasterLogFile   string `yaml:"master_log_file"`
	MasterLogPos    int64  `yaml:"master_log_pos"`
	ExecutedGTIDSet string `yaml:"executed_gtid_set"`
	AutoPosition    bool   `yaml:"auto_position"`

	SecondsBehindMaster int64  `yaml:"seconds_behind_master"` // last execute event timestamp
	ServerID            uint32 `yaml:"server-id"`
	ServerType          string `yaml:"server-type"` // mysql, mariadb
}

// MasterInfo replication status info
var MasterInfo = ChangeMaster{
	MasterPort: 3306,
	ServerID:   11,
	ServerType: "mysql",
}

// ShowMasterStatus execute `show master status`, get master info
func ShowMasterStatus(masterInfo ChangeMaster) ChangeMaster {
	db, err := sql.Open("mysql",
		fmt.Sprintf(`%s:%s@tcp(%s:%d)/`,
			masterInfo.MasterUser,
			masterInfo.MasterPassword,
			masterInfo.MasterHost,
			masterInfo.MasterPort,
		))
	if err != nil {
		Log.Error(err.Error())
		return masterInfo
	}
	defer db.Close()

	rows, err := db.Query("show master status")
	if err != nil {
		Log.Error(err.Error())
		return masterInfo
	}

	columns, err := rows.Columns()
	if err != nil {
		Log.Error(err.Error())
		return masterInfo
	}
	values := make([]sql.RawBytes, len(columns))
	scanArgs := make([]interface{}, len(values))
	for i := range values {
		scanArgs[i] = &values[i]
	}
	for rows.Next() {
		err = rows.Scan(scanArgs...)
		if err != nil {
			Log.Error(err.Error())
			break
		}
		for i, v := range values {
			switch columns[i] {
			case "File":
				masterInfo.MasterLogFile = string(v)
			case "Position":
				masterInfo.MasterLogPos, _ = strconv.ParseInt(string(v), 10, 64)
			case "Binlog_Do_DB":
			case "Binlog_Ignore_DB":
			case "Executed_Gtid_Set":
				masterInfo.ExecutedGTIDSet = string(v)
			}
		}
	}
	return masterInfo
}

// ParseConfig parse configuration
func ParseConfig() {
	var err error

	// Not in config flags
	noDefaults := flag.Bool("no-defaults", false, "don't load config from default file")
	configFile := flag.String("config", "", "load config from specify file")
	printConfig := flag.Bool("print-config", false, "print config into stdout")
	printMasterInfo := flag.Bool("print-master-info", false, "print master.info into stdout")
	checkConfig := flag.Bool("check-config", false, "check config file format")
	printVersion := flag.Bool("version", false, "print version info into stdout")
	listPlugin := flag.Bool("list-plugin", false, "list support plugins")

	// Global section config
	globalLogLevel := flag.Int("log-level", 0, "log level")
	globalLogOutput := flag.String("log-output", "", "log output file name")
	globalTimeZone := flag.String("time-zone", "", "time zone info")
	globalCharset := flag.String("charset", "", "charset use for binlog parsing")
	globalCPU := flag.Int("cpu", 0, "cpu cores limit")
	globalVerbose := flag.Bool("verbose", false, "verbose mode, more info will print")
	globalVerboseVerbose := flag.Bool("vv", false, "verbose verbose mode, more and more info will print")
	globalDaemon := flag.Bool("daemon", false, "replication run as daemon")
	globalHexString := flag.Bool("hex-string", false, "convert string to hex format")

	// MySQL section config
	mysqlUser := flag.String("user", "", "mysql user")
	mysqlHost := flag.String("host", "", "mysql host")
	mysqlPort := flag.Int("port", 0, "mysql port")
	mysqlPassword := flag.String("password", "", "mysql password")
	mysqlBinlogFile := flag.String("binlog-file", "", "binlog files separate with space, eg. --binlog-file='binlog.000001 binlog.000002'")
	mysqlSchemaFile := flag.String("schema-file", "", "schema load from file")
	mysqlMasterInfo := flag.String("master-info", "", "master.info file")
	mysqlReplicateFromCurrent := flag.Bool("replicate-from-current-position", false, "binlog dump from current `show master status`")
	mysqlSyncInterval := flag.String("sync-interval", "", "sync master.info interval")
	mysqlReadTimeout := flag.String("read-timeout", "", "I/O read timeout. The value must be a decimal number with a unit suffix ('ms', 's', 'm', 'h'), such as '30s', '0.5m' or '1m30s'.")
	mysqlRetryCount := flag.Int("retry-count", 0, "maximum number of attempts to re-establish a broken connection")

	// Filters section config
	filterThreadID := flag.Int("thread-id", 0, "binlog filter thread-id")
	filterServerID := flag.Int("server-id", 0, "binlog filter server-id")
	filterIncludeGTID := flag.String("include-gtids", "", "like mysqlbinlog include-gtids")
	filterExcludeGTID := flag.String("exclude-gtids", "", "like mysqlbinlog exclude-gtids")
	filterStartPosition := flag.Int64("start-position", 0, "binlog start-position")
	filterStopPosition := flag.Int64("stop-position", 0, "binlog stop-position")
	filterStartDatetime := flag.String("start-datetime", "", "binlog filter start-datetime")
	filterStopDatetime := flag.String("stop-datetime", "", "binlog filter stop-datetime")
	filterTables := flag.String("tables", "", "binlog filter tables. eg. -tables db1.tb1,db1.tb2,db2.%")
	filterIgnoreTables := flag.String("ignore-tables", "", "binlog filter ignore tables")
	filterEventTypes := flag.String("event-types", "", "binlog filter event types")

	// Rebuild section config
	rebuildPlugin := flag.String("plugin", "", "plugin name")
	rebuildCompleteInsert := flag.Bool("complete-insert", false, "complete column info, like 'INSERT INTO tb (col) VALUES (1)'")
	rebuildExtendedInsertCount := flag.Int("extended-insert-count", 0, "use multiple-row INSERT syntax that include several VALUES")
	rebuildReplace := flag.Bool("replace", false, "use REPLACE INTO instead of INSERT INTO, UPDATE")
	rebuildSleepInterval := flag.String("sleep-interval", "", "execute commands repeatedly with a sleep between")
	rebuildIgnoreColumns := flag.String("ignore-columns", "", "query rebuild ignore columns")
	rebuildLuaScript := flag.String("lua-script", "", "lua plugin script file")
	rebuildWithoutDBName := flag.Bool("without-db-name", false, "insert/delete/update query without database name, only table name")

	// master.info config
	masterHost := flag.String("master-host", "", "master.info master_host")
	masterUser := flag.String("master-user", "", "master.info master_user")
	masterPassword := flag.String("master-password", "", "master.info master_password")
	masterPort := flag.Int("master-port", 0, "master.info master_port")
	masterLogFile := flag.String("master-log-file", "", "master.info master_log_file")
	masterLogPos := flag.Int64("master-log-pos", 0, "master.info master_log_pos")
	executedGtidSet := flag.String("executed-gtid-set", "", "master.info executed_gtid_set")
	autoPosition := flag.Bool("auto-position", false, "master.info auto_position")
	serverId := flag.Uint("slave-server-id", 0, "master.info server-id")
	serverType := flag.String("server-type", "", "master.info server-type")

	flag.CommandLine.SetOutput(os.Stdout)
	flag.Parse()

	// Not in config flags
	if !*noDefaults {
		err = Config.parseConfigFile(*configFile)
	}
	if *configFile != "" {
		err = Config.parseConfigFile(*configFile)
	}
	if *checkConfig {
		if err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		} else {
			fmt.Println("OK")
			os.Exit(0)
		}
	}
	if *printVersion {
		version()
		os.Exit(0)
	}
	if *listPlugin {
		ListPlugin()
		os.Exit(0)
	}

	// Global config
	if *globalLogLevel > 0 {
		Config.Global.LogLevel = *globalLogLevel
	}
	if *globalLogOutput != "" {
		Config.Global.LogOutput = *globalLogOutput
	}
	if *globalTimeZone != "" {
		Config.Global.TimeZone = *globalTimeZone
	}
	Config.Global.Location, err = time.LoadLocation(Config.Global.TimeZone)
	if err != nil {
		Log.Error(errors.Trace(err).Error())
		Config.Global.Location = time.Now().Location()
	}
	if *globalCharset != "" {
		Config.Global.Charset = *globalCharset
	}
	if ok := pingcap.Charsets[Config.Global.Charset]; ok == "" {
		Log.Warn("Config.Global.Charset: %s not exist", Config.Global.Charset)
		Config.Global.Charset = "utf8mb4"
	}
	if *globalCPU > 0 {
		Config.Global.CPU = *globalCPU
		runtime.GOMAXPROCS(*globalCPU)
	}
	if *globalVerbose {
		Config.Global.Verbose = *globalVerbose
	}
	if *globalVerboseVerbose {
		Config.Global.VerboseVerbose = *globalVerboseVerbose
	}
	if *globalDaemon {
		Config.Global.Daemon = *globalDaemon
	}
	if *globalHexString {
		Config.Global.HexString = *globalHexString
	}

	// MySQL config
	if *mysqlBinlogFile != "" {
		Config.MySQL.BinlogFile = strings.Fields(*mysqlBinlogFile)
	} else {
		// Only parse first not flags file
		files := flag.Args()
		if len(files) >= 1 {
			Config.MySQL.BinlogFile = files
		}
	}
	if *mysqlSchemaFile != "" {
		Config.MySQL.SchemaFile = *mysqlSchemaFile
	}
	if *mysqlMasterInfo != "" {
		Config.MySQL.MasterInfo = *mysqlMasterInfo
	}
	if *mysqlReplicateFromCurrent {
		Config.MySQL.ReplicateFromCurrentPosition = *mysqlReplicateFromCurrent
	}
	if *mysqlSyncInterval != "" {
		_, err = time.ParseDuration(*mysqlSyncInterval)
		if err != nil {
			Log.Warn("-sync-interval '%s' Error: %s", *mysqlSyncInterval, err.Error())
		} else {
			Config.MySQL.SyncInterval = *mysqlSyncInterval
		}
	}
	Config.MySQL.SyncDuration, err = time.ParseDuration(Config.MySQL.SyncInterval)
	if err != nil {
		Log.Warn("sync-interval '%s' Error: %s", Config.MySQL.SyncInterval, err.Error())
		Config.MySQL.SyncDuration = time.Duration(0 * time.Second)
	}
	if *mysqlReadTimeout != "" {
		_, err = time.ParseDuration(*mysqlReadTimeout)
		if err != nil {
			Log.Warn("-read-timeout '%s' Error: %s", *mysqlReadTimeout, err.Error())
		} else {
			Config.MySQL.ReadTimeout = *mysqlReadTimeout
		}
	}
	if *mysqlRetryCount > 0 {
		Config.MySQL.RetryCount = *mysqlRetryCount
	}

	// Filters Config
	if *filterThreadID > 0 {
		Config.Filters.ThreadID = *filterThreadID
	}
	if *filterServerID > 0 {
		Config.Filters.ServerID = *filterServerID
	}
	if *filterIncludeGTID != "" {
		Config.Filters.IncludeGTIDSet = *filterIncludeGTID
	}
	if *filterExcludeGTID != "" {
		Config.Filters.ExcludeGTIDSet = *filterExcludeGTID
	}
	if *filterStartDatetime != "" {
		Config.Filters.StartDatetime = *filterStartDatetime
	}
	layout := "2006-01-02 15:04:05"
	if Config.Filters.StartDatetime != "" {
		t, err := time.ParseInLocation(layout, Config.Filters.StartDatetime, Config.Global.Location)
		if err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		} else {
			Config.Filters.StartTimestamp = t.Unix()
		}
	}
	VerboseVerbose("-- [DEBUG] Config.Filters.StartTimestamp: %d", Config.Filters.StartTimestamp)
	if *filterStopDatetime != "" {
		Config.Filters.StopDatetime = *filterStopDatetime
	}
	if Config.Filters.StopDatetime == "" && Config.Global.Daemon == false {
		Config.Filters.StopDatetime = time.Now().Format(layout)
	}

	if Config.Filters.StopDatetime != "" {
		t, err := time.ParseInLocation(layout, Config.Filters.StopDatetime, Config.Global.Location)
		if err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		} else {
			Config.Filters.StopTimestamp = t.Unix()
		}
	}
	VerboseVerbose("-- [DEBUG] Config.Filters.StopTimestamp: %d", Config.Filters.StopTimestamp)
	if *filterStartPosition > 0 {
		Config.Filters.StartPosition = *filterStartPosition
	}
	if *filterStopPosition > 0 {
		Config.Filters.StopPosition = *filterStopPosition
	}
	if *filterTables != "" {
		Config.Filters.Tables = strings.Split(*filterTables, ",")
	}
	for _, t := range Config.Filters.Tables {
		if !strings.Contains(t, ".") {
			fmt.Println("filter -tables format should be `db`.`tb`")
			os.Exit(1)
		}
	}
	if *filterIgnoreTables != "" {
		Config.Filters.IgnoreTables = strings.Split(*filterIgnoreTables, ",")
	}
	for _, t := range Config.Filters.IgnoreTables {
		if !strings.Contains(t, ".") {
			fmt.Println("filter -ignore-tables format should be `db`.`tb`")
			os.Exit(1)
		}
	}
	if *filterEventTypes != "" {
		Config.Filters.EventType = strings.Split(*filterEventTypes, ",")
	}

	// Rebuild config
	if *rebuildPlugin != "" {
		Config.Rebuild.Plugin = *rebuildPlugin
	}
	switch Config.Rebuild.Plugin {
	case "":
		Config.Rebuild.Plugin = "sql"
	case "lua", "sql", "flashback", "stat", "find":
	default:
		ListPlugin()
		os.Exit(1)
	}
	if *rebuildCompleteInsert {
		Config.Rebuild.CompleteInsert = *rebuildCompleteInsert
	}
	if *rebuildExtendedInsertCount > 0 {
		Config.Rebuild.ExtendedInsertCount = *rebuildExtendedInsertCount
	}
	if *rebuildReplace {
		Config.Rebuild.Replace = *rebuildReplace
	}
	if *rebuildSleepInterval != "" {
		_, err = time.ParseDuration(*rebuildSleepInterval)
		if err != nil {
			Log.Warn("-sleep-interval '%s' Error: %s", *rebuildSleepInterval, err.Error())
		} else {
			Config.Rebuild.SleepInterval = *rebuildSleepInterval
		}
	}
	Config.Rebuild.SleepDuration, err = time.ParseDuration(Config.Rebuild.SleepInterval)
	if err != nil {
		Log.Warn("sleep-interval '%s' Error: %s", Config.Rebuild.SleepInterval, err.Error())
		Config.Rebuild.SleepDuration = time.Duration(0 * time.Second)
	}
	if *rebuildIgnoreColumns != "" {
		Config.Rebuild.IgnoreColumns = strings.Split(*rebuildIgnoreColumns, ",")
	}
	if len(Config.Rebuild.IgnoreColumns) > 0 {
		Config.Rebuild.CompleteInsert = true
	}
	if *rebuildLuaScript != "" {
		Config.Rebuild.LuaScript = *rebuildLuaScript
	}
	if *rebuildWithoutDBName {
		Config.Rebuild.WithoutDBName = *rebuildWithoutDBName
	}

	LoadMasterInfo()

	if len(Config.MySQL.BinlogFile) == 0 && Config.MySQL.MasterInfo == "" {
		Config.MySQL.MasterInfo = "master.info"
	}

	if *printConfig {
		PrintConfiguration()
		os.Exit(0)
	}

	// master.info config
	if *mysqlUser != "" {
		MasterInfo.MasterUser = *mysqlUser
	}
	if *mysqlHost != "" {
		MasterInfo.MasterHost = *mysqlHost
	}
	if *mysqlPassword != "" {
		MasterInfo.MasterPassword = *mysqlPassword
	}
	if *mysqlPort != 0 {
		MasterInfo.MasterPort = *mysqlPort
	}
	if *masterHost != "" {
		MasterInfo.MasterHost = *masterHost
	}
	if *masterUser != "" {
		MasterInfo.MasterUser = *masterUser
	}
	if *masterPassword != "" {
		MasterInfo.MasterPassword = *masterPassword
	}
	if *masterPort != 0 {
		MasterInfo.MasterPort = *masterPort
	}
	if *masterLogFile != "" {
		MasterInfo.MasterLogFile = *masterLogFile
	}
	if *masterLogPos != 0 {
		MasterInfo.MasterLogPos = *masterLogPos
	}
	if *executedGtidSet != "" {
		MasterInfo.ExecutedGTIDSet = *executedGtidSet
	}
	if *autoPosition {
		MasterInfo.AutoPosition = *autoPosition
	}
	if *serverId != 0 {
		MasterInfo.ServerID = uint32(*serverId)
	}
	if *serverType != "" {
		MasterInfo.ServerType = *serverType
	}

	if *printMasterInfo {
		PrintMasterInfo()
		os.Exit(0)
	}

	loggerInit()
}

// PrintConfiguration for `-print-config` flag
func PrintConfiguration() {
	data, _ := yaml.Marshal(Config)
	fmt.Print(string(data))
}

// PrintMasterInfo for `-print-master-info` flag
func PrintMasterInfo() {
	data, _ := yaml.Marshal(MasterInfo)
	fmt.Print(string(data))
}

// parseConfigFile load config from file
func (conf *Configuration) parseConfigFile(path string) error {
	path = getConfigFile(path)
	configFile, err := os.Open(path)
	if err != nil {
		return err
	}
	defer configFile.Close()

	content, err := ioutil.ReadAll(configFile)
	if err != nil {
		return err
	}

	err = yaml.Unmarshal(content, &Config)
	if err != nil {
		return err
	}
	return nil
}

// getConfigFile config file load sequence
func getConfigFile(path string) string {
	if path == "" {
		path = "/etc/lightning.yaml"
		_, err := os.Stat(path)
		if err == nil {
			return path
		}

		path = "etc/lightning.yaml"
		_, err = os.Stat(path)
		if err == nil {
			return path
		}

		path = "lightning.yaml"
		_, err = os.Stat(path)
		if err == nil {
			return path
		}
	}
	return path
}

// version print version info
func version() {
	fmt.Println("Version: ", Version)
	fmt.Println("Compiled time: ", Compile)
	fmt.Println("Code branch: ", Branch)
	fmt.Println("GirDirty: ", GitDirty)
}

// SyncReplicationInfo sync replication status into master.info
func SyncReplicationInfo() {
	if Config.MySQL.SyncDuration.Seconds() == 0 {
		return
	}

	if Config.MySQL.MasterInfo == "" || len(Config.MySQL.BinlogFile) > 0 {
		Log.Info("SyncReplicationInfo -master-info not specified, reading from '%v'", Config.MySQL.BinlogFile)
		return
	}

	for {
		FlushReplicationInfo()
		time.Sleep(Config.MySQL.SyncDuration)
	}
}

// FlushReplicationInfo flush master.info
func FlushReplicationInfo() {
	if Config.MySQL.MasterInfo == "" {
		return
	}
	f, err := os.OpenFile(Config.MySQL.MasterInfo, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0644)
	if err != nil {
		Log.Error(errors.Trace(err).Error())
		return
	}
	defer f.Close()
	info, err := yaml.Marshal(MasterInfo)
	if err != nil {
		Log.Error(errors.Trace(err).Error())
		return
	}
	_, err = f.WriteString(string(info))
	if err != nil {
		Log.Error(errors.Trace(err).Error())
	}
}

// LoadMasterInfo get master.info from file
func LoadMasterInfo() {
	if Config.MySQL.MasterInfo == "" {
		return
	}
	conf, err := ioutil.ReadFile(Config.MySQL.MasterInfo)
	if err != nil {
		fmt.Println("-- LoadMasterInfo Error: ", err.Error())
		return
	}
	err = yaml.Unmarshal(conf, &MasterInfo)
	if err != nil {
		fmt.Println("-- LoadMasterInfo Error: ", err.Error())
		return
	}
	if MasterInfo.ServerID == 0 {
		s1 := rand.NewSource(time.Now().UnixNano())
		r1 := rand.New(s1)
		MasterInfo.ServerID = uint32(r1.Intn(3306) + 3306)
	}
}

// ListPlugin list support plugin name and description
func ListPlugin() {
	fmt.Println("lightning -plugin support following type")
	fmt.Println("  sql(default): parse ROW format binlog into SQL.")
	fmt.Println("  flashback: generate flashback query from ROW format binlog")
	fmt.Println("  stat: statistic ROW format binlog table update|insert|delete query count")
	fmt.Println("  lua: self define lua scripts")
	fmt.Println("  find: find binlog file name by event time")
}

// TimeOffset timezone offset seconds
func TimeOffset(timezone string) int {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		Log.Error(err.Error())
		return 0
	}
	now := time.Now()
	_, destOffset := now.In(loc).Zone()
	_, localOffset := now.Zone()
	return destOffset - localOffset
}

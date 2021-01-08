# 重建 SQL

lightning 内建支持两种重建规则以及 SQL 类型统计功能，同时支持是 lua 插件形式进行自定义二次开发。

* sql: 生成 ROW 格式对应的原始 SQL
* flashback: 生成数据闪回 SQL，即：INSERT -> DELETE, DELETE -> INSERT, UPDATE WHERE 和 SET 互换。
* stat: 按表统计各表的请求类型

注：MySQL 高版本（5.6.2+）支持 `binlog_rows_query_log_events` 参数，该参数默认是关闭的，开启后可以在 binlog 中记录原始 SQL 请求，不需要使用其他工具进行复原效果更好。

## 差异

* ENUM, SET, BIT 使用整型替代，不影响数据一致性。
* DECIMAL 使用 float 替代，不影响精度。

## 配置文件

```yaml
# 重建规则
rebuild:
  # 插件：sql, flashback, stat, lua
  plugin: sql
  # INSERT 语句是否补全列
  complete-insert: false
  # INSERT 语句多个 VALUES 合并
  extended-insert-count: 0
  # 使用 REPLACE INTO 替代 INSERT INTO
  replace: false
  # 两条 SQL 语句之前添加 sleep 间隔，，最小精度 us
  sleep-interval: 0s
  # 生成 SQL 语句省略某些列，如： INSERT 忽略主键
  ignore-columns:
    - id
  # lua 插件脚本位置
  lua-script: plugin/demo.flashback.lua
  # 对表名进行简写，如：`db`.`tb` -> `tb`，可以用在测试库做预恢复的场景
  without-db-name: false
```

## 示例

ROW 格式 binlog 生成 SQL 更新语句。不指定 `-plugin` 默认即为该模式。

```bash
lightning -no-defaults -schema-file test/schema.sql -binlog-file test/binlog.000002
```

ROW 格式 binlog 生成回滚语句。

```bash
lightning -no-defaults -plugin flashback -schema-file test/schema.sql -binlog-file test/binlog.000002
```

## 统计分析

### 统计各库表更新语句数量

```bash
lightning -no-defaults -plugin stat -binlog-file test/binlog.000002

{
  "TableStats": {
    "`test`.`bitTest`": {
      "insert": 1
    },
    "`test`.`enumTest`": {
      "insert": 1
    },
    "`test`.`setTest`": {
      "insert": 3
    },
    "`test`.`tb`": {
      "delete": 1,
      "insert": 1,
      "update": 1
    },
    "`test`.`testNoPRI`": {
      "insert": 1
    }
  },
  "QueryStats": {
    "ALTER": 1,
    "BEGIN": 9,
    "CREATE": 6,
    "DROP": 4
  },
  "TransactionStats": {
    "SizeBytes": {
      "Max": "141.0",
      "MaxTransactionPos": "1292",
      "Mean": "131.3",
      "Median": "129.0",
      "P95": "139.5",
      "P99": "139.5"
    },
    "TimeSeconds": {
      "Max": "0.00",
      "MaxTransactionPos": "0",
      "Mean": "0.00",
      "Median": "0.00",
      "P95": "0.00",
      "P99": "0.00"
    }
  }
}
```

### 使用 mysqlbinlog + awk 分析

参考: [Identifying useful info from MySQL row-based binary logs](https://www.percona.com/blog/2015/01/20/identifying-useful-information-mysql-row-based-binary-logs/)

Q1: Which tables received highest number of insert/update/delete statements?

```bash
./summarize_binlogs.sh | grep Table |cut -d':' -f5| cut -d' ' -f2 | sort | uniq -c | sort -nr
```

Q2: Which table received the highest number of DELETE queries?

```bash
./summarize_binlogs.sh | grep -E 'DELETE' |cut -d':' -f5| cut -d' ' -f2 | sort | uniq -c | sort -nr
```

Q3: How many insert/update/delete queries executed against sakila.country table?

```bash
./summarize_binlogs.sh | grep -i '`sakila`.`country`' | awk '{print $7 " " $11}' | sort -k1,2 | uniq -c
```

Q4: Give me the top 3 statements which affected maximum number of rows.

```bash
./summarize_binlogs.sh | grep Table | sort -nr -k 12 | head -n 3
```

Q5: Find DELETE queries that affected more than 1000 rows.

```bash
./summarize_binlogs.sh | grep -E 'DELETE' | awk '{if($12>1000) print $0}'

./summarize_binlogs.sh | grep -E 'Table' | awk '{if($12>1000) print $0}'
```

### 使用 mysqlbinlog + pt-query-digest 分析

参考：

* [pt-query-digest](https://www.percona.com/doc/percona-toolkit/LATEST/pt-query-digest.html)
* [pt-query-digest 解析 MySQL Binlog 日志文件](https://blog.csdn.net/dba_waterbin/article/details/14453255)

```bash
mysqlbinlog mysql-bin.000441 > mysql-bin.000441.txt

pt-query-digest --type binlog mysql-bin.000441.txt

pt-query-digest --type binlog --since "2019-01-06 20:55:00" --until "2019-01-06 21:00:00" mysql-bin.000441.txt

pt-query-digest --type binlog --group-by fingerprint --limit "100%" --order-by "Query_time:cnt" --output report --report-format profile mysql-bin.000441.txt
```

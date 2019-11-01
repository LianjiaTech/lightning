# 过滤器

## 表过滤器

可以配置只同步某些表 `tables` 或不同步某些表 `ignore-tables`。需要注意的是必需指定库名，不可以只指定表名，如需匹配所有库的某张表可以写 %.tb。

### 命令行

用逗号作为分隔符，格式为：{db}.{tb}，使用 `%` 作为通配符。

```bash
-tables db1.tb1,db1.tb2,db2.%,%.tb -ignore-tables db3.ignore_tb
```

### 配置文件

```yaml
filters:
  tables:
    - db1.tb1
    - db1.tb2
    - db2.%
  ignore-tables:
    - db2.ignore
```

## 事件过滤器

只匹配特殊的事件类型，如：insert, delete, update, delete, create, drop 等，主意中间不要有空格，使用小写字母。

### 命令行

```bash
-event-types delete,insert
```

### 配置文件

```yaml
filters:
  event-types:
    - insert
    - update
```

## 时间过滤器

像 `mysqlbinlog` 一样可以指定开始时间 `start-datetime` 和结束时间 `stop-datetime` 。如不指定 `stop-datetime` 又未配置 `demonize` 时 `stop-datetime` 使用当前时间为默认值。时间格式： `2006-01-02 15:04:05`。注意要配合时区使用，如不配置 lightning 使用 `UTC` 作为默认时区。

### 命令行

```bash
-time-zone Asia/Shanghai -start-datetime "2019-10-01 00:00:00" -stop-datetime "2019-10-01 01:00:00"
```

### 配置文件

```yaml
global:
  time-zone: Asia/shanghai
filters:
  start-datetime: ""
  stop-datetime: "2019-10-01 01:00:00"
```

## 文件及位点过滤器

lightning 可以从文件读取日志，也可以向 MySQL 发送 `Binlog Dump` 命令模拟从库读取日志。如需从文件读取日志使用 `-binlog-file` 指定启始读取的日志文件名，如使用 `Binlog Dump` 读取日志需使用 `-master-info` 指定 MySQL 同步源。只解析 ROW 格式的 binlog 无得到库表结构，因为无法直接还原为 SQL 语句，需要结合 `schema-file` 或 `master-info` 来获取库表结构。与 `mysqlbinlog` 使用方式相同，使用 `start-position` 和 `stop-position` 两个参数指定起始点和终止点位。

### 命令行

```bash
lightning -binlog-file binlog.000001
或将文件名置于最后一个参数
lightning binlog.000001
```

### 配置文件

```yaml
mysql:
  binlog-file: binlog.000002
  schema-file: schema.sql
  master-info: etc/master.info
filters:
  start-position: 0
  stop-position: 0
```

使用 `schema-file` 来读取库表结构的处是可以使用表结修改前的信息来复原 SQL 。

```sql
use test;
create table tb (
    `a` int,
    `b` varchar(10),
    PRIMARY KEY (`a`)
) ENGINE = InnoDB;
```

master.info 文件格式如下，如从文件读取日志不需要指定 `master_log_file` 和 `master_log_pos` 等信息，只会连接 MySQL 获取库表结构。如使用 `Binlog Dump` 方式读取日志，需要指定 `master_log_file`, `master_log_pos` 及 `server-id`。`server-id` 如不指定使用 3306+RAND(3306) 作为 `server-id`，为避免冲突建议手工指定 `server-id`。

```yaml
master_host: 127.0.0.1
master_user: root
master_password: ******
master_port: 3306
master_log_file: binlog.000002
master_log_pos: 4
gtid_next: ""
server-id: 3307
server-type: mysql
```

## 线程过滤器

通过 `thread-id` 过滤指定线程，可用于单次 SQL 上线的快速回滚。

### 命令行

```bash
-thread-id 10086
```

### 配置文件

```yaml
filters:
  thread-id: 10086
```

## 主库 server-id 过滤器

通过 `server-id` 过滤指定的主库，当存在多源级联同步或从库有数据写入时使用该参数。可以使用 `SELECT @@server_id;` 查看主库的 `server-id`。

### 命令行

```bash
-server-id 1
```

### 配置文件

```yaml
filers:
  server-id: 1
```

## GTID 过滤器

对于开启了 GTID 的 MySQL 实例也可以通过 `include-gtids` 和 `exclude-gtids` 来做过滤。以上两个参数可以配多个 `gtid_set`， 格式为 {uuid}:N-M，多个 `gtid_set` 使用逗号连接。

### 命令行

```bash
-include-gtids 376b1ae7-39a1-11e9-a253-14187759814e:1-100 -exclude-gtids 376b1ae7-39a1-11e9-a253-14187759814e:1-20
```

### 配置文件

```yaml
filters:
  include-gtids: 376b1ae7-39a1-11e9-a253-14187759814e:1-100
  exclude-gtids: 376b1ae7-39a1-11e9-a253-14187759814e:1-20,376b1ae7-39a1-11e9-a253-14187759814e:40-60
```

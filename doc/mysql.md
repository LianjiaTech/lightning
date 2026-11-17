# 数据库相关配置

## 数据库授权

```sql
CREATE USER 'user'@'%' IDENTIFIED /*!80000 WITH mysql_native_password */ BY 'xxx';
GRANT SELECT, REPLICATION SLAVE, REPLICATION CLIENT ON *.* TO 'user'@'%';
```

## mysql 段配置

```yaml
# 日志源
mysql:
  # 从文件读取二进制日志
  binlog-file: test/binlog.000002
  # 建表语句文件
  schema-file: test/schema.sql
  # MySQL 源
  master-info: etc/master.info
  # master-info sync interval，默认 1s sync 一次，配置为 0 后，每解析完成一个事务都会更新 master.info
  sync-interval: 1s
  # Binlog Dump I/O read timeout.
  read-timeout: 3s
```

## master.info

```yaml
# 主库 IP
master_host: 127.0.0.1
# 同步账号，用于查建表语句，发起 Binlog Dump 指令
master_user: root
# 同步账号密码
master_password: ******
# 数据库端口
master_port: 3306
# 起始同步文件
master_log_file: binlog.000002
# 起始同步点位
master_log_pos: 4
# 是否开启 GTID
auto_position: false
# 同步延迟时间
seconds_behind_master: 0
# GTID 模式下起始同步点位
gtid_next: ""
# 模拟从库 server-id
server-id: 33061
# 服务类型，暂未使用不建议修改
server-type: mysql
```

注意：如不指定 master_log_file 和 master_log_pos 默认从首个 binlog 文件开始同步，而不是当前最新的同步点位。如需从当前最新点位同步需指定 `replicate-from-current-position` 参数。 

## schema-file 表结构快照

`schema-file` 既可以是手工维护的建表语句文件，也可以由 lightning 自动补全为本地快照：

1. **文件存在且能解析出表结构**：直接加载，启动阶段不再连接 MySQL；
2. **文件不存在或内容为空**：从 MySQL 读取库表结构（使用 `master-info` 或 `-host/-user/-password`），成功后写入该文件；
3. **解析 binlog 时遇到文件里没有的表**：只对该表执行一次 `SHOW CREATE TABLE`，加入内存索引并追加到文件中。每张表最多尝试一次，失败后按原逻辑处理（全列作为主键），不会阻塞解析。

快照使用普通建表语句，格式与 `mysqldump --no-data` 的输出兼容，也可以直接当作 `-schema-file` 给其他工具使用：

```sql
USE `fieia`;
CREATE TABLE `t_order` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  ...
  PRIMARY KEY (`id`)
);
```

加载本地快照的成本很低：1000 张表约 30~40ms，5000 张表约 130~160ms；而逐表远程 `SHOW CREATE TABLE` 在大库上可能需要几十秒。对“每个 binlog 文件启动一次进程”的批处理方式，第一次运行会生成快照，后续启动直接读本地文件。

```bash
# 第一次：连库读取并生成快照；之后：直接使用本地快照
lightning -host 127.0.0.1 -user root -password '******' \
  -schema-file /var/tmp/fieia.schema \
  -plugin flashback -tables 'fieia.%' \
  -binlog-file binlog.000001 > flashback.sql
```

注意：

* 表结构变更后可删除该文件，让 lightning 重新生成；待分析 binlog 范围内的 DDL 仍会由内存表结构同步逻辑自动应用。
* 懒加载需要可用的 MySQL 连接（`master-info` 或 `-host/-user/-password`）；没有连接信息时，缺失表按原逻辑处理，不会阻塞解析；文件为空且没有连接信息时会记录错误并按无表结构继续。
* `-plugin stat` 不使用表结构，不会读取或更新快照。
* 多个 lightning 进程并行共用同一个快照文件时，建议先生成好快照再并发执行，避免同时追加。

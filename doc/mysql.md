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

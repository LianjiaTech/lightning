# FAQ

## 数据闪回实现的原理？

参考这篇文章 [MySQL 下实现闪回的设计思路 (MySQL Flashback Feature)](http://www.penglixun.com/tech/database/mysql_flashback_feature.html)

## 是否支持 GTID？

lightning 支持 GTID event 的分析，可以分析含有 GTID 的 binlog 文件，也可以实时分析开启 GTID 的 MySQL 二进制日志。

## 能否用于 relay-log 的解析？

MySQL relay-log 的格式与 binlog 相同，但对应 event 中结构体内成员代表的含义会略有不同。解析 relay-log 可以生成 SQL 及对应 SQL 的回滚语句，但如果用于统计分析，需要参考 MySQL [文档](https://dev.mysql.com/doc/internals/en/binary-log-structure-and-contents.html) 明确每个 event 成员的真实含义。

## 待恢复表有外键关联

```text
ERROR 1451 (23000): Connot delete or update a parent row: a foreign key constraint fails
```

存在外键的表应先恢复父表记录，再恢复子表，故报错。

```sql
SET SESSION FOREIGN_KEY_CHECKS = 0; -- 数据恢复前禁用外键检查

SOURCE rollback.sql

SET SESSION FOREIGN_KEY_CHECKS = 1; -- 数据恢复完成后开启外键检查
```

## GTID 同步报错

使用 Binlog Dump GTID 方式同步数据时如果报以下错误，很可能是 master.info 中的 executed_gtid_set 配置错了。正确的配置格式参考 MySQL [官方文档](https://dev.mysql.com/doc/refman/5.7/en/replication-gtids-concepts.html) GTID Sets。

```text
ERROR 1236 (HY000): The slave is connecting using CHANGE MASTER TO MASTER_AUTO_POSITION = 1, but the master has purged binary logs containing GTIDs that the slave requires.
```

查看当前主库 master status。

```sql
mysql > show master status;
+------------------+----------+--------------+------------------+-----------------------------------------------------------------------------------+
| File             | Position | Binlog_Do_DB | Binlog_Ignore_DB | Executed_Gtid_Set                                                                 |
+------------------+----------+--------------+------------------+-----------------------------------------------------------------------------------+
| mysql-bin.000020 |      234 |              |                  | 376b1ae7-39a1-11e9-a253-14187759814e:1,
3b0075c9-39a1-11e9-a250-f86eee9113c6:1-98 |
+------------------+----------+--------------+------------------+-----------------------------------------------------------------------------------+
1 row in set (0.00 sec)
```

## zombie Binlog Dump

当 lightning 异常退出未断开与主库的同步又重新启动后，主库错误日志会打印如下信息，这种情况不需要处理。

多个 lightning 同步同一个主库且 master.info 中 server-id 配置相同进也会出现类型错误日志，此时需要手工指定不同的 server-id 解决。

```text
2019-05-23T10:29:27.082175+08:00 631113 [Note] Start binlog_dump to master_thread_id(631113) slave_server(33061), pos(mysql-bin.000018, 2316)
2019-05-23T10:30:32.574203+08:00 631125 [Note] While initializing dump thread for slave with server_id <33061>, found a zombie dump thread with the same server_id. Master is killing the zombie dump thread(631113).
```

参考文章：

* [MySQL 多个 Slave 同一 server_id 的冲突原因分析](http://www.penglixun.com/tech/database/mysql_multi_slave_same_serverid.html)
* [两台备库设置 server_id 一致的问题](https://win-man.github.io/2017/07/11/%E4%B8%A4%E5%8F%B0%E5%A4%87%E5%BA%93%E8%AE%BE%E7%BD%AEserver-id%E4%B8%80%E8%87%B4%E7%9A%84%E9%97%AE%E9%A2%98/)

## 使用 lua 插件实现数据双写同步效率问题

尝试用 lua 插件来实现将某些表数据写两份时，我们发现写入速度不够快，在高 QPS(>1K) 情况下很难保证数据低延迟。

从实现机制上分析，lua 插件是通过 TCP 协议远程写对端 MySQL，执行一条写操作一来一回网络相对较大，而且更新线程只有一个，不像 MySQL 是进程内本地多线程复制。

优化思路：

1. 将 lightning 与待更新的 MySQL 本地部署，通过 127.0.0.1 或 socket 方式写数据库。
2. 对一定时间的 binlog 合并更新，减少更新中间态。
3. 模仿多线程同步，通过配置表过滤规则，不同表开启不同的 lightning 进程同步。

# 参考

## 同类产品

* [MariaDB mysqlbinlog](https://mariadb.com/kb/en/library/flashback/)
* [binlog2sql](https://github.com/danfengcao/binlog2sql)
* [MyFlash](https://github.com/Meituan-Dianping/MyFlash)
* [mysqlbinlog_flashback](https://github.com/58daojia-dba/mysqlbinlog_flashback)
* [pt-query-digest](https://www.percona.com/doc/percona-toolkit/LATEST/pt-query-digest.html)

## 技术文章

数据闪回

* [MySQL Internals Manual  /  The Binary Log](https://dev.mysql.com/doc/internals/en/binary-log.html)
* [ROWS_EVENT](https://dev.mysql.com/doc/internals/en/rows-event.html)
* [MySQL 下实现闪回的设计思路 (MySQL Flashback Feature)](http://www.penglixun.com/tech/database/mysql_flashback_feature.html)
* [Provide the flashback feature by binlog](https://bugs.mysql.com/bug.php?id=65178)
* [MySQL 闪回方案讨论及实现](https://dinglin.iteye.com/blog/1539167)
* [mysqlbinlog flashback 5.6 完全使用手册与原理](http://www.cnblogs.com/youge-OneSQL/p/5249736.html)
* [拿走不谢，Flashback for MySQL 5.7](http://t.cn/E9ZH8sU)
* [AliSQL and some features that have made it into MariaDB Server](https://mariadb.com/resources/blog/alisql-and-some-features-that-have-made-it-into-mariadb-server/)
* [MyFlash—— 美团点评的开源 MySQL 闪回工具](http://t.cn/RjRbjpM)
* [Identifying useful info from MySQL row-based binary logs](https://www.percona.com/blog/2015/01/20/identifying-useful-information-mysql-row-based-binary-logs/)
* [MySQL 多个 Slave 同一 server_id 的冲突原因分析](http://www.penglixun.com/tech/database/mysql_multi_slave_same_serverid.html)
* [两台备库设置 server_id 一致的问题](https://win-man.github.io/2017/07/11/两台备库设置server-id一致的问题/)
* [pt-query-digest 解析 MySQL Binlog 日志文件](https://blog.csdn.net/dba_waterbin/article/details/14453255)
* [Binary Log Options and Variables](https://dev.mysql.com/doc/refman/5.6/en/replication-options-binary-log.html)
* [Read MySQL Binlogs better with rows query log events](https://mydbops.wordpress.com/2017/08/02/read-mysql-binlogs-better-with-binlog_rows_query_log_events/)

Lua

* [Lua 教程](https://www.runoob.com/lua/lua-tutorial.html)
* [gopher-lua](https://github.com/yuin/gopher-lua)
* [Embedding Lua in Go](https://otm.github.io/2015/07/embedding-lua-in-go/)

## 第三方包引用

* [go-mysql](https://github.com/go-mysql-org/go-mysql)
* [pingcap/parser](https://github.com/pingcap/parser)
* [gopher-lua](https://github.com/yuin/gopher-lua)

## 性能对比

lightning 与 mysqlbinlog 原生工具比较在文件解析速度上存在差距。目前分析主要的瓶颈点在于 lightning 需要识别不同的库、表、列、值等数据，并按照正确的语法逻辑进行拼接，但 mysqlbinlog 并不需要这样做，即使是 verbose 模式生成的 SQL 也并不能真正执行。

```bash
ls -lh mysql-bin.001287
-rw-rw---- 1 mysql mysql 513M May 22 01:12 mysql-bin.001287

time lightning mysql-bin.001287 > mysql-bin.001287.lightning.sql

real    0m20.563s
user    0m23.280s
sys     0m6.806s

time mysqlbinlog -vv mysql-bin.001287 > mysql-bin.001287.mysqlbinlog.sql

real    0m7.892s
user    0m5.636s
sys     0m2.163s

time mysqlbinlog mysql-bin.001287 > mysql-bin.001287.mysqlbinlog.raw.sql

real    0m3.492s
user    0m1.571s
sys     0m1.871s

ls -lh mysql-bin.001287.*
-rw-r--r-- 1 mysql mysql 758M May 23 11:32 mysql-bin.001287.lightning.sql
-rw-r--r-- 1 mysql mysql 1.4G May 23 11:32 mysql-bin.001287.mysqlbinlog.sql
-rw-r--r-- 1 mysql mysql 635M May 23 12:37 mysql-bin.001287.mysqlbinlog.raw.sql
```

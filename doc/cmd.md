# 常用命令

## ROW 格式转换为 STATEMENT 格式

```bash
lightning -user xxx -password xxx -host xxx -port xxx binlog.00000[123]
或
lightning -schema-file schema.sql -plugin sql binlog.00000[123]

cat schema.sql
use test;
create table tb (
  a int,
  b varchar(10),
  primary key (a)
)
```

## 生成回滚语句

```bash
lightning -schema-file schema.sql -plugin flashback binlog.000001
```

## 统计各表更新量

```bash
lightning -no-defaults -plugin stat -event-types insert,update,delete binlog.000001 | jq -r '.TableStats | keys[] as $k | "\($k)  \(.[$k] | .insert + .delete + .update)"'  | sort -k 2 -nr | column -t | head
```

## 大事务、长事务分析

verbose 模式中可以看到很多 binlog event 的信息，其中 TransactionSizeBytes 表示事务的 binlog event 大小。主库 binlog 和从库的 relay-log 中 ExecutionTime 显示的是事务执行时间，从库的 binlog 中 ExecutionTime 为从库同步延迟时间并不是事务执行耗时。

```bash
lightning -no-defaults -verbose -schema-file test/schema.sql test/binlog.000002  | grep "DEBUG" | grep "TransactionSizeBytes\|ExecutionTime"
```

## 查找指定时间的 event 在哪个 binlog 文件？

```
# 要注意命令行支持的最大长度，直接 $(ls mysql-bin.0*) 可能导致参数过长无法获取结果
lightning -binlog-file "$(ls mysql-bin.0*)" -start-datetime "2021-01-13 07:00:00" -stop-datetime "2021-01-13 18:00:00" -plugin find
```

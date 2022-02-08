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

```bash
# 要注意命令行支持的最大长度，直接 $(ls mysql-bin.0*) 可能导致参数过长无法获取结果
lightning -binlog-file "$(ls mysql-bin.0*)" -start-datetime "2021-01-13 07:00:00" -stop-datetime "2021-01-13 18:00:00" -plugin find
```

## 通过 keyring 解密 MySQL 8.0 加密的 binlog

MySQL 8.0 支持通过 keyring 对 binlog 进行加密，已经加密的 binlog lightning 解密需要提供 keyring 文件路径。只是解密不需要提取 SQL 时可以通过如下命令进行解密。

```bash
lightning -plugin decrypt -keyring keyring binlog.encrypted > binlog.decrypted
```

注意：lightning 解密会将结果打印到标准输出，需要添加输出重定向，不然会满屏乱码。lightning 的解密方式是流式的，不用担心大文件导致内存使用过多。

lightning 还支持直接分析加密的 binlog，只需要添加 `-keyring` 配置即可，会根据 binlog 文件头的 magic header 类型自动识别。

```bash
# 回滚所有删除事件
lightning -plugin flashback -keyring keyring -event-types delete binlog.encrypted
```

## 从标准输入读取 binlog

有时候需要分析的 binlog 并不在本机，可能存储在 s3 或其他服务器上。分析远程 binlog 文件时不想占用本地磁盘空间可以选择从标准输入读取 binlog。添加 `-` 表示从标准输入读取文件内容，示例如下：

```bash
# 仅以 binlog 文件存储在 minio 为例，使用 minio client 读取远程经过压缩的 binlog 文件
mc cat path/to/binlog_file.gz | gunzip | ./lightning - > decrypt_log.sql
```

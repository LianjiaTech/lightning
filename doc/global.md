# 全局配置

```yaml
# 全局配置
global:
  # 日志级别
  log-level: 3
  # 日志文件名
  log-output: lightning.log
  # 是否开启守护进程模式
  demonize: false
  # 数据库字符集
  charset: utf8mb4
  # CPU 使用限制
  cpu: 0
  # Verbose 模式，打印更多信息
  verbose: false
  # 比 Verbose 还 Verbose
  verbose-verbose: false
  # 设置时区，默认 UTC
  time-zone: UTC
```

## time-zone

该参数需要与 MySQL 服务器指定的时区相同，否则会导致 `start-datetime`, `stop-datetime` 等参数无法生效。

* UTC：世界标准时间。协调世界时，又称世界标准时间或世界协调时间，其以原子时秒长为基础，在时刻上尽量接近于格林尼治标准时间。
* Asia/Shanghai：为本地时间，一个国家或地区使用时间，中国为东八区。

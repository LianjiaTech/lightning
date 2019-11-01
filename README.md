# 简介

[文档](http://github.com/LianjiaTech/lightning/blob/master/doc/) | [English Readme](http://github.com/LianjiaTech/lightning/blob/master/README_EN.md)

![lightning](https://github.githubassets.com/images/icons/emoji/unicode/26a1.png) lightning 是由贝壳找房 DBA 团队开发和维护的一个 MySQL binlog 转换工具。该工具可以将 MySQL ROW 格式的 binlog 转换为想要的 SQL，如：原始 SQL，闪回 SQL等。也可以对 binlog 进行统计分析，用于数据库异常分析。甚至可以通过定制 lua 插件进行二次开发，发挥无限的想象力。

## 应用

* 数据修改错误，需要快速回滚 (闪回)
  * DELETE, UPDATE 未指定 WHERE 条件
  * UPDATE SET 误用 AND 连接
* 数据异常， 从 binlog 中找特定表某些数据是什么时间修的
* 业务流量异常或从库同步延迟，需要统计排查是哪些表在频繁更新
* 需要把指定表，指定时间的更新提供给开发定位服务异常问题
* 主从切换后新主库丢失数据的修复
* 从 binlog 生成标准 SQL，带来的衍生功能
* 找出某个时间点数据库是否有大事务 (Size) 或者长事务 (Time)

## 优点

* 跨平台支持，二进制文件即下即用，无其他依赖。
* 支持 lua 定制化插件，发挥无限的想象力，二次开发周期短。
* 支持从 SQL 文件加载库表信息，不必连接 MySQL 便于历史变更恢复。
* SQL 进行多行合并，相比 mysqlbinlog ROW 格式，更好过滤。

## 安装

### 二进制安装

lightning 使用 Go 1.11+ 开发，可以直接下载编译好的二进制文件在命令行下使用。由于 Go 原生对跨平台支持较好，在 Windows, Linux, Mac 下均可使用。

[下载地址](https://github.com/LianjiaTech/lightning/releases)

### 源码安装

```bash
go get -d github.com/LianjiaTech/lightning
cd ${PATH_TO_SOURCE}/lightning # 进入源码路径，PATH_TO_SOURCE 需要人为具体指定。
make
```

## 测试示例

[常用命令](http://github.com/LianjiaTech/lightning/blob/master/doc/cmd.md)

直接读取文件生成回滚语句

```bash
lightning -no-defaults \
-plugin flashback \
-start-datetime "2019-01-01 00:00:00" \
-stop-datetime "2019-01-01 00:01:00" \
-event-types delete,update \
-tables test.tb \
-schema-file schema.sql \
-binlog-file binlog.0000001 > flashback.sql
```

使用 `Binlog Dump` 方式读取日志生成回滚语句

```bash
cat > master.info
master_host: 127.0.0.1
master_user: root
master_password: ****** 
master_port: 3306
master_log_file: binlog.000002
master_log_pos: 4
<ctrl>+D

lightning -no-defaults \
-plugin flashback \
-start-datetime "2019-01-01 00:00:00" \
-stop-datetime "2019-01-01 00:01:00" \
-event-types delete,update \
-tables test.tb \
-master-info master.info > flashback.sql
```

## 配置

lightning 使用 YAML 格式的配置文件。使用 `-config` 参数指定配置文件路径，如不指定默认按 /etc/lightning.yaml -> ./etc/lightning.yaml -> ./lightning.yaml 的顺序加载配置文件。如果不想使用默认路径下的配置文件还可以通过 `-no-defaults` 参数屏蔽所有默认配置文件。

* [全局配置](http://github.com/LianjiaTech/lightning/blob/master/doc/global.md)
* [MySQL 日志源](http://github.com/LianjiaTech/lightning/blob/master/doc/mysql.md)
* [过滤器](http://github.com/LianjiaTech/lightning/blob/master/doc/filters.md)
* [SQL 重建规则](http://github.com/LianjiaTech/lightning/blob/master/doc/rebuild.md)

## 限制/局限

* 仅测试了 v4 版本 (MySQL 5.1+) 的 binlog，更早版本未做测试。
* BINLOG_FORMAT = ROW
* 参数 BINLOG_ROW_IMAGE 必须为 FULL，暂不支持 MINIMAL
* 由于添加了更多的处理逻辑，解析速度不如 mysqlbinlog 快
* 当 binlog 中的 DDL 语句变更表结构时，lightning 中的表结构原数据并不随之改变（TODO）

## 沟通交流

* [常见问题(FAQ)](http://github.com/LianjiaTech/lightning/blob/master/doc/FAQ.md)
* 欢迎通过 Github Issues 提交问题报告与建议
* QQ 群： 573877257

![QQ](http://github.com/LianjiaTech/lightning/raw/master/doc/qq_group.png)

## License

[Apache License 2.0](http://github.com/LianjiaTech/lightning/blob/master/LICENSE)

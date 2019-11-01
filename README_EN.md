# Introduction

![lightning](https://github.githubassets.com/images/icons/emoji/unicode/26a1.png) lightning is developed and maintained by Ke's DBA Team. It's a tool for binlog parsing. It can generate rollback SQL if BINLOG_FORMAT=ROW, and also binlog statistics. Lua self develop plugin is also supportted, which can do what you can imagine.

## Scenario

* Data modification error, need to quickly rollback (flashback).
  * DELETE, UPDATE with no WHERE condition.
  * UPDATE SET connect with AND.
* Data is abnormal, find at which time it changed?
* binlog statistic, find which table update most.
* filter specified table's query.
* Repair of lost data from master failure.
* Generate self define format SQL from binlog.
* Find out if the database has a large transaction (Size) or a long transaction (Time) at certain time.

## Advance

* Cross operation system support.
* Lua plugin supported, self develop friendly.
* Schema info can load from file, can parse binlog file offline.
* SQL format self definable, can be filtered line by line.

## Installation

### Binary Install

lightning developed with Go 1.11+，no matter you are using with Windows, Linux, MySQL, just downloading the released binary file and happy to work with it.

[Download](https://github.com/LianjiaTech/lightning/releases)

### Source Code Install

```bash
go get -d github.com/LianjiaTech/lightning
cd ${PATH_TO_SOURCE}/lightning
make
```

## Try it

[Useful command](http://github.com/LianjiaTech/lightning/blob/master/doc/cmd.md)

Read binlog event from file and generate rollback SQL.

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

Send `Binlog Dump` command and simulate as slave genearate rollback SQL.

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

## Configuration

lightning's config file is YAML formated. Configure file load sequence: /etc/lightning.yaml -> ./etc/lightning.yaml -> ./lightning.yaml. With `-config` argument can find new config file, with `-no-defaults` can disable all default config.

* [Global Config](http://github.com/LianjiaTech/lightning/blob/master/doc/global.md)
* [MySQL Config](http://github.com/LianjiaTech/lightning/blob/master/doc/mysql.md)
* [Filter Config](http://github.com/LianjiaTech/lightning/blob/master/doc/filters.md)
* [Rebuild Config](http://github.com/LianjiaTech/lightning/blob/master/doc/rebuild.md)

## Limitation

* binlog version only support v4 (MySQL 5.1+), no test with(<= MySQL 5.0)
* BINLOG_FORMAT = ROW
* BINLOG_ROW_IMAGE = FULL
* for binlog parsing performance not better than mysqlbinlog itself.

## Communication

* [FAQ](http://github.com/LianjiaTech/lightning/blob/master/doc/FAQ.md)
* Welcome feed back with Github Issues.
* QQ Group： 573877257

![QQ](http://github.com/LianjiaTech/lightning/raw/master/doc/qq_group.png)

## License

[Apache License 2.0](http://github.com/LianjiaTech/lightning/blob/master/LICENSE)

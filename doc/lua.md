# Lua 插件

## 简介

lightning 使用 [gopher-lua](https://github.com/yuin/gopher-lua) 作为 [Lua](http://www.lua.org/) 解析执行引擎，可以根据用户的需求对 binlog 的转写方式进行定制化修改，而又不必修改重新编译 Go 代码。为了方便访问数据库 lightning 默认加载了 [gluadb](https://github.com/zhu327/gluadb) 库，用于连接 MySQL, Redis。

## 限制

当使用 lua 插件做数据同步或双写时要注意数据库写流量不可过大，一般超过 1K qps 就很难保证同步的实时性了。从原理上讲 lightning 写入的速度因为依赖连接 MySQL 的执行速度，所以不可能比 MySQL 自己的同步快，大部分开销在网络上。即使转为使用本地 IP 访问或 SOCKET 连接由于单线程同步，在更新量过大时也无法和 MySQL 原生的同步效率媲美。但如果表与表之间的更新互无相关性时可以考虑为每张表启动一个 lightning 进程实现并行复制。

## 配置

配置文件

```yaml
rebuild:
    plugin: lua
    lua-script: plugin/demo.flashback.lua
```

命令行参数

```bash
lightning -plugin lua -lua-script plugin/demo.flashback.lua
```

## 示例脚本

* [demo.flashback.lua](http://github.com/LianjiaTech/lightning/tree/master/plugin/demo.flashback.lua) 数据闪回示例
* [demo.mysql.lua](http://github.com/LianjiaTech/lightning/tree/master/plugin/demo.mysql.lua) 连接 MySQL 示例
* [demo.redis.lua](http://github.com/LianjiaTech/lightning/tree/master/plugin/demo.redis.lua) 连接 Redis 示例
* [demo.sql.lua](http://github.com/LianjiaTech/lightning/tree/master/plugin/demo.sql.lua) 转写 SQL 示例
* [demo.mode.lua](http://github.com/LianjiaTech/lightning/tree/master/plugin/demo.mod.lua) 引入第三方 Lua 库示例

## 全局变量

库表元数据

* GoPrimaryKeys map[string][]string
* GoColumns map[string][]string

记录值

* GoValues [][]string
* GoValuesWhere []string
* GoValuesSet []string

## 接口函数

以下接口函数必须在 lua 脚本中存在，不需要的函数可以将函数体留空。

### Init

全局初始化函数

### InsertRewrite

WRITE_ROWS_EVENT 转写函数。

### DeleteRewrite

DELETE_ROWS_EVENT 转写函数。

### UpdateRewrite

UPDATE_ROWS_EVENT 转写函数。

### QueryRewrite

QUERY_EVENT 转写函数。

### Finalizer

全局析构函数。
-- GoPrimaryKeys
-- GoColumns
-- GoValues

-- GoValuesWhere
-- GoValuesSet

mysql = require "mysql"
db = mysql:new()

-- Init inital funcation
function Init()
    connect()
end

-- InsertRewrite insert rewrite logic
function InsertRewrite (tab)

end

-- DeleteRewrite delete rewrite logic
function DeleteRewrite (tab)

end

-- UpdateRewrite update rewrite logic
function UpdateRewrite (tab)

end

-- QueryRewrite query rewrite logic
function QueryRewrite (sql)

end

-- Finalizer final destructor function
function Finalizer ()
    db:close()
end

function connect ()
    -- lua not support MySQL 8.0 caching_sha2_password authentication method yet
    local ok, err, errcode, sqlstate = db:connect{
        host = "127.0.0.1",
        port = 3306,
        database = "mysql",
        user = "root",
        password = "******",
        charset = "utf8",
        max_packet_size = 64 * 1024 * 1024, -- 64MB
    }
    if err then
        print("MySQL connected failed: ", err)
        return
    end
end

-- MySQL query
function query (sql)
    if db.state ~= STATE_CONNECTED then
        db:close()
        connect()
    end

    local res, err, errcode, sqlstate = db:query(sql, 0)
    if err then
        print("-- ", sql)
        print("MySQL query error: ", errcode, err)
    end
end

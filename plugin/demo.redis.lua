-- GoPrimaryKeys
-- GoColumns
-- GoValues

-- GoValuesWhere
-- GoValuesSet

redis = require "redis"
red = redis:new()

-- Init inital funcation
function Init()
    local ok, err = red:connect("127.0.0.1", 6379)
    ok, err = red:set("dog", "an animal")
    local res, err = red:get("dog")
    print("redis status: ",res)
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
    red:close()
end
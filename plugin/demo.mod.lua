-- GoPrimaryKeys
-- GoColumns
-- GoValues

-- GoValuesWhere
-- GoValuesSet

-- Init inital funcation
function Init()
    -- append your package.path
    -- use `mymod = require('mymod')` import third party lua library
    lfs = require("lfs")
    package.path = package.path .. lfs.currentdir() .. [[/?.lua]]
    local mymod = require("plugin/mymod")
    mymod.myfunc()
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
end

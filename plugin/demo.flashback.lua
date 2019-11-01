-- GoPrimaryKeys
-- GoColumns
-- GoValues

-- GoValuesWhere
-- GoValuesSet

-- Init inital funcation
function Init()
    -- print("Init ...")
end

-- InsertRewrite insert rewrite logic
function InsertRewrite (tab)
    local whereStr = {}
    -- primary-keys
    for k, keys in pairs(GoPrimaryKeys) do
        if k == tab
        then
            for _, key in ipairs(keys) do
                for t, cols in pairs(GoColumns) do
                    if k == t
                    then
                        for i, col in ipairs(cols) do
                            if col == key
                            then
                                if GoValues[i] == "NULL"
                                then
                                    table.insert(whereStr, string.format("%s IS %s", col, GoValues[i]))
                                else
                                    table.insert(whereStr, string.format("%s = %s", col, GoValues[i]))
                                end
                            end
                        end
                    end
                end
            end
        end
    end
    print(string.format("DELETE FROM %s WHERE %s;", tab, table.concat(whereStr, " AND ")))
end

-- DeleteRewrite delete rewrite logic
function DeleteRewrite (tab)
    local columnStr = ""
    -- columns
    for k, values in pairs(GoColumns) do
        if k == tab
        then
            columnStr = table.concat(values, ", ")
        end
    end
    print(string.format("INSERT INTO %s (%s) VALUES (%s);", tab, columnStr, table.concat(GoValues, ", ")))
end

-- UpdateRewrite update rewrite logic
function UpdateRewrite (tab)
    local whereStr = {}
    local setStr = {}
    for k, keys in pairs(GoPrimaryKeys) do
        if k == tab
        then
            for _, key in ipairs(keys) do
                for t, cols in pairs(GoColumns) do
                    if k == t
                    then
                        for i, col in ipairs(cols) do
                            if col == key
                            then
                                if GoValuesWhere[i] == "NULL"
                                then
                                    table.insert(setStr, string.format("%s IS %s", col, GoValuesWhere[i]))
                                else
                                    table.insert(setStr, string.format("%s = %s", col, GoValuesWhere[i]))
                                end
                            end
                        end
                    end
                end
            end
        end
    end

    for t, cols in pairs(GoColumns) do
        if t == tab
        then
            for i, col in ipairs(cols) do
                table.insert(whereStr, string.format("%s = %s", col, GoValuesSet[i]))
            end
        end
    end

    print(string.format("UPDATE %s SET %s WHERE %s;", tab, table.concat(setStr, ", "), table.concat(whereStr, " AND ")))
end

-- QueryRewrite query rewrite logic
function QueryRewrite (sql)
    print(string.format("%s", sql))
end

-- Finalizer final destructor function
function Finalizer ()
    -- print("Finalizer ...")
end

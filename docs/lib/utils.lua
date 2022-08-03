local Lib = {}

function Lib.totitlecase(name)
    return string
        .gsub(name, "(%l)(%w*)", function(a, b)
            return string.upper(a) .. b
        end)
        :gsub("-", " ")
        :gsub("^%d+", "")
end

function Lib.scandir(directory)
    local i, t, popen = 0, {}, io.popen
    
    local pfile = popen('ls -a "' .. directory .. '"')
    if pfile then
        for filename in pfile:lines() do
            i = i + 1
            t[i] = filename
        end
        pfile:close()
    end
    return t
end

return Lib

local Lib = {}

function Lib.normalize(name)
    return name
        :gsub("-", " ")
        :gsub("^%d+", "")
end

function Lib.totitlecase(name)
    return string
        .gsub(name, "(%l)(%w*)", function(a, b)
            return string.upper(a) .. b
        end)
end

function Lib.starts_with(str,start)
    return string.sub(str,1,string.len(start))==start
end


return Lib

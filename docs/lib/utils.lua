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

return Lib

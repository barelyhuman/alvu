local Lib = {}

function Lib.totitlecase(name)
    return string
        .gsub(name, "(%l)(%w*)", function(a, b)
            return string.upper(a) .. b
        end)
        :gsub("-", " ")
        :gsub("^%d+", "")
end

return Lib

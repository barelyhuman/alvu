---@diagnostic disable-next-line: undefined-global
local wdir = workingdir

local json = require("json")

ForFile = "00-readme.md"

function Writer(filedata)
    local sourcedata = json.decode(filedata)
    if sourcedata.name == "00-readme.html"
    then 
        local f = assert(io.open(wdir.."/../readme.md", "rb"))
        local content = f:read("*all")
        f:close()
        sourcedata.content = content
    end
    return json.encode(sourcedata)
end
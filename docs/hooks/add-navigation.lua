---@diagnostic disable-next-line: undefined-global
package.path = package.path .. ";" .. workingdir .. "/lib/?.lua"


local json = require("json")
local utils = require(workingdir .. ".lib.utils")

function Writer(filedata)
    local pagesPath = workingdir .. "/pages"
    local files = utils.scandir(pagesPath)
    local index = {}
    for fileIndex = 3, #files do
        if not (files[fileIndex] == "_head.html" or files[fileIndex] == "index.md" or files[fileIndex] == "_tail.html")
        then
            local name = string.gsub(files[fileIndex], ".md", "")
            name = string.gsub(name, ".html", "")
            local title, _ = utils.totitlecase(name)

            table.insert(index, {
                name = title,
                slug = name
            })
        end
    end

    table.insert(index, 1, {
        name = "..",
        slug = "index"
    })

    local source_data = json.decode(filedata)

    local template = [[
<header class="container">
<nav>
{{$baseurl:=.Meta.BaseURL}}
{{range .Data.index}}
    <a href={{print $baseurl .slug}}>{{.name}}</a>
{{end}}
</nav>
</header>
<main class="container">
]]

    source_data.content = template .. "\n" .. source_data.content .. "</main>"
    source_data.data = {
        index = index
    }

    return json.encode(source_data)
end

---@diagnostic disable-next-line: undefined-global
local wdir = workingdir

package.path = package.path .. ";" .. wdir .. "/lib/?.lua"

local json = require("json")
local alvu = require("alvu")
local utils = require(wdir .. ".lib.utils")

function Writer(filedata)
	local pagesPath = wdir .. "/pages"
	local index = {}
	local files = alvu.files(pagesPath)

	for fileIndex = 1, #files do
		local file_name = files[fileIndex]
		if
			not (file_name == "_layout.html" or file_name == "index.md" or utils.starts_with(file_name, "concepts/"))
		then
			local name = string.gsub(file_name, ".md", "")
			name = string.gsub(name, ".html", "")
			local title, _ = utils.normalize(name):lower()

			table.insert(index, {
				name = title,
				slug = name,
			})
		end
	end

	table.insert(index, 1, {
		name = "..",
		slug = "index",
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
]]

	source_data.content = template .. "\n" .. source_data.content
	source_data.data = {
		index = index,
	}

	return json.encode(source_data)
end

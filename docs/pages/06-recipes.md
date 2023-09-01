# Recipes

### TOC

- [Watching for Changes](#watching-for-changes)
- [Importing Other lua files](#importing-other-lua-files)
- [String Interpolation](#string-interpolation)
- [String Functions](#string-functions)
- [Get Files from a Dir](#get-files-from-a-directory)
- [Reading Writing Files](#reading--writing-files)
- [Getting network Data](#getting-network-data)
- [Templates](#templates)

Methods and ways to be able to do basic tasks while working with alvu

## Watching for changes

I use [entr](https://github.com/eradman/entr) as a file notifier which can run
arbitrary commands on file changes, which can be done like so to have alvu
monitor for files

```sh
ls docs/**/* | entr -cr alvu --path='./docs'
```

> **Note**: since v0.2.9, alvu comes with it's own file watcher and live-reload
> but is limited to the `public` and `pages` directory and it is still
> recommended to use `entr` if you wish to handle watching custom paths

This will list all files in the `docs` folder (root of an alvu project) and then
run the alvu command while specifying the base path to be `./docs`

## Importing other lua files

You'll need to work with lua files that are in a sibling directory in the
project and you can do so by adding them to the scripts `package.path` like so

```lua
-- specify that you wish to taken in any `.lua` file in the `lib` folder
package.path = package.path .. ";../lib/?.lua"

-- require lib/utils.lua to use utilities from it
local lib = require("lib.utils")
```

## String Interpolation

There's no way to directly do string interpolation in lua but is almost always
needed so here's how you can implement a small helper for it

```lua
local function interpolate(s, tab)
    return (s:gsub('($%b{})', function(w) return tab[w:sub(3, -2)] or w end))
end

-- usage
interpolate("this is an ${message} string", { message = "interpolated" })
```

## String Functions

A helper library is injected into all alvu hook files which can be required into
the script to help with basic string manipulation and querying

```lua
local strings = require("strings")
if strings.contains("hello world", "hello")
then
	print("found hello")
end
```

You can read more about these from the
[gopher-lua-libs](https://github.com/vadv/gopher-lua-libs/tree/master/strings)
repo.

## Get files from a directory

If working with blogs and nested data you might wanna get files in a directory
and you can use the following function

There's 2 methods, you can either use the `alvu` helper library or you can add
the `scandir` function shown below to your `libs` folder if you wish to maintain
control over the file reading functionality

- Using `alvu` helpers

```lua
local alvu = require("alvu")
local all_files = alvu.files(path)
```

- Custom function

```lua
function scandir(directory)
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
```

## Reading / Writing files

This can be done with native lua functions but here's a snippet of the
`onFinish` hook from [reaper.is](https://github.com/barelyhuman/reaper.is)' RSS
Feed hook

```lua
function OnFinish()
    -- attempt to open the template file in read mode
	local rss_temp_fd = io.open("dist/rss_tmpl.xml", "r")
	-- attempt to open the final file in write mode
    local rss_fd = io.open("dist/rss.xml", "w")

	-- check if the file descriptors are available and usable
    if rss_temp_fd and rss_fd
    then
		-- read the entire template file's body
		-- which contains the rss <item></item> tags
        local body = ""
        for c in rss_temp_fd:lines() do
            body = body .. "\n" .. c
        end

		-- generate a rss file template for the following
		-- site data
        local rss_data = rss_template({
            site_name = "reaper",
            site_link = "https://reaper.is",
            site_description = "reaper's rants,notes and stuff",
            itembody = body
        })

		-- write the whole thing to the final rss.xml file
        rss_fd:write(rss_data)
    end
end
```

## Getting Network Data

Getting data at build time for dynamic data is a very common usecase and this is
something that alvu supports.

The network data needs to be fetched from a hook, let's take a simple example.

I need to get downloads for a certain set of packages that've been published to
the NPM Registry.

```lua
-- require the needed packages. 
-- both http and json are injected packages by alvu and 
-- dont need any dependencies from lua to be added
local http = require("http")
local json = require("json")

-- we only want this hook to run for the file 
-- packages.md
ForFile = "packages.md"

local npm_url = "https://api.npmjs.org/downloads/point/last-month/"

-- tiny utility function that takes in 
-- a pkg_name and returns the number of downloads from 
-- the request. 
-- we use the `json` utility to parse the text response
-- into a json
local function get_downloads_for_pkg(pkg_name)
    local response,error_message = http.get(npm_url..pkg_name)
    local body_json = json.decode(response.body)
    return body_json.downloads
end

local packages = {
    "@barelyhuman/tocolor",
    "@barelyhuman/pipe",
    "@barelyhuman/preact-island-plugins",
    "@barelyreaper/themer",
    "jotai-form"
}

function Writer(source_data)
    local source = json.decode(source_data)

    -- create a table with the download count for each package,
    -- mentioned in the `packages` variable
    local downloads_table = {}

    for k,pkg_name in ipairs(packages) do
        local download_count = get_downloads_for_pkg(pkg_name)
        table.insert(downloads_table,{
            title = pkg_name,
            downloads = download_count
        })
    end

    -- Pass down the data to file that's being rendered, in this case `packages.md` 
    return json.encode({
		data = {
			packages = downloads_table,
		},
	})
end
```

Once you have the hook ready, you can now add in the file this is for.
`pages/packages.md`

Which would look, something like this.

```md
# Packages

| name                     | downloads(last 30 days) |
| ------------------------ | ----------------------- |
| {{range .Data.packages}} |                         |
| {{.title}}               | {{.downloads}}          |
| {{end}}                  |                         |
```

Here the `range .Data.packages` loops through the elements that were returned
from the hook on the `data` parameter, and you can now point to the variables
you need to get the required data into the template.

## Templates

The most preferred way of using alvu is to avoid having to construct hooks and
use existing example repositories as the source, this gives us the advantage of
not having to spend time writing similar static site generation logic while
keeping it easy to extend.

### Official Templates

[alvu-foam-template](https://github.com/barelyhuman/alvu-foam-template) - Foam
plugin template for writing a wiki styled website

### Community Templates

Feel free to add in your templates here via PR's on the
[source repo](http://github.com/barelyhuman/alvu) or mailing
[me](mailto:ahoy@barelyhuman.dev)

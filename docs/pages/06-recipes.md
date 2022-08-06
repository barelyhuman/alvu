# Recipes

[Jump to Templates](#templates)

Methods and ways to be able to do basic tasks while working with alvu

## Importing other lua files

You'll need to work with lua files that are in a sibling directory
in the project and you can do so by adding them to the scripts
`package.path` like so

```lua
-- specify that you wish to taken in any `.lua` file in the `lib` folder
package.path = package.path .. ";../lib/?.lua"

-- require lib/utils.lua to use utilities from it
local lib = require("lib.utils")
```

## String Interpolation

There's no way to directly do string interpolation in lua but is almost always needed so here's how you can implement a small helper for it

```lua
local function interpolate(s, tab)
    return (s:gsub('($%b{})', function(w) return tab[w:sub(3, -2)] or w end))
end

-- usage
interpolate("this is a ${message} string", { message = "interpolated" })
```

## String Functions

A helper library is injected into all alvu hook files which can be required into the script to help with basic string manipulation and querying

```lua
local strings = require("strings")
if strings.contains("hello world", "hello")
then
	print("found hello")
end
```

You can read more about these from the [gopher-lua-libs](https://github.com/vadv/gopher-lua-libs/tree/master/strings) repo.

## Get files from a directory

If working with blogs and nested data you might wanna get files in a directory and you can use the following function

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

This can be done with native lua functions but here's a snippet of
the `onFinish` hook from [reaper.is](https://github.com/barelyhuman/reaper.is)' RSS Feed hook

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

## Templates

The most preferred way of using alvu is to avoid having to construct
hooks and use existing example repositories as the source, this gives us the advantage of not having to spend time writing similar static site generation logic while keeping it easy to extend.

### Official Templates

TBD

### Community Templates

Feel free to add in your templates here via PR's on the [source repo](http://codeberg.org/reaper/alvu) or
mailing [me](mailto:ahoy@barelyhuman.dev) if you wish to avoid creating a [codeberg.org](https://codeberg.org) account

# Scripting

The whole point of writing this as a CLI instead of just a simple script to
convert folders of markdown was to be able to add and extend the basic
functionality as needed.

Here's where the `hooks` folder is being used and this part of the documentation
is a quick reference on how this works.

## Language

[lua](https://www.lua.org) is the language of choice, primarily because it's
very easy to add more functionality by just bringing in `.lua` files from the
web and adding them to your project.

Here's what a simple writer function would look like, this function adds a
`Hello World` heading to every html/markdown file that alvu would process.

```lua
-- add-hello-world.lua

-- a special json library injected by alvu
local json = require("json")

function Writer(filedata)
    -- convert the file's information into
    -- a lua accessible table
    local source_data = json.decode(filedata)

    -- concatenate the heading with the rest of the content
    source_data.content = "<h1>Hello World</h1>" .. source_data.content

    -- send back serialized json data to the tool
    return json.encode(source_data)
end
```

While the comments might already help you out, let's just lay them down as
simple points

1. You get data from alvu in the `Writer` function as a serialized/stringified JSON.
2. You can manipulate the data in lua using the helper library `json`
3. Return the data as a serialized JSON, so the modifications can be processed.

You'll learn more about helper libraries as we move ahead.

## Data Injection

There are going to be cases where you might wanna pass back data from the lua
function to the templates and if you wish to build an automatic
`index.html` file that lists all the posts of your blog from a certain folder.

This can be done by passing one of two keys in the returned JSON. `data` or
`extras`

Something like this,

```lua
local json = require("json")

function Writer()
    return json.encode({
        data = {
            name = "reaper"
        }
        -- or
        extras = {
            name = "reaper"
        }
    })
end
```

You don't have to send back the whole `source_content` for each file since it's
only merged when you send something back, else the original is kept as is and
not all keys that you send back are taken into consideration.

If you did send the above data `name = "reaper"` and would like to access it in
all templates, you could just do the following

```html
<p>{{.Data.name}}</p>
<!-- or if you set it under extras -->
<p>{{.Extras.name}}</p>
```

The lua table could nest as much as needed and the same could be accessed in the
templates, but try to avoid it.

and yes, **all templates**, each hook runs on each template and every
manipulation cascades on the other so if you are running a lot of hooks, name
them with numbers (eg: `01-add-navigation.lua` ) if you wish for them to follow a
certain sequence

## Single File Hooks

In most cases you use hooks for common functionality but there's definitely cases
where just one file needs to be processed with hook and that's easily doable with
alvu.

You use the `ForFile` global value in the lua hook and the hook will only run for that
file. An example of this would look something like this.

```lua
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
```

The above only runs for the file `00-readme.md` and is responsible for copying the contents
of the `readme.md` and overwriting the `00-readme.md` file's content with it at **build time**

[More about Writers &rarr; ]({{.Meta.BaseURL}}03-writers)

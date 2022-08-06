# Scripting

The whole point of writing this as a CLI instead of just a simple script to
convert folders of markdown was to be able to add and extend on the basic
functionality as needed.

Here's where the `hooks` folder is being used and this part of the documentation
is a quick reference on how this works.

## Language

[lua](https://www.lua.org) is the language of choice, primarily because it's
very easy to add in more functionality by just bringing in `.lua` files from the
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

While, the comments might already help you out, let's just lay them down as
simple points

1. You get data from alvu in the `Writer` function as a serialized json or
   stringified json.
2. You can manipulate the data in lua using the helper library `json`
3. Return the data as a serialized json as well.

You'll learn more about helper libraries as we move ahead.

## Data Injection

There's going to be cases where you might wanna pass back data from the lua
function to the templates and like if you wish to build an automatic
`index.html` file which lists all the posts of your blog from a certain folder.

This can be done by passing one of two keys in the returned json. `data` or
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

and yes, **all templates** , each hook runs on each template and every
manipulation cascades on the other so if you are running a lot of hooks, name
them with numbers (eg: `01-add-navigation.lua`) if you wish for them to follow a
certain sequence

[More about Writers &rarr;](03-writers)

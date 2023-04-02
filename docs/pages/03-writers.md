# Writer and Hooks

The tool comes with 3 basic hooks

1. OnStart
2. Writer
3. OnFinish

Each of them is a simple lua function and might later move to go plugins if the
need arises or if people would like to be able to talk to alvu in various
languages.

> **Note**: The choice of lua was made because it's easy to pass down new
> utilities to the language but then there's obvious cases where the language
> falls behind. (regex, string manipulations, etc etc)

## `OnStart`

This hook is triggered right before processing the files and it's going to get
called just once per hook file, and as applies with other hook rules, these will
be cascaded, so if you are working with writing and deleting files, please make
sure you order the hooks with file names

## `Writer`

The [Scripting]({{.Meta.BaseURL}}02-scripting) section, covers most of what this writer does but
to reiterate, the `Writer` hooks are called for everyfile in the `pages`
directory and allow you to manipulate the content of the file before it gets
compiled

## `OnFinish`

This hook is triggered right after all the processing as completed and the files
have been compiled. This is primarily for you to be able to run cleanup tasks
but is not limited to that.

[Read the CLI reference &rarr;]({{.Meta.BaseURL}}05-CLI)

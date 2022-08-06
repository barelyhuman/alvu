# Basics

Yeah, so to use `alvu`, you just run it.

```sh
$ alvu
```

Yep, that's it, that's all you do.

Mainly because the tool comes with it's preferred defaults and they are as
follows.

- a directory named `pages`
- a directory named `hooks`
- a directory named `public`

and each one of them represent exactly what they are called.

## Pages

This is simply a collection of files that you're website would be made up of,
these can be of the following formats.

- `.md` - Markdown - Will be converted to HTML
- `.html` - HTML - Will be converted to nothing.
- `.xml` - XML - Will be converted to nothing.

**So, just a markdown processor huh?**

Yeah... and no.

### Special Files

- `_head.html` - will add the header section to the final HTML
- `_tail.html` - will add the footer section to the final HTML

These files basically execute right before the conversion (if necessary) takes
place, and is where you'd put your `<head></head>` related stuff.

Both markdown and html files are template friendly and you can use variables and
any other Go Template functionality in them.

## Hooks

The other side of the CLI was to be able to extend simple functionalities when
needed, and this is where the `hooks` folder comes.

This is going to be a collection of `.lua` files that can each have a `Writer`
function which is given data of the file that's being processed.

So, if I'm processing `index.md` then you'd get the name, path, current content
of the file, which you can return as is or change it and that'll over-ride the
original content in the compiled files.

For an example, you can check this very documentation site's source code

## Public

Pretty self explanatory but the `public` folder will basically copy everything
put into it to the `dist` folder.

Let's move forward to [scripting &rarr;](02-scripting)

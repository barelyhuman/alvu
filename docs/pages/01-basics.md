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

- `_head.html` - will add the header section to the final HTML (deprecated in v0.2.7)
- `_tail.html` - will add the footer section to the final HTML (deprecated in v0.2.7)
- `_layout.html` - defines a common layout for all files that'll be rendered.

The `_head.html` and `_tail.html` files were used as placeholders for
repeated layout across your markdown files, this has now been replaced
by the `_layout.html` file which wraps around your markdown content and
can be defined as shown below

```go-html-template
<!DOCTYPE html>
<html lang="en">
  <head></head>
  <body>
     { { .Content } }
  </body>
</html>
```

`.Content` can be used as a slot or placeholder to be replaced by the content of each markdown file.

> **Note**: Make sure to remove the spaces between the `{` and `}` in the above code snippet, these were added to avoid getting replaced by the template code

We deprecated `_head.html` and `_tail.html` because they would cause abnormalities in the HTML output causing certain element tags to be duplicated. Which isn't semantically correct, also the template execution for these would end up creating arbitrary string nodes at the end of the HTML, which isn't intentional. 

The fix for this would include writing an HTML dedupe handler, which might be a project in itself considering all the edge cases. It was easier to just let golang templates get what they want, hence the introduction of the `_layout.html` file.

## Hooks

The other reason for writing `alvu` was to be able to extend simple functionalities when
needed, and this is where the concept of hooks comes in.

Hooks are a collection of `.lua` files that can each have a `Writer`
function, this function receives the data of the file that's currently being processed.

So, if `alvu` is processing `index.md` then you will get the name, path, its current content, which you can return as is or change it and that'll override the content for the compiled version of the file.

For example, you can check this very documentation site's source code to check how the navigation is added
to each file.

## Public

Pretty self-explanatory but the `public` folder will copy everything
put into it to the `dist` folder. This can be used for assets, styles, etc.

Let's move forward to [scripting &rarr;]({{.Meta.BaseURL}}02-scripting)

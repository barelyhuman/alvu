# CLI Reference

This can also be accessed by using the `-h` flag on the binary. 

```sh
$ alvu -h
```

```
Usage of alvu:
  -baseurl URL
        URL to be used as the root of the project (default "/")
  -hard-wrap <br>
        enable hard wrapping of elements with <br> (default true)
  -highlight
        enable highlighting for markdown files
  -highlight-theme THEME
        THEME to use for highlighting (supports most themes from pygments) (default "bw")
  -hooks DIR
        DIR that contains hooks for the content (default "./hooks")
  -out DIR
        DIR to output the compiled files to (default "./dist")
  -path DIR
        DIR to search for the needed folders in (default ".")
  -port PORT
        PORT to start the server on (default "3000")
  -serve
        start a local server
```

[Check out Recipes &rarr;]({{.Meta.BaseURL}}06-recipes)

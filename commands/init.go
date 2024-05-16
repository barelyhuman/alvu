package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/barelyhuman/alvu/pkg/alvu"
	"github.com/barelyhuman/go/color"
	"github.com/urfave/cli/v2"
)

func AlvuInit(c *cli.Context) (err error) {
	basePath := c.Args().First()
	forceFlag := c.Bool("force")
	logger := alvu.NewLogger()
	logger.LogPrefix = "[alvu]"

	fileInfo, err := os.Stat(basePath)

	if err == nil {
		if fileInfo.IsDir() && !forceFlag {
			logger.Error(fmt.Sprintf("Directory: %v , already exists, cannot overwrite, if you wish to force overwrite use the -f flag with the `init` command", basePath))
			os.Exit(1)
		}
	}

	mustCreateDir(basePath, "public")
	mustCreateDir(basePath, "hooks")
	mustCreateDir(basePath, "pages")
	prepareBaseStyles(basePath)
	preparePages(basePath)

	logger.Success(
		fmt.Sprintf("Alvu initialized in: %v", basePath),
	)

	runStr := color.ColorString{}

	fmt.Println(runStr.Dim("\n> Run the following to get started").String())

	commandStr := color.ColorString{}
	commandStr.Cyan(
		fmt.Sprintf("\n  alvu -s -path %v\n", basePath),
	)
	fmt.Println(commandStr.String())
	return
}

func mustCreateDir(root, dir string) {
	pathToCreate := filepath.Join(root, dir)
	err := os.MkdirAll(pathToCreate, os.ModePerm)
	if err != nil {
		panic(fmt.Sprintf("Failed to create %v due to error: %v\n", pathToCreate, err))
	}
}

func prepareBaseStyles(root string) {
	fileHandle, err := os.OpenFile(filepath.Join(root, "public", "styles.css"), os.O_CREATE|os.O_RDWR, os.ModePerm)
	if err != nil {
		fmt.Printf("Failed to open file public/styles.css with error: %v", err)
	}
	defer fileHandle.Sync()
	defer fileHandle.Close()

	fileHandle.WriteString(`
/* Resets */
html {
	box-sizing: border-box;
	font-size: 16px;
	font-family: -apple-system, BlinkMacSystemFont, avenir next, avenir, segoe ui, helvetica neue, helvetica, Cantarell, Ubuntu, roboto, noto, arial, sans-serif;
}

*, *:before, *:after {
	box-sizing: inherit;
}

body, h1, h2, h3, h4, h5, h6, p {
	margin: 0;
	padding: 0;
	font-weight: normal;
}

img {
	max-width: 100%;
	height: auto;
}

/* Styles */

:root {
	--base: #efefef;
	--text: #181819;
}

@media (prefers-color-scheme: dark) {
	:root {
		--base: #181819;
		--text: #efefef;
	}
}

body {

	background: var(--base);
	color: var(--text);

	max-width: 900px;
	margin: 0 auto;
	padding: 4px;
	display: flex;
	flex-direction: column;
	justify-content: center;
	min-height: 100vh;
  }


  ol,ul,p{
	line-height: 1.7;
  }

	`)

}

func preparePages(root string) {
	layoutHandle, _ := os.OpenFile(filepath.Join(root, "pages", "_layout.html"), os.O_CREATE|os.O_RDWR, os.ModePerm)
	defer layoutHandle.Sync()
	defer layoutHandle.Close()

	rootPageHandle, _ := os.OpenFile(filepath.Join(root, "pages", "index.md"), os.O_CREATE|os.O_RDWR, os.ModePerm)

	defer rootPageHandle.Sync()
	defer rootPageHandle.Close()

	layoutHandle.WriteString(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Alvu | Minimal Starter</title>
	<link href="/styles.css" rel="stylesheet">
</head>
<body>
    <slot></slot>
</body>
</html>`)

	rootPageHandle.WriteString(`# Alvu

- Scriptable
- Fast 
- Tiny 

In whatever order you'd like...

`)

}

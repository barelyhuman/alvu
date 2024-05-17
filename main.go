package main

import (
	_ "embed"
	"fmt"
	"os"
	"time"

	"github.com/barelyhuman/alvu/commands"
	"github.com/urfave/cli/v2"
)

//go:embed .commitlog.release
var version string

const logPrefix string = "[alvu] %v"

func main() {
	app := &cli.App{
		Name:            "alvu",
		Usage:           "A scriptable static site generator",
		CommandNotFound: cli.ShowCommandCompletions,
		Action: func(c *cli.Context) error {
			return commands.Alvu(c)
		},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "hooks",
				Value: "./hooks",
			},
			&cli.StringFlag{
				Name:  "out",
				Value: "./dist",
			},
			&cli.StringFlag{
				Name:  "path",
				Value: ".",
			},
			&cli.StringFlag{
				Name:  "baseurl",
				Value: "/",
			},
			&cli.BoolFlag{
				Name:  "hard-wrap",
				Value: false,
			},
			&cli.BoolFlag{
				Name:  "highlight",
				Value: false,
			},
			&cli.StringFlag{
				Name:  "highlight-theme",
				Value: "bw",
			},
			&cli.BoolFlag{
				Name:    "serve",
				Value:   false,
				Aliases: []string{"s"},
			},
			&cli.IntFlag{
				Name:  "poll",
				Usage: "Define the poll duration in seconds",
				Value: 1000,
			},
			&cli.StringFlag{
				Name:  "env",
				Usage: "Environment File to consider",
				Value: ".env",
			},
			&cli.StringFlag{
				Name:    "port",
				Usage:   "port to use for serving the application",
				Value:   "3000",
				Aliases: []string{"p"},
			},
		},
		Version:     version,
		Compiled:    time.Now(),
		HideVersion: false,
		Commands: []*cli.Command{
			{
				Name:        "init",
				Description: "Initialise a new alvu Project",
				Args:        true,
				ArgsUsage:   "<directory>",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "force",
						Aliases: []string{"f"},
						Usage:   "Force create in the directory even overwriting any files that exist",
					},
				},
				Action: func(ctx *cli.Context) error {
					return commands.AlvuInit(ctx)
				},
			},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		fmt.Fprintf(os.Stderr, logPrefix, err)
	}
}

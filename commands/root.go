package commands

import (
	"github.com/barelyhuman/alvu/pkg/alvu"
	"github.com/urfave/cli/v2"
)

func Alvu(c *cli.Context) (err error) {
	baseConfig := alvu.AlvuConfig{}

	// Basics
	baseConfig.HookDir = c.String("hooks")
	baseConfig.OutDir = c.String("out")
	baseConfig.RootPath = c.String("path")

	// Transformation Config
	baseConfig.BaseURL = c.String("baseurl")
	baseConfig.EnableHardWrap = c.Bool("hard-wrap")
	baseConfig.EnableHighlighting = c.Bool("highlight")
	baseConfig.HighlightingTheme = c.String("highlight-theme")

	// Serve config
	baseConfig.Serve = c.Bool("serve")
	baseConfig.PollDuration = c.Int("poll")
	baseConfig.PortNumber = c.String("port")

	return baseConfig.Run()
}

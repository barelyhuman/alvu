package alvu

import (
	"fmt"

	"github.com/barelyhuman/go/color"
)

type Logger struct {
	logPrefix string
}

func (l *Logger) Success(msg string) {
	cs := color.ColorString{}
	cs.Gray(l.logPrefix).Reset(" ").Green("✔").Reset(" ").Green(msg)
	fmt.Println(cs.String())
}

func (l *Logger) Info(msg string) {
	cs := color.ColorString{}
	cs.Gray(l.logPrefix).Reset(" ").Cyan("ℹ").Reset(" ").Cyan(msg)
	fmt.Println(cs.String())
}

func (l *Logger) Warning(msg string) {
	cs := color.ColorString{}
	cs.Gray(l.logPrefix).Reset(" ").Yellow(msg)
	fmt.Println(cs.String())
}

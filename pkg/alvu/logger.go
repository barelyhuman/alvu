package alvu

import (
	"fmt"

	"github.com/barelyhuman/go/color"
	"github.com/barelyhuman/go/env"
)

type Logger struct {
	logPrefix string
}

func (l *Logger) Debug(msg string) {
	env := env.Get("DEBUG", "false")
	if env == "false" {
		return
	}
	cs := color.ColorString{}
	cs.Gray(l.logPrefix).Reset(" ").Green("✔").Reset(" ").Green(msg)
	fmt.Println(cs.String())
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

func (l *Logger) Error(msg string) {
	cs := color.ColorString{}
	cs.Gray(l.logPrefix).Reset(" ").Red(msg)
	fmt.Println(cs.String())
}

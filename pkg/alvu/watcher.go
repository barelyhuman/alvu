package alvu

import (
	"fmt"
	"os"

	"github.com/barelyhuman/go/poller"
)

type Watcher struct {
	poller    *poller.Poller
	logger    Logger
	recompile chan string
}

type HookFn func(path string)

func NewWatcher() *Watcher {
	return &Watcher{
		poller:    poller.NewPollWatcher(2000),
		recompile: make(chan string, 1),
	}
}

func (p *Watcher) AddDir(path string) {
	p.poller.Add(path)
}

func (p *Watcher) Start() {
	go p.poller.Start()
	go func() {
		for {
			select {
			case evt := <-p.poller.Events:
				_, err := os.Stat(evt.Path)

				p.logger.Debug(fmt.Sprintf("Change Event: %v", evt.Path))

				// Do nothing if the file doesn't exit, just continue
				if err != nil {
					if os.IsNotExist(err) {
						continue
					}
					p.logger.Error(err.Error())
				}

				p.recompile <- evt.Path
				continue
			case err := <-p.poller.Errors:
				p.logger.Error(err.Error())
			}
		}
	}()
}

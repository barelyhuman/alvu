package main

import (
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"time"
)

type PollerEvent struct {
	path string
}

type Poller struct {
	intervalInSeconds int
	directories       []string
	files             []string
	fileModsTimes     map[string]int64
	Events            chan PollerEvent
}

func NewPollWatcher() *Poller {
	poller := &Poller{
		Events:        make(chan PollerEvent, 1),
		fileModsTimes: map[string]int64{},
	}
	return poller
}

func (pw *Poller) Add(dir string) {
	filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if d.IsDir() && dir != path {
			pw.directories = append(pw.directories, path)
			pw.Add(path)
		} else {
			pw.files = append(pw.files, path)
			fInfo, err := d.Info()
			if err != nil {
				log.Println(err)
			}
			// Initial mod times
			pw.fileModsTimes[path] = fInfo.ModTime().UnixMilli()
		}
		return nil
	})
}

func (pw *Poller) StartPoller() chan struct{} {
	ticker := time.NewTicker(time.Duration(pw.intervalInSeconds) * time.Second)
	quit := make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				for _, filePath := range pw.files {
					oldFileTime := pw.fileModsTimes[filePath]
					fileInfo, err := os.Stat(filePath)
					if err != nil {
						continue
					}
					if fileInfo.ModTime().UnixMilli() != oldFileTime {
						pw.Events <- PollerEvent{
							path: filePath,
						}
						pw.fileModsTimes[filePath] = fileInfo.ModTime().UnixMilli()
						break
					}
				}
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}()

	return quit
}

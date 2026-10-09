package main

import (
	"fmt"
	"os"
	"os/exec"
	"time"
)

type result struct {
	name string
	err  error
}

func supervise(commands []*exec.Cmd, names []string, signals <-chan os.Signal) error {
	events := make(chan result, len(commands))
	started := []*exec.Cmd{}
	defer func() {
		for _, cmd := range started {
			killGroup(cmd)
		}
	}()
	var startErr error
	for i, cmd := range commands {
		prepareGroup(cmd)
		if err := cmd.Start(); err != nil {
			startErr = fmt.Errorf("start %s: %w", names[i], err)
			break
		}
		started = append(started, cmd)
		name := names[i]
		go func() { events <- result{name, cmd.Wait()} }()
	}
	remaining := len(started)
	var err = startErr
	if startErr == nil {
		select {
		case <-signals:
		case event := <-events:
			remaining--
			if event.err != nil {
				err = fmt.Errorf("%s exited: %w", event.name, event.err)
			}
		}
	}
	for _, cmd := range started {
		stopGroup(cmd)
	}
	timeout := time.NewTimer(3 * time.Second)
	defer timeout.Stop()
	for remaining > 0 {
		select {
		case <-events:
			remaining--
		case <-timeout.C:
			for _, cmd := range started {
				killGroup(cmd)
			}
			for remaining > 0 {
				<-events
				remaining--
			}
		}
	}
	return err
}

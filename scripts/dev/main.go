// Command dev starts the Go API and Next.js development server together.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/rajibdpi/trynas/config"
)

func main() {
	if err := dev(); err != nil {
		fmt.Fprintln(os.Stderr, "dev:", err)
		os.Exit(1)
	}
}

func command(dir, name string, args ...string) *exec.Cmd {
	if runtime.GOOS == "windows" && name == "pnpm" {
		args = append([]string{"/d", "/s", "/c", "pnpm"}, args...)
		name = "cmd.exe"
	}
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = os.Environ()
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd
}

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

func dev() error {
	// Configuration flags and precedence match the standalone backend.
	if err := config.Load(); err != nil {
		return err
	}
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		return fmt.Errorf("run from the TryNAS project root: %w", err)
	}
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	frontend := filepath.Join(root, "frontend")
	if _, err := os.Stat(filepath.Join(frontend, "node_modules")); os.IsNotExist(err) {
		if err := command(frontend, "pnpm", "install", "--frozen-lockfile").Run(); err != nil {
			return err
		}
	}
	dir, err := os.MkdirTemp("", "trynas-dev-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	binary := filepath.Join(dir, "trynas")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	fmt.Println("Compiling Go backend...")
	if err := command(root, "go", "build", "-o", binary, ".").Run(); err != nil {
		return err
	}
	backend := command(root, binary, os.Args[1:]...)
	ui := command(frontend, "pnpm", "dev", "--port", "3000")
	// Always use the development API, even if a production export was built in
	// the same shell. Leave frontend/.env.local untouched.
	ui.Env = append(ui.Env, "NEXT_PUBLIC_EMBEDDED=false", "TRYNAS_EMBEDDED_BUILD=false")
	fmt.Println("Starting development servers. Frontend: http://localhost:3000; backend: ADDR or 127.0.0.1:8080. Ctrl+C stops both.")
	return supervise([]*exec.Cmd{backend, ui}, []string{"backend", "frontend"}, signals)
}

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

	"github.com/rajibdpi/trynas/config"
	"github.com/rajibdpi/trynas/scripts/internal/tooling"
)

func main() {
	if err := dev(); err != nil {
		fmt.Fprintln(os.Stderr, "dev:", err)
		os.Exit(1)
	}
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
		if err := tooling.Command(frontend, "pnpm", "install", "--frozen-lockfile").Run(); err != nil {
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
	if err := tooling.Command(root, "go", "build", "-o", binary, ".").Run(); err != nil {
		return err
	}
	backend := tooling.Command(root, binary, os.Args[1:]...)
	ui := tooling.Command(frontend, "pnpm", "dev", "--port", "3000")
	// Always use the development API, even if a production export was built in
	// the same shell. Leave frontend/.env.local untouched.
	ui.Env = append(ui.Env, "NEXT_PUBLIC_EMBEDDED=false", "TRYNAS_EMBEDDED_BUILD=false")
	fmt.Println("Starting development servers. Frontend: http://localhost:3000; backend: ADDR or 127.0.0.1:8080. Ctrl+C stops both.")
	return supervise([]*exec.Cmd{backend, ui}, []string{"backend", "frontend"}, signals)
}

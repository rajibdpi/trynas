// Package tooling contains process setup shared by developer commands.
package tooling

import (
	"os"
	"os/exec"
	"runtime"
)

// Command connects a process to the terminal and inherits the environment.
// pnpm is a Windows command shim, so it needs cmd.exe on that platform.
func Command(dir, name string, args ...string) *exec.Cmd {
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

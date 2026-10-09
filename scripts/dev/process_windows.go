package main

import (
	"os/exec"
	"strconv"
)

func prepareGroup(cmd *exec.Cmd) {}
func stopGroup(cmd *exec.Cmd)    { killGroup(cmd) }
func killGroup(cmd *exec.Cmd) {
	_ = exec.Command("taskkill", "/PID", strconv.Itoa(cmd.Process.Pid), "/T", "/F").Run()
}

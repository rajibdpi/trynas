package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestHelperProcess(t *testing.T) {
	switch os.Getenv("TRYNAS_DEV_TEST_HELPER") {
	case "fail":
		os.Exit(3)
	case "wait":
		time.Sleep(time.Minute)
		os.Exit(0)
	}
}

func helper(mode string) *exec.Cmd {
	cmd := exec.Command(os.Args[0], "-test.run=^TestHelperProcess$")
	cmd.Env = append(os.Environ(), "TRYNAS_DEV_TEST_HELPER="+mode)
	return cmd
}

func TestSupervisorStopsPeerOnFailure(t *testing.T) {
	first, second := helper("fail"), helper("wait")
	err := supervise([]*exec.Cmd{first, second}, []string{"backend", "frontend"}, make(chan os.Signal))
	if err == nil || !strings.Contains(err.Error(), "backend exited") {
		t.Fatalf("expected backend failure, got %v", err)
	}
	if first.ProcessState == nil || second.ProcessState == nil {
		t.Fatal("children were not reaped")
	}
}

func TestSupervisorStopsOnInterrupt(t *testing.T) {
	signals := make(chan os.Signal, 1)
	signals <- os.Interrupt
	first, second := helper("wait"), helper("wait")
	if err := supervise([]*exec.Cmd{first, second}, []string{"backend", "frontend"}, signals); err != nil {
		t.Fatal(err)
	}
	if first.ProcessState == nil || second.ProcessState == nil {
		t.Fatal("children were not reaped")
	}
}

func TestSupervisorCleansUpOnStartFailure(t *testing.T) {
	first := helper("wait")
	bad := exec.Command("/nonexistent/trynas-dev-command")
	err := supervise([]*exec.Cmd{first, bad}, []string{"backend", "frontend"}, make(chan os.Signal))
	if err == nil || !strings.Contains(err.Error(), "start frontend") {
		t.Fatalf("expected startup error, got %v", err)
	}
	if first.ProcessState == nil {
		t.Fatal("backend was not reaped")
	}
}

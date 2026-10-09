package main

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func TestPowerShellInstaller(t *testing.T) {
	shell, err := exec.LookPath("pwsh")
	if err != nil {
		shell, err = exec.LookPath("powershell")
	}
	if err != nil {
		t.Skip("PowerShell is not installed; run powershell_test.ps1 on Windows")
	}
	path, err := filepath.Abs("powershell_test.ps1")
	if err != nil {
		t.Fatal(err)
	}
	output, err := exec.Command(shell, "-NoProfile", "-File", path).CombinedOutput()
	if err != nil {
		t.Fatalf("PowerShell installer tests: %v\n%s", err, output)
	}
}

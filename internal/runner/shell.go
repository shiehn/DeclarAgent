package runner

import (
	"bytes"
	"os/exec"
)

// ShellResult holds the output of a shell command.
type ShellResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// Run executes a command via sh -c and captures output.
func Run(command, workDir string) *ShellResult {
	cmd := exec.Command("sh", "-c", command)
	if workDir != "" {
		cmd.Dir = workDir
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}

	return &ShellResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
	}
}

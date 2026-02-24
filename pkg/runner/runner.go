package runner

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/healrun/healrun/pkg/types"
)

const (
	BufferSize = 16384
)

// RunCommand executes a command and streams output in real-time
func RunCommand(command string) (*types.CommandResult, error) {
	return RunCommandInDir(command, "")
}

// RunCommandInDir executes a command in a specific directory
func RunCommandInDir(command string, dir string) (*types.CommandResult, error) {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}

	cmd := exec.Command(shell, "-c", command)
	if dir != "" {
		cmd.Dir = dir
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start command: %w", err)
	}

	var stdoutBuf, stderrBuf strings.Builder
	var wg sync.WaitGroup

	streamOutput := func(reader io.Reader, writer io.Writer, buffer *strings.Builder) {
		defer wg.Done()
		scanner := bufio.NewScanner(reader)
		buf := make([]byte, BufferSize)
		scanner.Buffer(buf, BufferSize)
		for scanner.Scan() {
			line := scanner.Text() + "\n"
			fmt.Fprint(writer, line)
			buffer.WriteString(line)
		}
	}

	wg.Add(2)
	go streamOutput(stdout, os.Stdout, &stdoutBuf)
	go streamOutput(stderr, os.Stderr, &stderrBuf)

	err = cmd.Wait()
	wg.Wait()

	exitCode := 0
	if err != nil {
		exitCode = 1
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
	}

	return &types.CommandResult{
		Stdout:   stdoutBuf.String(),
		Stderr:   stderrBuf.String(),
		ExitCode: exitCode,
		Success:  exitCode == 0,
	}, nil
}

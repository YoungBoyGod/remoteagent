package runtime

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"syscall"
	"time"
)

type commandResult struct {
	ExitCode  int
	Stdout    string
	Stderr    string
	Truncated bool
}

func runCommand(parent context.Context, command string, timeout time.Duration) (commandResult, error) {
	return runCommandWithType(parent, "shell", command, timeout)
}

func runCommandWithType(parent context.Context, taskType string, command string, timeout time.Duration) (commandResult, error) {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()

	var cmd *exec.Cmd
	switch taskType {
	case "python":
		cmd = exec.Command("python3", "-c", command)
	default: // "shell" or anything else
		cmd = exec.Command("sh", "-c", command)
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	stdout := newLimitedBuffer(maxCommandOutputBytes)
	stderr := newLimitedBuffer(maxCommandOutputBytes)
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Start(); err != nil {
		return commandResult{ExitCode: -1, Stderr: err.Error()}, err
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	var waitErr error
	select {
	case waitErr = <-done:
	case <-ctx.Done():
		killProcessGroup(cmd)
		waitErr = <-done
	}

	exitCode := 0
	if waitErr != nil {
		exitCode = extractExitCode(waitErr)
	}
	result := commandResult{
		ExitCode:  exitCode,
		Stdout:    stdout.String(),
		Stderr:    stderr.String(),
		Truncated: stdout.Truncated() || stderr.Truncated(),
	}

	if ctx.Err() != nil {
		return result, ctx.Err()
	}
	return result, waitErr
}

func killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err == nil {
		_ = syscall.Kill(-pgid, syscall.SIGKILL)
		return
	}
	_ = cmd.Process.Kill()
}

func extractExitCode(err error) int {
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return -1
	}
	if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
		return status.ExitStatus()
	}
	return -1
}

type limitedBuffer struct {
	max       int
	buf       bytes.Buffer
	truncated bool
}

func newLimitedBuffer(max int) *limitedBuffer {
	return &limitedBuffer{max: max}
}

func (buffer *limitedBuffer) Write(payload []byte) (int, error) {
	if buffer.max <= 0 {
		buffer.truncated = true
		return len(payload), nil
	}
	if buffer.buf.Len() >= buffer.max {
		buffer.truncated = true
		return len(payload), nil
	}
	remaining := buffer.max - buffer.buf.Len()
	if len(payload) > remaining {
		_, _ = buffer.buf.Write(payload[:remaining])
		buffer.truncated = true
		return len(payload), nil
	}
	_, _ = buffer.buf.Write(payload)
	return len(payload), nil
}

func (buffer *limitedBuffer) String() string {
	return buffer.buf.String()
}

func (buffer *limitedBuffer) Truncated() bool {
	return buffer.truncated
}

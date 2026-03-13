package executor

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
)

// EmitFn is called for each output line. isErr = true means stderr.
type EmitFn func(line string, isErr bool) error

// RunScript executes a bash script and streams output line-by-line via emit.
// Returns the exit code and any execution error.
func RunScript(scriptPath string, ctx context.Context, emit EmitFn) (int, error) {
	if _, err := os.Stat(scriptPath); err != nil {
		return 1, fmt.Errorf("script not found: %s", scriptPath)
	}

	cmd := exec.CommandContext(ctx, "bash", scriptPath)
	cmd.Env = os.Environ()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return 1, fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return 1, fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return 1, fmt.Errorf("start script: %w", err)
	}

	done := make(chan struct{})
	errc := make(chan error, 2)

	streamLines := func(r io.Reader, isErr bool) {
		defer func() { done <- struct{}{} }()
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			if err := emit(scanner.Text(), isErr); err != nil {
				errc <- err
				return
			}
		}
	}

	go streamLines(stdout, false)
	go streamLines(stderr, true)

	// wait for both goroutines
	<-done
	<-done

	waitErr := cmd.Wait()

	select {
	case streamErr := <-errc:
		return exitCode(waitErr), streamErr
	default:
	}

	return exitCode(waitErr), nil
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode()
	}
	return 1
}

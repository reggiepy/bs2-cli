// Package supervisor gives blocking native calls a bounded, owned process lifetime.
package supervisor

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Heartbeat marks a worker operation boundary, filtered out of user diagnostics.
const Heartbeat = "\x1eBS2_PROGRESS\n"

type progressWriter struct {
	out     io.Writer
	pending string
	pulse   chan struct{}
}

func (w *progressWriter) Write(p []byte) (int, error) {
	w.pending += string(p)
	for {
		at := strings.IndexByte(w.pending, '\n')
		if at < 0 {
			break
		}
		line := w.pending[:at+1]
		w.pending = w.pending[at+1:]
		if line == Heartbeat {
			select {
			case w.pulse <- struct{}{}:
			default:
			}
		} else {
			if _, err := io.WriteString(w.out, line); err != nil {
				return 0, err
			}
		}
	}
	if len(w.pending) > 65536 {
		if _, err := io.WriteString(w.out, w.pending); err != nil {
			return 0, err
		}
		w.pending = ""
	}
	return len(p), nil
}

// Run launches a worker. Cancellation closes stdin, giving the worker a chance
// to restore state before forced termination. For sessions timeout is idle time.
func Run(ctx context.Context, exe string, args []string, timeout, grace time.Duration, session bool, stdout, stderr io.Writer) error {
	cmd := exec.Command(exe, args...)
	for _, v := range os.Environ() {
		if !strings.HasPrefix(strings.ToUpper(v), "BS2_INTERNAL_WORKER=") {
			cmd.Env = append(cmd.Env, v)
		}
	}
	cmd.Env = append(cmd.Env, "BS2_INTERNAL_WORKER=1")
	pulses := make(chan struct{}, 1)
	diagnostics := &progressWriter{out: stderr, pulse: pulses}
	cmd.Stdout = stdout
	cmd.Stderr = diagnostics
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	if err = cmd.Start(); err != nil {
		return errors.Join(err, stdin.Close())
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	var reason error
running:
	for {
		select {
		case err = <-done:
			closeErr := stdin.Close()
			if errors.Is(closeErr, os.ErrClosed) {
				closeErr = nil
			}
			_, flushErr := io.WriteString(stderr, diagnostics.pending)
			return errors.Join(err, closeErr, flushErr)
		case <-pulses:
			if session {
				timer.Reset(timeout)
			}
		case <-ctx.Done():
			reason = ctx.Err()
			break running
		case <-timer.C:
			reason = fmt.Errorf("worker exceeded %s operation timeout", timeout)
			break running
		}
	}
	closeErr := stdin.Close()
	if errors.Is(closeErr, os.ErrClosed) {
		closeErr = nil
	}
	graceTimer := time.NewTimer(grace)
	defer graceTimer.Stop()
	select {
	case err = <-done:
		_, flushErr := io.WriteString(stderr, diagnostics.pending)
		if errors.Is(reason, context.Canceled) && err == nil {
			return errors.Join(closeErr, flushErr)
		}
		return errors.Join(reason, err, closeErr, flushErr)
	case <-graceTimer.C:
		killErr := cmd.Process.Kill()
		if errors.Is(killErr, os.ErrProcessDone) {
			killErr = nil
		}
		waitErr := <-done
		return errors.Join(reason, fmt.Errorf("worker could not stop in %s; restoration is not confirmed", grace), killErr, waitErr, closeErr)
	}
}

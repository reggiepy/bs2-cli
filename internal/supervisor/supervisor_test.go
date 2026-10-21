package supervisor

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"testing"
	"time"
)

func TestWorker(t *testing.T) {
	if os.Getenv("BS2_INTERNAL_WORKER") != "1" {
		return
	}
	switch os.Getenv("BS2_TEST_MODE") {
	case "hang":
		time.Sleep(10 * time.Second)
	case "cooperate":
		fmt.Fprint(os.Stderr, Heartbeat)
		if _, err := io.Copy(io.Discard, os.Stdin); err != nil {
			os.Exit(2)
		}
		if _, err := fmt.Fprintln(os.Stdout, "restored"); err != nil {
			os.Exit(2)
		}
	case "heartbeat":
		for range 8 {
			fmt.Fprint(os.Stderr, Heartbeat)
			time.Sleep(50 * time.Millisecond)
		}
	}
	os.Exit(0)
}
func TestCooperativeCancellation(t *testing.T) {
	t.Setenv("BS2_TEST_MODE", "cooperate")
	t.Setenv("GORACE", "atexit_sleep_ms=0")
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	timer := time.AfterFunc(300*time.Millisecond, cancel)
	defer timer.Stop()
	defer cancel()
	var out, logs bytes.Buffer
	err = Run(ctx, exe, []string{"-test.run=^TestWorker$"}, 3*time.Second, 2*time.Second, true, &out, &logs)
	if err != nil || out.String() != "restored\n" || logs.Len() != 0 {
		t.Fatalf("%v out=%q logs=%q", err, out.String(), logs.String())
	}
}
func TestWatchdog(t *testing.T) {
	t.Setenv("BS2_TEST_MODE", "hang")
	t.Setenv("GORACE", "atexit_sleep_ms=0")
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	var out, logs bytes.Buffer
	started := time.Now()
	err = Run(context.Background(), exe, []string{"-test.run=^TestWorker$"}, 300*time.Millisecond, 100*time.Millisecond, true, &out, &logs)
	if err == nil || time.Since(started) > 3*time.Second {
		t.Fatalf("watchdog: %v", err)
	}
}
func TestProgressWriterFragments(t *testing.T) {
	var out bytes.Buffer
	w := progressWriter{out: &out, pulse: make(chan struct{}, 1)}
	for _, s := range []string{Heartbeat[:3], Heartbeat[3:], "hello\n"} {
		if _, err := w.Write([]byte(s)); err != nil {
			t.Fatal(err)
		}
	}
	if out.String() != "hello\n" || len(w.pulse) != 1 {
		t.Fatalf("%q", out.String())
	}
}

func TestHeartbeatExtendsSession(t *testing.T) {
	t.Setenv("BS2_TEST_MODE", "heartbeat")
	t.Setenv("GORACE", "atexit_sleep_ms=0")
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	var out, logs bytes.Buffer
	err = Run(context.Background(), exe, []string{"-test.run=^TestWorker$"}, 300*time.Millisecond, time.Second, true, &out, &logs)
	if err != nil {
		t.Fatal(err)
	}
}

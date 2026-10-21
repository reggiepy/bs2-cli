// Command bs2 controls a Flydigi BS2 over Windows Bluetooth LE.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"time"

	"bs2-cli/internal/ble"
	"bs2-cli/internal/device"
	"bs2-cli/internal/protocol"
)

const usage = `bs2 — experimental Flydigi BS2 BLE CLI (Windows)

bs2 scan   [--duration 8s]
bs2 devices
bs2 status --address AA:BB:CC:DD:EE:FF
bs2 set    --address AA:BB:CC:DD:EE:FF --rpm 1800 [--hold 8s]
bs2 preset --address AA:BB:CC:DD:EE:FF --number 1

Common flags: --timeout 30s (hard process deadline), --debug (hex packets).
set enters software control; persistence after disconnect is firmware dependent.
No automatic temperature control or USB support in this prototype.
`

type options struct {
	command, address  string
	rpm, preset       int
	timeout, duration time.Duration
	hold              time.Duration
	debug             bool
}

func parse(args []string) (options, error) {
	var o options
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" {
		return o, flag.ErrHelp
	}
	o.command = args[0]
	if o.command != "devices" && o.command != "scan" && o.command != "status" && o.command != "set" && o.command != "preset" {
		return o, fmt.Errorf("unknown command %q", o.command)
	}
	f := flag.NewFlagSet(o.command, flag.ContinueOnError)
	f.StringVar(&o.address, "address", "", "BLE address")
	f.IntVar(&o.rpm, "rpm", 0, "target RPM")
	f.IntVar(&o.preset, "number", 0, "preset number")
	f.DurationVar(&o.timeout, "timeout", 30*time.Second, "hard deadline")
	f.DurationVar(&o.duration, "duration", 8*time.Second, "scan duration")
	f.DurationVar(&o.hold, "hold", 0, "monitor after setting speed")
	f.BoolVar(&o.debug, "debug", false, "log packets")
	if err := f.Parse(args[1:]); err != nil {
		return o, err
	}
	if f.NArg() != 0 {
		return o, fmt.Errorf("unexpected positional arguments")
	}
	if o.timeout <= 0 || (o.command == "scan" && (o.duration <= 0 || o.duration >= o.timeout)) {
		return o, fmt.Errorf("require 0 < duration < timeout")
	}
	if o.command == "set" && (o.hold < 0 || o.hold+5*time.Second >= o.timeout) {
		return o, fmt.Errorf("hold must be nonnegative and leave at least 5s before timeout")
	}
	if o.command != "devices" && o.command != "scan" && o.address == "" {
		return o, fmt.Errorf("--address is required (use bs2 scan)")
	}
	if o.command == "set" {
		if _, err := protocol.SpeedPayload(o.rpm); err != nil {
			return o, err
		}
	}
	if o.command == "preset" && (o.preset < 1 || o.preset > 4) {
		return o, fmt.Errorf("--number must be 1..4")
	}
	return o, nil
}

func output(v any) error { e := json.NewEncoder(os.Stdout); e.SetIndent("", "  "); return e.Encode(v) }

func worker(ctx context.Context, o options) (err error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if o.command == "devices" {
		devices, e := ble.Devices(ctx)
		if e != nil {
			return e
		}
		return output(devices)
	}
	if o.command == "scan" {
		scanCtx, cancel := context.WithTimeout(ctx, o.duration)
		defer cancel()
		ads, e := ble.Scan(scanCtx)
		if e != nil {
			return e
		}
		return output(ads)
	}
	link, err := ble.Open(ctx, o.address, o.debug)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, link.Close()) }()
	client := device.Client{Link: link}
	var s device.Status
	switch o.command {
	case "status":
		s, err = client.ReadStatus(ctx)
	case "set":
		s, err = client.SetSpeed(ctx, o.rpm)
		if err == nil && o.hold > 0 {
			until := time.Now().Add(o.hold)
			for time.Now().Before(until) {
				timer := time.NewTimer(time.Second)
				select {
				case <-ctx.Done():
					timer.Stop()
					return ctx.Err()
				case <-timer.C:
				}
				s, err = client.ReadStatus(ctx)
				if err != nil {
					break
				}
				if int(s.TargetRPM) != o.rpm {
					return fmt.Errorf("target changed during hold: %d", s.TargetRPM)
				}
				if o.debug {
					fmt.Fprintf(os.Stderr, "observed current=%d target=%d mode=%d\n", s.CurrentRPM, s.TargetRPM, s.Mode)
				}
			}
		}
	case "preset":
		s, err = client.SetPreset(ctx, o.preset)
	}
	if err != nil {
		return err
	}
	return output(s)
}

func run() error {
	o, err := parse(os.Args[1:])
	if errors.Is(err, flag.ErrHelp) {
		fmt.Print(usage)
		return nil
	}
	if err != nil {
		return err
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	ctx, deadline := context.WithTimeout(ctx, o.timeout)
	defer deadline()
	if os.Getenv("BS2_INTERNAL_WORKER") == "1" {
		return worker(ctx, o)
	}
	// WinRT APIs in the BLE dependency may block without a context-aware cancel.
	// Isolate them in an owned child so the deadline terminates only our worker.
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, exe, os.Args[1:]...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	for _, v := range os.Environ() {
		if !strings.HasPrefix(strings.ToUpper(v), "BS2_INTERNAL_WORKER=") {
			cmd.Env = append(cmd.Env, v)
		}
	}
	cmd.Env = append(cmd.Env, "BS2_INTERNAL_WORKER=1")
	err = cmd.Run()
	if ctx.Err() != nil {
		return fmt.Errorf("operation ended: %w (device state may have changed if this was a write)", ctx.Err())
	}
	return err
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "bs2:", err)
		os.Exit(1)
	}
}

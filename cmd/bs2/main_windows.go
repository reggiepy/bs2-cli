// Command bs2 controls a Flydigi BS2 over Windows Bluetooth LE.
package main

import (
	"bs2-cli/internal/ble"
	"bs2-cli/internal/config"
	"bs2-cli/internal/device"
	"bs2-cli/internal/protocol"
	"bs2-cli/internal/supervisor"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime"
	"time"
)

const usage = `bs2 — Flydigi BS2 BLE CLI (Windows)

bs2 devices
bs2 levels
bs2 scan    [--duration 8s]
bs2 config  [--address AA:BB:CC:DD:EE:FF]
bs2 status
bs2 watch   [--interval 1s] [--duration 30s]
bs2 set     --rpm 2800 [--hold 8s]
bs2 control --rpm 2800 [--duration 30s] [--restore=false]
bs2 control --level 7 [--duration 30s]
bs2 preset  --number 3
bs2 option  --name gear-led --value on
bs2 option  --name power-on --value on
bs2 option  --name smart-stop --value off

Common flags: --address MAC, --config FILE, --timeout 30s, --debug.
Address priority: explicit flag, saved configuration, single Windows-known BS2.
watch/control emit JSON lines. Duration 0 means until Ctrl+C.
control restores its initial mode/target on normal exit or Ctrl+C by default.
External control changes stop control without overwriting the new setting.
timeout bounds an operation in watch/control, the whole command otherwise.
USB and temperature-based automatic control are not implemented.
`

type options struct {
	command, address, configPath, name, value string
	rpm, preset                               int
	level                                     int
	timeout, duration, hold, interval         time.Duration
	debug, restore                            bool
}

func (o options) session() bool { return o.command == "watch" || o.command == "control" }

func parse(args []string) (options, error) {
	var o options
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" {
		return o, flag.ErrHelp
	}
	o.command = args[0]
	switch o.command {
	case "levels", "devices", "scan", "config", "status", "watch", "set", "control", "preset", "option":
	default:
		return o, fmt.Errorf("unknown command %q", o.command)
	}
	path, err := config.DefaultPath()
	if err != nil {
		return o, err
	}
	f := flag.NewFlagSet(o.command, flag.ContinueOnError)
	f.StringVar(&o.address, "address", "", "BLE address")
	f.StringVar(&o.configPath, "config", path, "configuration file")
	f.StringVar(&o.name, "name", "", "setting name")
	f.StringVar(&o.value, "value", "", "setting value")
	f.IntVar(&o.rpm, "rpm", 0, "target RPM")
	f.IntVar(&o.level, "level", 0, "native UI level 1..9 (fixed RPM equivalent)")
	f.IntVar(&o.preset, "number", 0, "preset number")
	f.DurationVar(&o.timeout, "timeout", 30*time.Second, "operation deadline")
	duration := time.Duration(0)
	if o.command == "scan" {
		duration = 8 * time.Second
	}
	f.DurationVar(&o.duration, "duration", duration, "session duration; zero runs until canceled")
	f.DurationVar(&o.interval, "interval", time.Second, "poll interval")
	f.DurationVar(&o.hold, "hold", 0, "monitor after one-shot setting")
	f.BoolVar(&o.debug, "debug", false, "log packets")
	f.BoolVar(&o.restore, "restore", true, "restore initial state after control")
	if err = f.Parse(args[1:]); err != nil {
		return o, err
	}
	if f.NArg() != 0 {
		return o, fmt.Errorf("unexpected positional arguments")
	}
	if o.configPath == "" {
		return o, fmt.Errorf("config path must not be empty")
	}
	if o.address != "" {
		o.address, err = config.NormalizeAddress(o.address)
		if err != nil {
			return o, err
		}
	}
	if o.timeout <= 0 || o.duration < 0 {
		return o, fmt.Errorf("timeout must be positive; duration must be nonnegative")
	}
	if o.command == "scan" && (o.duration <= 0 || o.duration >= o.timeout) {
		return o, fmt.Errorf("scan requires 0 < duration < timeout")
	}
	if o.session() && (o.interval < 500*time.Millisecond || o.interval > 10*time.Second || o.timeout < 15*time.Second) {
		return o, fmt.Errorf("sessions require interval 500ms..10s and timeout >=15s")
	}
	if o.command == "set" && (o.hold < 0 || o.hold >= o.timeout-5*time.Second) {
		return o, fmt.Errorf("hold must leave at least 5s before timeout")
	}
	if o.command == "set" || o.command == "control" {
		if o.level != 0 {
			if o.rpm != 0 {
				return o, fmt.Errorf("choose either --rpm or --level")
			}
			o.rpm, err = device.LevelRPM(o.level)
			if err != nil {
				return o, err
			}
		}
		if _, err = protocol.SpeedPayload(o.rpm); err != nil {
			return o, err
		}
	}
	if o.command == "preset" && (o.preset < 1 || o.preset > 3) {
		return o, fmt.Errorf("BS2 --number must be 1..3")
	}
	if o.command == "option" {
		valid := map[string]map[string]bool{"gear-led": {"on": true, "off": true}, "power-on": {"on": true, "off": true}, "smart-stop": {"off": true, "instant": true, "delay": true}}
		if !valid[o.name][o.value] {
			return o, fmt.Errorf("invalid option name/value (see bs2 help)")
		}
	}
	return o, nil
}
func output(v any) error { e := json.NewEncoder(os.Stdout); e.SetIndent("", "  "); return e.Encode(v) }

func resolveAddress(ctx context.Context, o options) (string, error) {
	if o.address != "" {
		return o.address, nil
	}
	settings, err := config.Load(o.configPath)
	if err != nil {
		return "", err
	}
	if settings.Address != "" {
		return settings.Address, nil
	}
	known, err := ble.Devices(ctx)
	if err != nil {
		return "", err
	}
	if len(known) != 1 {
		return "", fmt.Errorf("found %d Windows-known BS2 devices; use --address or bs2 config --address", len(known))
	}
	return known[0].Address, nil
}

func worker(ctx context.Context, o options) (err error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if o.command == "levels" {
		return output(device.Levels())
	}
	if o.command == "config" {
		if o.address != "" {
			if err = config.Save(o.configPath, config.Settings{Address: o.address}); err != nil {
				return err
			}
		}
		settings, e := config.Load(o.configPath)
		if e != nil {
			return e
		}
		return output(struct {
			Path     string          `json:"path"`
			Settings config.Settings `json:"settings"`
		}{o.configPath, settings})
	}
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
	address, err := resolveAddress(ctx, o)
	if err != nil {
		return err
	}
	unlock, err := ble.Lock(address)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, unlock()) }()
	link, err := ble.Open(ctx, address, o.debug)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, link.Close()) }()
	client := device.Client{Link: link}
	if o.session() {
		rpm := 0
		if o.command == "control" {
			rpm = o.rpm
		}
		encoder := json.NewEncoder(os.Stdout)
		return device.Monitor(ctx, client, device.SessionOptions{RPM: rpm, Interval: o.interval, Duration: o.duration, Restore: o.restore,
			Progress: func() { fmt.Fprint(os.Stderr, supervisor.Heartbeat) }, CheckExclusive: ble.CheckExclusive}, func(e device.Event) error { return encoder.Encode(e) })
	}
	if o.command == "option" {
		result, e := client.SetOption(ctx, o.name, o.value)
		if e != nil {
			return e
		}
		return output(result)
	}
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
				if int(s.TargetRPM) != o.rpm || s.Mode != 5 {
					return fmt.Errorf("device control changed: mode=%d target=%d", s.Mode, s.TargetRPM)
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
	if os.Getenv("BS2_INTERNAL_WORKER") == "1" {
		// Parent closes the stop pipe on cancellation, including in a hidden console.
		go func() {
			_, e := io.Copy(io.Discard, os.Stdin)
			if e != nil && !errors.Is(e, os.ErrClosed) {
				fmt.Fprintln(os.Stderr, "stop pipe:", e)
			}
			cancel()
		}()
		return worker(ctx, o)
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	return supervisor.Run(ctx, exe, os.Args[1:], o.timeout, 12*time.Second, o.session(), os.Stdout, os.Stderr)
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "bs2:", err)
		os.Exit(1)
	}
}

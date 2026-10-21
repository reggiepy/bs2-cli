package device

import (
	"context"
	"errors"
	"testing"
	"time"
)

type sessionFake struct {
	state                    Status
	writeError, restoreError error
	external                 bool
	writes, restores         int
}

func (f *sessionFake) ReadStatus(context.Context) (Status, error) {
	if f.external && f.writes > 0 {
		return Status{Mode: 2, TargetRPM: 2000}, nil
	}
	return f.state, nil
}
func (f *sessionFake) SetSpeed(_ context.Context, rpm int) (Status, error) {
	f.writes++
	f.state = Status{Mode: 5, TargetRPM: uint16(rpm)}
	return f.state, f.writeError
}
func (f *sessionFake) SetPreset(_ context.Context, n int) (Status, error) {
	f.restores++
	f.state = Status{Mode: byte(n), TargetRPM: 3000}
	return f.state, f.restoreError
}
func TestMonitorRestore(t *testing.T) {
	for _, tt := range []struct {
		name                 string
		writeErr, restoreErr error
		external             bool
		wantRestores         int
		wantErr              bool
	}{
		{"duration", nil, nil, false, 1, false},
		{"partial write", errors.New("partial write"), nil, false, 1, true},
		{"restore failure", nil, errors.New("disconnected"), false, 1, true},
		{"external setting", nil, nil, true, 0, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := &sessionFake{state: Status{Mode: 3, TargetRPM: 3000}, writeError: tt.writeErr, restoreError: tt.restoreErr, external: tt.external}
			var events []Event
			err := Monitor(context.Background(), f, SessionOptions{RPM: 2800, Interval: time.Millisecond, Duration: 8 * time.Millisecond, Restore: true}, func(e Event) error { events = append(events, e); return nil })
			if (err != nil) != tt.wantErr || f.restores != tt.wantRestores {
				t.Fatalf("restores=%d err=%v", f.restores, err)
			}
			if !tt.wantErr && events[len(events)-1].Event != "restored" {
				t.Fatal("no restoration confirmation")
			}
		})
	}
}
func TestMonitorCancelAndReadOnly(t *testing.T) {
	for _, rpm := range []int{0, 2800} {
		ctx, cancel := context.WithCancel(context.Background())
		f := &sessionFake{state: Status{Mode: 3, TargetRPM: 3000}}
		err := Monitor(ctx, f, SessionOptions{RPM: rpm, Interval: time.Millisecond, Restore: true}, func(e Event) error {
			if e.Event == "status" {
				cancel()
			}
			return nil
		})
		cancel()
		if err != nil {
			t.Fatal(err)
		}
		if rpm == 0 && (f.writes != 0 || f.restores != 0) {
			t.Fatal("watch mutated device")
		}
		if rpm != 0 && f.restores != 1 {
			t.Fatal("cancel did not restore")
		}
	}
}

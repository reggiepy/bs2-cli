package device

import (
	"bs2-cli/internal/protocol"
	"context"
	"errors"
	"fmt"
	"time"
)

// Controller is the set of operations required by a monitored control session.
type Controller interface {
	ReadStatus(context.Context) (Status, error)
	SetSpeed(context.Context, int) (Status, error)
	SetPreset(context.Context, int) (Status, error)
}

// Event describes a readback or restoration. Watch/control emit JSON lines.
type Event struct {
	Event  string    `json:"event"`
	Time   time.Time `json:"time"`
	Status Status    `json:"status"`
}

// SessionOptions controls monitoring and restoration. Duration zero means until canceled.
type SessionOptions struct {
	RPM                int
	Interval, Duration time.Duration
	Restore            bool
	Progress           func()
	CheckExclusive     func() error
}

func emitStatus(emit func(Event) error, kind string, s Status) error {
	return emit(Event{kind, time.Now(), s})
}

// Monitor observes the device or maintains a verified fixed target. It stops on
// externally changed settings rather than fighting another controller or a user.
// Restoration is attempted even after a partially completed initial write.
func Monitor(ctx context.Context, c Controller, o SessionOptions, emit func(Event) error) (err error) {
	if o.Interval <= 0 || o.Duration < 0 {
		return fmt.Errorf("invalid session timing")
	}
	if o.RPM != 0 {
		if _, err = protocol.SpeedPayload(o.RPM); err != nil {
			return err
		}
	}
	pulse := func() {
		if o.Progress != nil {
			o.Progress()
		}
	}
	pulse()
	before, err := c.ReadStatus(ctx)
	if err != nil {
		return err
	}
	if err = emitStatus(emit, "initial", before); err != nil {
		return err
	}
	changedExternally := false
	if o.RPM != 0 {
		if o.Restore {
			if before.Mode < 1 || before.Mode > 5 {
				return fmt.Errorf("cannot restore unknown mode %d", before.Mode)
			}
			if before.Mode == 5 {
				if _, e := protocol.SpeedPayload(int(before.TargetRPM)); e != nil {
					return fmt.Errorf("cannot restore software target: %w", e)
				}
			}
			defer func() {
				if changedExternally {
					return
				}
				pulse()
				if o.CheckExclusive != nil {
					if e := o.CheckExclusive(); e != nil {
						err = errors.Join(err, fmt.Errorf("restore skipped: %w", e))
						return
					}
				}
				restoreCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				var restored Status
				var e error
				if before.Mode == 5 {
					restored, e = c.SetSpeed(restoreCtx, int(before.TargetRPM))
				} else {
					restored, e = c.SetPreset(restoreCtx, int(before.Mode))
				}
				if e == nil && (restored.Mode != before.Mode || restored.TargetRPM != before.TargetRPM) {
					e = fmt.Errorf("restore mismatch: expected mode %d target %d; got mode %d target %d", before.Mode, before.TargetRPM, restored.Mode, restored.TargetRPM)
				}
				if e != nil {
					err = errors.Join(err, fmt.Errorf("restore failed: %w", e))
					return
				}
				err = errors.Join(err, emitStatus(emit, "restored", restored))
			}()
		}
		pulse()
		state, e := c.SetSpeed(ctx, o.RPM)
		if e != nil {
			if ctx.Err() != nil {
				return nil
			}
			return e
		}
		if e = emitStatus(emit, "controlled", state); e != nil {
			return e
		}
	}
	if o.Duration > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, o.Duration)
		defer cancel()
	}
	for {
		pulse()
		timer := time.NewTimer(o.Interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-timer.C:
		}
		if o.CheckExclusive != nil {
			if e := o.CheckExclusive(); e != nil {
				changedExternally = true
				return e
			}
		}
		pulse()
		s, e := c.ReadStatus(ctx)
		if e != nil {
			if ctx.Err() != nil {
				return nil
			}
			return e
		}
		if e = emitStatus(emit, "status", s); e != nil {
			return e
		}
		if o.RPM != 0 && (int(s.TargetRPM) != o.RPM || s.Mode != 5) {
			changedExternally = true
			return fmt.Errorf("device control changed externally (mode=%d target=%d); stopped without overwriting the new setting", s.Mode, s.TargetRPM)
		}
	}
}

// Package device provides sequential BS2 request/response operations.
package device

import (
	"bs2-cli/internal/protocol"
	"context"
	"fmt"
	"time"
)

// Transport sends a request and waits for its matching notification.
type Transport interface {
	Exchange(context.Context, byte, []byte) ([]byte, error)
}

// Client controls a single BS2. It is not safe for concurrent use.
type Client struct{ Link Transport }

// Status is a snapshot read from the device, not a cached desired state.
type Status struct {
	Firmware   string `json:"firmware_hex"`
	Mode       byte   `json:"mode"`
	ModeName   string `json:"mode_name"`
	CurrentRPM uint16 `json:"current_rpm"`
	TargetRPM  uint16 `json:"target_rpm"`
}

// ReadStatus queries firmware, mode and current/target RPM.
func (c Client) ReadStatus(ctx context.Context) (Status, error) {
	var s Status
	firmware, err := c.Link.Exchange(ctx, 1, nil)
	if err != nil {
		return s, err
	}
	if len(firmware) != 4 {
		return s, fmt.Errorf("firmware response: expected 4 bytes, got %d", len(firmware))
	}
	s.Firmware = fmt.Sprintf("% X", firmware)
	mode, err := c.Link.Exchange(ctx, 0x25, nil)
	if err != nil {
		return s, err
	}
	if len(mode) != 1 || mode[0] > 5 {
		return s, fmt.Errorf("unexpected mode payload: %x", mode)
	}
	s.Mode = mode[0]
	s.ModeName = []string{"unknown-0", "preset-1", "preset-2", "preset-3", "preset-4", "software-control"}[s.Mode]
	speed, err := c.Link.Exchange(ctx, 0x22, nil)
	if err != nil {
		return s, err
	}
	s.CurrentRPM, s.TargetRPM, err = protocol.Speeds(speed)
	return s, err
}

func (c Client) ack(ctx context.Context, cmd byte, payload []byte, allowThree bool) error {
	reply, err := c.Link.Exchange(ctx, cmd, payload)
	if err != nil {
		return err
	}
	if len(reply) != 1 || (reply[0] != 1 && (!allowThree || reply[0] != 3)) {
		return fmt.Errorf("command %02x rejected: %x", cmd, reply)
	}
	return nil
}

// SetSpeed enters software control and sets a fixed target RPM. Readback is required.
func (c Client) SetSpeed(ctx context.Context, rpm int) (Status, error) {
	payload, err := protocol.SpeedPayload(rpm)
	if err != nil {
		return Status{}, err
	}
	if err = c.ack(ctx, 0x23, nil, true); err != nil {
		return Status{}, err
	}
	if err = c.ack(ctx, 0x21, payload, false); err != nil {
		return Status{}, fmt.Errorf("software mode entered but speed write failed: %w", err)
	}
	timer := time.NewTimer(1500 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return Status{}, ctx.Err()
	case <-timer.C:
	}
	s, err := c.ReadStatus(ctx)
	if err != nil {
		return s, err
	}
	if int(s.TargetRPM) != rpm || s.Mode != 5 {
		return s, fmt.Errorf("control readback mismatch: requested software mode at %d RPM; device reports mode %d target %d", rpm, s.Mode, s.TargetRPM)
	}
	return s, nil
}

// SetPreset returns control to a physical preset (1..4).
func (c Client) SetPreset(ctx context.Context, preset int) (Status, error) {
	if preset < 1 || preset > 4 {
		return Status{}, fmt.Errorf("preset must be 1..4")
	}
	if err := c.ack(ctx, 8, []byte{byte(preset)}, false); err != nil {
		return Status{}, err
	}
	s, err := c.ReadStatus(ctx)
	if err == nil && int(s.Mode) != preset {
		return s, fmt.Errorf("preset readback mismatch: got mode %d", s.Mode)
	}
	return s, err
}

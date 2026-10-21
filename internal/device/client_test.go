package device

import (
	"context"
	"errors"
	"testing"
)

type fakeLink struct {
	commands []byte
	replies  map[byte][]byte
	fail     byte
}

func (f *fakeLink) Exchange(ctx context.Context, cmd byte, _ []byte) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f.commands = append(f.commands, cmd)
	if cmd == f.fail {
		return nil, errors.New("transport failed")
	}
	return f.replies[cmd], nil
}
func TestReadStatus(t *testing.T) {
	for _, tt := range []struct {
		name      string
		mode      []byte
		speed     []byte
		fail      byte
		wantError bool
	}{
		{"valid", []byte{5}, []byte{0x06, 7, 8, 7}, 0, false},
		{"mode missing", nil, []byte{0, 0, 0, 0}, 0, true},
		{"unknown mode", []byte{255}, []byte{0, 0, 0, 0}, 0, true},
		{"speed truncated", []byte{4}, []byte{0}, 0, true},
		{"transport failure", []byte{4}, []byte{0, 0, 0, 0}, 0x22, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := &fakeLink{replies: map[byte][]byte{1: {1, 0, 0, 1}, 0x25: tt.mode, 0x22: tt.speed}, fail: tt.fail}
			s, err := (Client{f}).ReadStatus(context.Background())
			if (err != nil) != tt.wantError {
				t.Fatalf("status %+v error %v", s, err)
			}
			if !tt.wantError && (s.CurrentRPM != 1798 || s.TargetRPM != 1800 || s.ModeName != "software-control") {
				t.Fatalf("%+v", s)
			}
		})
	}
}
func TestRejectedStartNeverWritesSpeed(t *testing.T) {
	f := &fakeLink{replies: map[byte][]byte{0x23: {0}}}
	if _, err := (Client{f}).SetSpeed(context.Background(), 1800); err == nil {
		t.Fatal("accepted rejection")
	}
	if len(f.commands) != 1 || f.commands[0] != 0x23 {
		t.Fatalf("wrote speed after rejected start: %x", f.commands)
	}
}
func TestInvalidSpeedNeverTouchesDevice(t *testing.T) {
	f := &fakeLink{}
	if _, err := (Client{f}).SetSpeed(context.Background(), 50000); err == nil {
		t.Fatal("accepted invalid speed")
	}
	if len(f.commands) != 0 {
		t.Fatal("invalid speed touched device")
	}
}

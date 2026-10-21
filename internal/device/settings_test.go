package device

import (
	"context"
	"testing"
)

type optionLink struct {
	cmd   byte
	data  []byte
	reply []byte
	calls int
}

func (l *optionLink) Exchange(_ context.Context, cmd byte, p []byte) ([]byte, error) {
	l.cmd = cmd
	l.data = append([]byte(nil), p...)
	l.calls++
	return l.reply, nil
}
func TestSettingsEncoding(t *testing.T) {
	for _, tt := range []struct {
		name, value string
		cmd, data   byte
	}{
		{"gear-led", "on", 0x48, 1}, {"gear-led", "off", 0x48, 0},
		{"power-on", "on", 0x0c, 1}, {"power-on", "off", 0x0c, 2},
		{"smart-stop", "off", 0x0d, 0}, {"smart-stop", "instant", 0x0d, 1}, {"smart-stop", "delay", 0x0d, 2},
	} {
		t.Run(tt.name+"/"+tt.value, func(t *testing.T) {
			l := &optionLink{reply: []byte{1}}
			r, err := (Client{l}).SetOption(context.Background(), tt.name, tt.value)
			if err != nil || l.cmd != tt.cmd || len(l.data) != 1 || l.data[0] != tt.data || !r.Acknowledged || r.ReadbackVerified {
				t.Fatalf("%+v %+v %v", l, r, err)
			}
		})
	}
}
func TestSettingsReject(t *testing.T) {
	l := &optionLink{reply: []byte{0}}
	if _, err := (Client{l}).SetOption(context.Background(), "power-on", "on"); err == nil {
		t.Fatal("accepted NACK")
	}
	l.calls = 0
	if _, err := (Client{l}).SetOption(context.Background(), "power-on", "true"); err == nil || l.calls != 0 {
		t.Fatal("invalid value sent")
	}
}
func TestLevels(t *testing.T) {
	for _, tt := range []struct{ level, rpm int }{{1, 1300}, {2, 1700}, {4, 2100}, {7, 2800}, {9, 3000}} {
		rpm, err := LevelRPM(tt.level)
		if err != nil || rpm != tt.rpm {
			t.Fatalf("%+v got %d %v", tt, rpm, err)
		}
	}
	for _, n := range []int{0, 10, -1} {
		if _, err := LevelRPM(n); err == nil {
			t.Fatalf("accepted level %d", n)
		}
	}
}

package protocol

import (
	"bytes"
	"encoding/hex"
	"testing"
)

func TestVectors(t *testing.T) {
	for _, tt := range []struct {
		name    string
		cmd     byte
		payload []byte
		hex     string
	}{
		{"firmware", 1, nil, "5aa5010203"},
		{"speed query", 0x22, nil, "5aa5220224"},
		{"enter smart", 0x23, nil, "5aa5230225"},
		{"1800 RPM", 0x21, []byte{0x08, 0x07}, "5aa52104080734"},
		{"response current 1798 target 1800", 0x22, []byte{0x06, 0x07, 0x08, 0x07}, "5aa522060607080744"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Encode(tt.cmd, tt.payload)
			if err != nil {
				t.Fatal(err)
			}
			if hex.EncodeToString(got) != tt.hex {
				t.Fatalf("got %x want %s", got, tt.hex)
			}
			cmd, p, err := Decode(got)
			if err != nil || cmd != tt.cmd || !bytes.Equal(p, tt.payload) {
				t.Fatalf("decode: %x %x %v", cmd, p, err)
			}
		})
	}
}

func TestRejectInvalidFrames(t *testing.T) {
	for _, s := range []string{"", "5aa5", "00a5220224", "5aa5220124", "5aa5220225", "5aa522022400", "5aa52206"} {
		t.Run(s, func(t *testing.T) {
			b, err := hex.DecodeString(s)
			if err != nil {
				t.Fatal(err)
			}
			if _, _, err = Decode(b); err == nil {
				t.Fatal("accepted malformed frame")
			}
		})
	}
	if _, err := Encode(1, make([]byte, 254)); err == nil {
		t.Fatal("accepted oversized payload")
	}
}

func TestSpeedsAndBounds(t *testing.T) {
	current, target, err := Speeds([]byte{0x06, 0x07, 0x08, 0x07})
	if err != nil || current != 1798 || target != 1800 {
		t.Fatalf("%d %d %v", current, target, err)
	}
	if _, _, err = Speeds([]byte{1}); err == nil {
		t.Fatal("accepted short speed reply")
	}
	for _, rpm := range []int{-1, 0, 1299, 3001, 65536} {
		if _, err := SpeedPayload(rpm); err == nil {
			t.Fatalf("accepted %d", rpm)
		}
	}
	for _, rpm := range []int{1300, 1800, 3000} {
		b, err := SpeedPayload(rpm)
		if err != nil || int(b[0])+int(b[1])*256 != rpm {
			t.Fatalf("%d: %x %v", rpm, b, err)
		}
	}
}

func FuzzDecode(f *testing.F) {
	f.Add([]byte{0x5a, 0xa5, 0x22, 2, 0x24})
	f.Fuzz(func(t *testing.T, b []byte) {
		cmd, p, err := Decode(b)
		if err != nil {
			return
		}
		encoded, err := Encode(cmd, p)
		if err != nil || !bytes.Equal(encoded, b) {
			t.Fatalf("invalid accepted frame %x", b)
		}
	})
}

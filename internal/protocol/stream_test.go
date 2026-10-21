package protocol

import (
	"bytes"
	"testing"
)

func TestStreamAllSplits(t *testing.T) {
	// Captured BS2 firmware response followed by a mode response.
	raw := []byte{0x5a, 0xa5, 1, 6, 0, 0, 3, 5, 15, 0x5a, 0xa5, 0x25, 3, 4, 0x2c}
	for split := 0; split <= len(raw); split++ {
		var s Stream
		a, err := s.Feed(raw[:split])
		if err != nil {
			t.Fatal(err)
		}
		b, err := s.Feed(raw[split:])
		if err != nil {
			t.Fatal(err)
		}
		frames := append(a, b...)
		if len(frames) != 2 || frames[1].Command != 0x25 || !bytes.Equal(frames[0].Payload, []byte{0, 0, 3, 5}) {
			t.Fatalf("split %d: %+v", split, frames)
		}
	}
}
func TestStreamSubscriptionMidFrame(t *testing.T) {
	var s Stream
	frames, err := s.Feed([]byte{0xa5, 9, 3, 255, 11, 0x5a})
	if err != nil || len(frames) != 0 {
		t.Fatalf("%v %v", frames, err)
	}
	frames, err = s.Feed([]byte{0xa5, 0x25, 3, 4, 0x2c})
	if err != nil || len(frames) != 1 {
		t.Fatalf("%v %v", frames, err)
	}
}

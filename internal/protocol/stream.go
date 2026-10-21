package protocol

import "bytes"

// Frame is one validated response from a notification stream.
type Frame struct {
	Command byte
	Payload []byte
}

// Stream reassembles fragmented and coalesced BLE notifications across requests.
// Bytes preceding the first header are discarded, allowing subscription mid-stream.
type Stream struct{ pending []byte }

// Feed consumes a notification and returns every complete validated frame.
func (s *Stream) Feed(data []byte) ([]Frame, error) {
	s.pending = append(s.pending, data...)
	var result []Frame
	for {
		at := bytes.Index(s.pending, []byte{0x5a, 0xa5})
		if at < 0 {
			if len(s.pending) > 0 && s.pending[len(s.pending)-1] == 0x5a {
				s.pending = []byte{0x5a}
			} else {
				s.pending = nil
			}
			return result, nil
		}
		s.pending = s.pending[at:]
		if len(s.pending) < 4 {
			return result, nil
		}
		size := int(s.pending[3]) + 3
		if size < 5 {
			s.pending = s.pending[2:]
			continue
		}
		if len(s.pending) < size {
			return result, nil
		}
		cmd, payload, err := Decode(s.pending[:size])
		s.pending = s.pending[size:]
		if err != nil {
			return result, err
		}
		result = append(result, Frame{cmd, payload})
	}
}

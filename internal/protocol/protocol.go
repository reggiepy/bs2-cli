// Package protocol implements the BS2 frame format recovered from the installed controller.
package protocol

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// Encode creates a frame. The checksum is an additive sum, not a polynomial CRC.
func Encode(command byte, payload []byte) ([]byte, error) {
	if len(payload) > 253 {
		return nil, errors.New("payload exceeds one-byte length")
	}
	frame := append([]byte{0x5a, 0xa5, command, byte(len(payload) + 2)}, payload...)
	var sum byte
	for _, b := range frame[2:] {
		sum += b
	}
	return append(frame, sum), nil
}

// Decode validates one complete frame and returns a copy of its payload.
func Decode(frame []byte) (byte, []byte, error) {
	if len(frame) < 5 || frame[0] != 0x5a || frame[1] != 0xa5 {
		return 0, nil, errors.New("invalid BS2 header")
	}
	if frame[3] < 2 || len(frame) != int(frame[3])+3 {
		return 0, nil, errors.New("invalid BS2 frame length")
	}
	var sum byte
	for _, b := range frame[2 : len(frame)-1] {
		sum += b
	}
	if sum != frame[len(frame)-1] {
		return 0, nil, errors.New("invalid BS2 checksum")
	}
	return frame[2], append([]byte(nil), frame[4:len(frame)-1]...), nil
}

// Speeds decodes the current and target RPM values from a query response.
func Speeds(payload []byte) (uint16, uint16, error) {
	if len(payload) != 4 {
		return 0, 0, fmt.Errorf("speed response: expected 4 bytes, got %d", len(payload))
	}
	return binary.LittleEndian.Uint16(payload), binary.LittleEndian.Uint16(payload[2:]), nil
}

// SpeedPayload restricts this prototype to the BS2's documented-in-code 1300–3000 RPM range.
func SpeedPayload(rpm int) ([]byte, error) {
	if rpm < 1300 || rpm > 3000 {
		return nil, errors.New("this BS2 prototype accepts 1300..3000 RPM")
	}
	return []byte{byte(rpm), byte(rpm >> 8)}, nil
}

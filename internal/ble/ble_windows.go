// Package ble implements the Windows BLE transport without Flydigi DLLs.
package ble

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"bs2-cli/internal/protocol"
	"tinygo.org/x/bluetooth"
)

// Link owns a BLE connection. All methods must be called sequentially on the COM thread.
type Link struct {
	device   bluetooth.Device
	write    bluetooth.DeviceCharacteristic
	notify   bluetooth.DeviceCharacteristic
	incoming chan []byte
	debug    bool
	stream   protocol.Stream
}

// Open connects to an explicit BLE address and subscribes to BS2 responses.
// The CLI also imposes a process deadline because the upstream WinRT calls lack cancellation.
func Open(ctx context.Context, address string, debug bool) (*Link, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := CheckExclusive(); err != nil {
		return nil, err
	}
	mac, err := bluetooth.ParseMAC(address)
	if err != nil {
		return nil, fmt.Errorf("address: %w", err)
	}
	adapter := bluetooth.DefaultAdapter
	if err = adapter.Enable(); err != nil {
		return nil, fmt.Errorf("enable Bluetooth: %w", err)
	}
	dev, err := adapter.Connect(bluetooth.Address{MACAddress: bluetooth.MACAddress{MAC: mac}}, bluetooth.ConnectionParams{})
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	l := &Link{device: dev, incoming: make(chan []byte, 64), debug: debug}
	fail := func(cause error) (*Link, error) { return nil, errors.Join(cause, dev.Disconnect()) }
	services, err := dev.DiscoverServices([]bluetooth.UUID{bluetooth.New16BitUUID(0xfff0)})
	if err != nil {
		return fail(fmt.Errorf("discover FFF0: %w", err))
	}
	chars, err := services[0].DiscoverCharacteristics([]bluetooth.UUID{bluetooth.New16BitUUID(0xfff2), bluetooth.New16BitUUID(0xfff1)})
	// Windows may expose an empty characteristic list briefly after reconnecting.
	// Retry only discovery, never an already-issued device command.
	for attempt := 0; err != nil && attempt < 2; attempt++ {
		timer := time.NewTimer(500 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return fail(ctx.Err())
		case <-timer.C:
		}
		chars, err = services[0].DiscoverCharacteristics([]bluetooth.UUID{bluetooth.New16BitUUID(0xfff2), bluetooth.New16BitUUID(0xfff1)})
	}
	if err != nil {
		return fail(fmt.Errorf("discover FFF2/FFF1: %w", err))
	}
	l.write, l.notify = chars[0], chars[1]
	err = l.notify.EnableNotifications(func(b []byte) {
		copyOf := append([]byte(nil), b...)
		select {
		case l.incoming <- copyOf:
		default: /* Bounded queue: a missing reply becomes an explicit timeout. */
		}
	})
	if err != nil {
		return fail(fmt.Errorf("subscribe FFF1: %w", err))
	}
	return l, nil
}

// Close removes the subscription and releases the connection.
func (l *Link) Close() error {
	return errors.Join(l.notify.EnableNotifications(nil), l.device.Disconnect())
}

// Exchange sends one command and requires a validated matching protocol response.
func (l *Link) Exchange(ctx context.Context, cmd byte, payload []byte) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	frame, err := protocol.Encode(cmd, payload)
	if err != nil {
		return nil, err
	}
	// Discard old notifications before issuing a new request.
drain:
	for {
		select {
		case data := <-l.incoming:
			if _, err := l.stream.Feed(data); err != nil {
				return nil, err
			}
		default:
			break drain
		}
	}
	if l.debug {
		fmt.Fprintf(os.Stderr, "TX % X\n", frame)
	}
	var n int
	if l.write.Properties()&8 != 0 {
		n, err = l.write.Write(frame)
	} else {
		n, err = l.write.WriteWithoutResponse(frame)
	}
	if err != nil {
		return nil, fmt.Errorf("write %02x: %w", cmd, err)
	}
	if n != len(frame) {
		return nil, io.ErrShortWrite
	}
	deadline := time.NewTimer(3 * time.Second)
	defer deadline.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-deadline.C:
			return nil, fmt.Errorf("command %02x: notification timeout", cmd)
		case data := <-l.incoming:
			if l.debug {
				fmt.Fprintf(os.Stderr, "RX % X\n", data)
			}
			frames, err := l.stream.Feed(data)
			if err != nil {
				return nil, err
			}
			for _, frame := range frames {
				if frame.Command == cmd {
					return frame.Payload, nil
				}
			}
		}
	}
}

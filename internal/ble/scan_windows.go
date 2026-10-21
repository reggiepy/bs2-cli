package ble

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"unsafe"

	"github.com/go-ole/go-ole"
	"github.com/saltosystems/winrt-go"
	"github.com/saltosystems/winrt-go/windows/devices/bluetooth/advertisement"
	"github.com/saltosystems/winrt-go/windows/foundation"
)

// Advertisement identifies a BS2 discovered over the air.
type Advertisement struct {
	Address string `json:"address"`
	Name    string `json:"name"`
	RSSI    int16  `json:"rssi"`
}

// Scan collects BS2 advertisements until ctx expires. It does not connect or write settings.
func Scan(ctx context.Context) (results []Advertisement, err error) {
	if err = ole.RoInitialize(1); err != nil {
		return nil, err
	}
	watcher, err := advertisement.NewBluetoothLEAdvertisementWatcher()
	if err != nil {
		return nil, err
	}
	defer watcher.Release()
	if err = watcher.SetScanningMode(advertisement.BluetoothLEScanningModeActive); err != nil {
		return nil, err
	}
	var mu sync.Mutex
	found := map[string]Advertisement{}
	guid := winrt.ParameterizedInstanceGUID(foundation.GUIDTypedEventHandler, advertisement.SignatureBluetoothLEAdvertisementWatcher, advertisement.SignatureBluetoothLEAdvertisementReceivedEventArgs)
	handler := foundation.NewTypedEventHandler(ole.NewGUID(guid), func(_ *foundation.TypedEventHandler, _, arg unsafe.Pointer) {
		args := (*advertisement.BluetoothLEAdvertisementReceivedEventArgs)(arg)
		ad, e := args.GetAdvertisement()
		if e != nil {
			return
		}
		defer ad.Release()
		name, e := ad.GetLocalName()
		if e != nil || !strings.EqualFold(name, "FlyDigi BS2") {
			return
		}
		addr, e := args.GetBluetoothAddress()
		if e != nil {
			return
		}
		rssi, e := args.GetRawSignalStrengthInDBm()
		if e != nil {
			return
		}
		address := fmt.Sprintf("%02X:%02X:%02X:%02X:%02X:%02X", byte(addr>>40), byte(addr>>32), byte(addr>>24), byte(addr>>16), byte(addr>>8), byte(addr))
		mu.Lock()
		found[address] = Advertisement{address, name, rssi}
		mu.Unlock()
	})
	defer handler.Release()
	token, err := watcher.AddReceived(handler)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, watcher.RemoveReceived(token)) }()
	if err = watcher.Start(); err != nil {
		return nil, err
	}
	<-ctx.Done()
	if err = watcher.Stop(); err != nil {
		return nil, err
	}
	mu.Lock()
	defer mu.Unlock()
	results = make([]Advertisement, 0, len(found))
	for _, ad := range found {
		results = append(results, ad)
	}
	if errors.Is(ctx.Err(), context.Canceled) {
		return results, ctx.Err()
	}
	return results, nil
}

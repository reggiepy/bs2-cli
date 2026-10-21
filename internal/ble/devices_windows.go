package ble

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// KnownDevice is a Windows device record; it does not imply current connectivity.
type KnownDevice struct {
	Address string `json:"address"`
	Name    string `json:"name"`
	Source  string `json:"source"`
}

// Devices lists Windows-known BS2 devices even when they are not advertising.
func Devices(ctx context.Context) (result []KnownDevice, err error) {
	result = []KnownDevice{}
	root, err := registry.OpenKey(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Enum\BTHLE`, registry.READ)
	if errors.Is(err, registry.ErrNotExist) {
		return result, nil
	}
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, root.Close()) }()
	names, err := root.ReadSubKeyNames(-1)
	if err != nil {
		return nil, err
	}
	for _, name := range names {
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		if !strings.HasPrefix(strings.ToUpper(name), "DEV_") || len(name) != 16 {
			continue
		}
		key, e := registry.OpenKey(root, name, registry.READ)
		if e != nil {
			continue
		}
		children, e := key.ReadSubKeyNames(-1)
		closeErr := key.Close()
		if e != nil || closeErr != nil {
			return nil, errors.Join(e, closeErr)
		}
		for _, child := range children {
			instance, e := registry.OpenKey(root, name+`\`+child, registry.READ)
			if e != nil {
				continue
			}
			friendly, _, readErr := instance.GetStringValue("FriendlyName")
			if e = instance.Close(); e != nil {
				return nil, e
			}
			if readErr != nil || !strings.EqualFold(friendly, "FlyDigi BS2") {
				continue
			}
			mac := strings.ToUpper(name[4:])
			address := fmt.Sprintf("%s:%s:%s:%s:%s:%s", mac[:2], mac[2:4], mac[4:6], mac[6:8], mac[8:10], mac[10:12])
			result = append(result, KnownDevice{address, friendly, "windows-device-record"})
			break
		}
	}
	return result, nil
}

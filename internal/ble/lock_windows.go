package ble

import (
	"errors"
	"fmt"
	"golang.org/x/sys/windows"
	"strings"
)

// Lock prevents concurrent bs2 workers from using the same device in this session.
func Lock(address string) (func() error, error) {
	name, err := windows.UTF16PtrFromString(`Local\BS2CLI-` + strings.ReplaceAll(address, ":", ""))
	if err != nil {
		return nil, err
	}
	h, err := windows.CreateMutex(nil, false, name)
	if errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		return nil, errors.Join(fmt.Errorf("another bs2 command is using %s", address), windows.CloseHandle(h))
	}
	if err != nil {
		return nil, err
	}
	return func() error { return windows.CloseHandle(h) }, nil
}

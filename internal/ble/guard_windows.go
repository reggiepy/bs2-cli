package ble

import (
	"errors"
	"fmt"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// CheckExclusive rejects an active Space Station process before talking to BS2.
// Another controller started later cannot be excluded by this initial check.
func CheckExclusive() (err error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, windows.CloseHandle(snapshot)) }()
	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	err = windows.Process32First(snapshot, &entry)
	for err == nil {
		if strings.EqualFold(windows.UTF16ToString(entry.ExeFile[:]), "FlydigiSpaceStation.exe") {
			return fmt.Errorf("FlydigiSpaceStation is running (PID %d); exit it from the system tray before using bs2", entry.ProcessID)
		}
		err = windows.Process32Next(snapshot, &entry)
	}
	if errors.Is(err, windows.ERROR_NO_MORE_FILES) {
		return nil
	}
	return err
}

// Package config stores the selected BS2 independently of the executable location.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
)

// Settings contains user preferences. Only the device address is persisted.
type Settings struct {
	Address string `json:"address"`
}

// DefaultPath returns the per-user configuration filename.
func DefaultPath() (string, error) {
	p, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(p, "bs2-cli", "config.json"), nil
}

// NormalizeAddress validates a Bluetooth address and normalizes its spelling.
func NormalizeAddress(s string) (string, error) {
	a, err := net.ParseMAC(s)
	if err != nil || len(a) != 6 {
		return "", fmt.Errorf("invalid Bluetooth address %q", s)
	}
	return strings.ToUpper(a.String()), nil
}

// Load reads settings; a missing file represents an unset default.
func Load(path string) (Settings, error) {
	var s Settings
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	if err = json.Unmarshal(b, &s); err != nil {
		return s, fmt.Errorf("read config %s: %w", path, err)
	}
	if s.Address != "" {
		s.Address, err = NormalizeAddress(s.Address)
	}
	return s, err
}

// Save atomically replaces settings after validation.
func Save(path string, s Settings) (err error) {
	s.Address, err = NormalizeAddress(s.Address)
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".config-*.tmp")
	if err != nil {
		return err
	}
	name := f.Name()
	defer func() {
		if e := os.Remove(name); e != nil && !errors.Is(e, os.ErrNotExist) {
			err = errors.Join(err, e)
		}
	}()
	_, writeErr := f.Write(append(b, '\n'))
	syncErr := f.Sync()
	closeErr := f.Close()
	if err = errors.Join(writeErr, syncErr, closeErr); err != nil {
		return err
	}
	return os.Rename(name, path)
}

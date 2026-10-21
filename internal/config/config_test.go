package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.json")
	s, err := Load(path)
	if err != nil || s.Address != "" {
		t.Fatalf("missing: %+v %v", s, err)
	}
	if err = Save(path, Settings{Address: "de:32:62:6d:30:20"}); err != nil {
		t.Fatal(err)
	}
	s, err = Load(path)
	if err != nil || s.Address != "DE:32:62:6D:30:20" {
		t.Fatalf("%+v %v", s, err)
	}
	if err = Save(path, Settings{Address: "AA:BB:CC:DD:EE:FF"}); err != nil {
		t.Fatal(err)
	}
	if err = Save(path, Settings{Address: "invalid"}); err == nil {
		t.Fatal("invalid saved")
	}
	s, err = Load(path)
	if err != nil || s.Address != "AA:BB:CC:DD:EE:FF" {
		t.Fatalf("previous config lost: %+v %v", s, err)
	}
	if err = os.WriteFile(path, []byte("{broken"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = Load(path); err == nil {
		t.Fatal("corrupt config accepted")
	}
}
func TestAddressValidation(t *testing.T) {
	for _, s := range []string{"", "00:11:22:33:44:55:66:77", "bad"} {
		if _, err := NormalizeAddress(s); err == nil {
			t.Fatalf("accepted %q", s)
		}
	}
}

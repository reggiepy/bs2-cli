package ble

import "testing"

func TestLockExclusionAndRelease(t *testing.T) {
	release, err := Lock("00:00:00:00:00:01")
	if err != nil {
		t.Fatal(err)
	}
	release2, err := Lock("00:00:00:00:00:01")
	if err == nil {
		if e := release2(); e != nil {
			t.Fatal(e)
		}
		t.Fatal("duplicate device lock acquired")
	}
	if err = release(); err != nil {
		t.Fatal(err)
	}
	release, err = Lock("00:00:00:00:00:01")
	if err != nil {
		t.Fatal(err)
	}
	if err = release(); err != nil {
		t.Fatal(err)
	}
}

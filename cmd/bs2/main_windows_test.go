package main

import (
	"testing"
)

func TestParseValidation(t *testing.T) {
	for _, tt := range []struct {
		name string
		args []string
		ok   bool
	}{
		{"devices", []string{"devices"}, true},
		{"status short deadline", []string{"status", "--address", "AA:BB:CC:DD:EE:FF", "--timeout", "2s"}, true},
		{"missing address", []string{"status"}, false},
		{"invalid rpm", []string{"set", "--address", "AA:BB:CC:DD:EE:FF", "--rpm", "4000"}, false},
		{"scan deadline", []string{"scan", "--duration", "10s", "--timeout", "5s"}, false},
		{"hold deadline", []string{"set", "--address", "AA:BB:CC:DD:EE:FF", "--rpm", "2800", "--hold", "30s"}, false},
		{"invalid preset", []string{"preset", "--address", "AA:BB:CC:DD:EE:FF", "--number", "5"}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parse(tt.args)
			if (err == nil) != tt.ok {
				t.Fatalf("error %v", err)
			}
		})
	}
}

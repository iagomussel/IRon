package router

import (
	"testing"
)

func TestRouter_Route(t *testing.T) {
	r := New()

	tests := []struct {
		input       string
		wantIntent  string
		wantMatched bool
	}{
		{"/help", "help", true},
		{"ping", "ping", true},
		{"nota: comprar leite", "notes.append", true},
		{"Note: meeting", "notes.append", true},
		{"random text", "", false},
	}

	for _, tt := range tests {
		packet, matched := r.Route(tt.input)
		if matched != tt.wantMatched {
			t.Errorf("Route(%q) matched = %v, want %v", tt.input, matched, tt.wantMatched)
			continue
		}
		if matched && packet.Intent != tt.wantIntent {
			t.Errorf("Route(%q) intent = %v, want %v", tt.input, packet.Intent, tt.wantIntent)
		}
	}
}

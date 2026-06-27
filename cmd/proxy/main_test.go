package main

import (
	"testing"
)

func TestHumanBytes(t *testing.T) {
	tests := []struct {
		n    int64
		want string
	}{
		{0, "0 B"},
		{500, "500 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{104857600, "100.0 MB"},
		{1073741824, "1.0 GB"},
	}
	for _, tt := range tests {
		got := humanBytes(tt.n)
		if got != tt.want {
			t.Errorf("humanBytes(%d): got %q, want %q", tt.n, got, tt.want)
		}
	}
}

func TestAcceptContains(t *testing.T) {
	tests := []struct {
		accept string
		mime   string
		want   bool
	}{
		{"text/html,application/json", "text/html", true},
		{"application/json", "text/html", false},
		{"", "text/html", false},
		{"text/html", "text/html", true},
		{"TEXT/HTML", "text/html", false},
	}
	for _, tt := range tests {
		got := acceptContains(tt.accept, tt.mime)
		if got != tt.want {
			t.Errorf("acceptContains(%q, %q): got %v, want %v", tt.accept, tt.mime, got, tt.want)
		}
	}
}

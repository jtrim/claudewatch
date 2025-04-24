package main

import (
	"testing"
)

func TestIsEmacsTemp(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     bool
	}{
		{"Emacs auto-save file", "#tempfile#", true},
		{"Emacs backup file", "tempfile~", true},
		{"Emacs lock file", ".#tempfile", true},
		{"Regular file", "normalfile.txt", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isEmacsTemp(tt.filename); got != tt.want {
				t.Errorf("isEmacsTemp(%q) = %v, want %v", tt.filename, got, tt.want)
			}
		})
	}
}

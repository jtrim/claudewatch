package main

import (
	"testing"
)

func TestIsHiddenOrSpecialFile(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
		want     bool
	}{
		{"Hidden file", ".hidden", true},
		{"Hidden file in path", "/path/to/.hidden", true},
		{"Normal file", "normal.txt", false},
		{"Normal file with path", "/path/to/normal.txt", false},
		{"Emacs auto-save file", "#tempfile#", true},
		{"Emacs backup file", "file.txt~", true},
		{"Emacs lock file", ".#tempfile", true},
		{"Emacs auto-save file with path", "/path/to/#tempfile#", true},
		{"Path containing dot but not hidden", "/path/with./file.txt", false},
		{"Parent directory reference", "..", true},
		{"Current directory reference", ".", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsHiddenOrSpecialFile(tt.filePath); got != tt.want {
				t.Errorf("IsHiddenOrSpecialFile(%q) = %v, want %v", tt.filePath, got, tt.want)
			}
		})
	}
}

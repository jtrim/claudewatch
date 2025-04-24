package main

import (
	"testing"
)

// Test if content has active AI markers
func TestHasActiveAIMarkers(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    bool
	}{
		// Base cases
		{
			name:    "No AI markers",
			content: "This is a regular file\nwith no AI markers",
			want:    false,
		},

		// Standard marker "ai!" tests ai:ignore
		{
			name:    "Single active AI marker ai! comment-style 1", // ai:ignore
			content: "This is a file\n// with an active ai!",       // ai:ignore
			want:    true,
		},
		{
			name:    "Single active AI marker ai! comment-style 2", // ai:ignore
			content: "This is a file\n# with an active ai!",        // ai:ignore
			want:    true,
		},
		{
			name:    "Single active AI marker ai! comment-style 3", // ai:ignore
			content: "This is a file\n/* with an active ai! */",    // ai:ignore
			want:    true,
		},
		{
			name:    "Single active AI marker ai! comment-style 4", // ai:ignore
			content: "This is a file\n* with an active ai!",        // ai:ignore
			want:    true,
		},

		// Alternative marker "!ai" tests ai:ignore
		{
			name:    "Single active AI marker !ai comment-style 1", // ai:ignore
			content: "This is a file\n// with an active !ai",       // ai:ignore
			want:    true,
		},
		{
			name:    "Single active AI marker !ai comment-style 2", // ai:ignore
			content: "This is a file\n# with an active !ai",        // ai:ignore
			want:    true,
		},
		{
			name:    "Single active AI marker !ai comment-style 3", // ai:ignore
			content: "This is a file\n/* with an active !ai */",    // ai:ignore
			want:    true,
		},
		{
			name:    "Single active AI marker !ai comment-style 4", // ai:ignore
			content: "This is a file\n* with an active !ai",        // ai:ignore
			want:    true,
		},

		// Alternative marker "ai?" tests ai:ignore
		{
			name:    "Single active AI marker ai? comment-style 1", // ai:ignore
			content: "This is a file\n// with an active ai?",       // ai:ignore
			want:    true,
		},
		{
			name:    "Single active AI marker ai? comment-style 2", // ai:ignore
			content: "This is a file\n# with an active ai?",        // ai:ignore
			want:    true,
		},
		{
			name:    "Single active AI marker ai? comment-style 3", // ai:ignore
			content: "This is a file\n/* with an active ai? */",    // ai:ignore
			want:    true,
		},
		{
			name:    "Single active AI marker ai? comment-style 4", // ai:ignore
			content: "This is a file\n* with an active ai?",        // ai:ignore
			want:    true,
		},

		// Case insensitivity tests
		{
			name:    "Case insensitive AI!",  // ai:ignore
			content: "// with an active AI!", // ai:ignore
			want:    true,
		},
		{
			name:    "Case insensitive !AI",  // ai:ignore
			content: "// with an active !AI", // ai:ignore
			want:    true,
		},
		{
			name:    "Case insensitive AI?",  // ai:ignore
			content: "// with an active AI?", // ai:ignore
			want:    true,
		},
		{
			name:    "Case insensitive aI:iGnOrE",                  // ai:ignore
			content: "// aI:iGnOrE\n// this marker is ignored ai!", // ai:ignore
			want:    false,
		},

		// Ignore directive tests
		{
			name:    "Ignored AI marker with ai:ignore directly before",
			content: "This is a file\n// ai:ignore\n// with an ignored ai!", // ai:ignore
			want:    false,
		},
		{
			name:    "Ignore applies to alternative marker !ai", // ai:ignore
			content: "// ai:ignore\n// with an ignored !ai",     // ai:ignore
			want:    false,
		},
		{
			name:    "Ignore applies to alternative marker ai?", // ai:ignore
			content: "// ai:ignore\n// with an ignored ai?",     // ai:ignore
			want:    false,
		},
		{
			name:    "Mixed active and ignored markers",
			content: "// ai:ignore\n// this marker is ignored ai!\n// but this one is active ai!", // ai:ignore
			want:    true,
		},
		{
			name:    "ai:ignore applies only to next marker",
			content: "// ai:ignore\n// this marker is ignored ai!\n// some other line\n// this one is active ai!", // ai:ignore
			want:    true,
		},
		{
			name:    "All markers ignored",
			content: "// ai:ignore\n// this marker is ignored ai!\n// ai:ignore\n// this one is also ignored ai!", // ai:ignore
			want:    false,
		},
		{
			name:    "ai:ignore with line in between doesn't apply",
			content: "// ai:ignore\n// some other line\n// this marker is active ai!", // ai:ignore
			want:    true,
		},
		{
			name:    "Multiple ai:ignore lines",
			content: "// ai:ignore\n// ai:ignore\n// this is still only ignored once ai!", // ai:ignore
			want:    false,
		},
		{
			name:    "Different comment styles",
			content: "/* ai:ignore */\n# this marker is ignored ai!", // ai:ignore
			want:    false,
		},
		{
			name:    "ai:ignore with whitespace",
			content: "//    ai:ignore    \n//   this is ignored ai!   ", // ai:ignore
			want:    false,
		},

		// Same line ignore tests
		{
			name:    "ai:ignore and ai! on same line comment-style 1", // ai:ignore
			content: "// ai:ignore ai!",                               // ai:ignore
			want:    false,
		},
		{
			name:    "ai:ignore and !ai on same line comment-style 1", // ai:ignore
			content: "// ai:ignore !ai",                               // ai:ignore
			want:    false,
		},
		{
			name:    "ai:ignore and ai? on same line comment-style 1", // ai:ignore
			content: "// ai:ignore ai?",                               // ai:ignore
			want:    false,
		},
		{
			name:    "ai! and ai:ignore on same line (reversed order)", // ai:ignore
			content: "// ai! ai:ignore",                                // ai:ignore
			want:    false,
		},
		{
			name:    "ai:ignore and ai! on same line comment-style 2", // ai:ignore
			content: "# ai:ignore ai!",                                // ai:ignore
			want:    false,
		},
		{
			name:    "ai:ignore and ai! on same line comment-style 3", // ai:ignore
			content: "/* ai:ignore ai! */",                            // ai:ignore
			want:    false,
		},
		{
			name:    "ai:ignore and ai! on same line comment-style 4", // ai:ignore
			content: "* ai:ignore ai!",                                // ai:ignore
			want:    false,
		},
		{
			name:    "ai:ignore and ai! with text between",    // ai:ignore
			content: "// ai:ignore here's an instruction ai!", // ai:ignore
			want:    false,
		},
		{
			name:    "Mixed ignored inline and active markers",     // ai:ignore
			content: "// ai:ignore ai!\n// this one is active ai!", // ai:ignore
			want:    true,
		},
		{
			name:    "Only ignored inline markers",
			content: "// ai:ignore ai!\n// also ai:ignore ai!", // ai:ignore
			want:    false,
		},

		// Mixed marker types
		{
			name:    "Mixed marker types all active",
			content: "// ai!\n// !ai\n// ai?", // ai:ignore
			want:    true,
		},
		{
			name:    "Mixed marker types with some ignored",
			content: "// ai:ignore\n// ai!\n// !ai\n// ai?", // ai:ignore
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasActiveAIMarkers(tt.content); got != tt.want {
				t.Errorf("hasActiveAIMarkers() = %v, want %v for content:\n%s", got, tt.want, tt.content)
			}
		})
	}
}

// Test finding all active AI markers in content
func TestFindActiveAIMarkers(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    int   // Number of markers expected
		lines   []int // Expected line numbers
	}{
		{
			name:    "No AI markers",
			content: "This is a regular file\nwith no AI markers",
			want:    0,
			lines:   []int{},
		},
		{
			name:    "Single active AI marker",
			content: "This is a file\n// with an active ai!", // ai:ignore
			want:    1,
			lines:   []int{2},
		},
		{
			name:    "Multiple active AI markers",
			content: "// This file has ai!\nseveral markers\n// on different !ai\nlines\n// like ai?", // ai:ignore
			want:    3,
			lines:   []int{1, 3, 5},
		},
		{
			name:    "Ignored and active markers",
			content: "// ai:ignore\n// this marker is ignored ai!\n// but this one is active ai!",
			want:    1,
			lines:   []int{3},
		},
		{
			name:    "All markers ignored",
			content: "// ai:ignore\n// this marker is ignored ai!\n// ai:ignore\n// this one is also ignored ai!",
			want:    0,
			lines:   []int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			markers := findActiveAIMarkers(tt.content)

			// Check count
			if got := len(markers); got != tt.want {
				t.Errorf("findActiveAIMarkers() returned %v markers, want %v for content:\n%s", got, tt.want, tt.content)
			}

			// Check line numbers if we have markers
			if len(markers) > 0 {
				for i, marker := range markers {
					if i >= len(tt.lines) {
						t.Errorf("findActiveAIMarkers() returned more markers than expected")
						break
					}
					if marker.LineNumber != tt.lines[i] {
						t.Errorf("findActiveAIMarkers() marker %d has line number %d, want %d", i, marker.LineNumber, tt.lines[i])
					}
				}
			}
		})
	}
}

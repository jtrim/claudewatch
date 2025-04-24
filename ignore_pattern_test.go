package main

import (
	"regexp"
	"testing"
)

func TestCompileIgnorePattern(t *testing.T) {
	tests := []struct {
		name        string
		pattern     string
		wantErr     bool
		testStrings []struct {
			str  string
			want bool
		}
	}{
		{
			name:    "Valid pattern - ignore .js files",
			pattern: `\.js$`,
			wantErr: false,
			testStrings: []struct {
				str  string
				want bool
			}{
				{"file.js", true},
				{"path/to/file.js", true},
				{"file.jsx", false},
				{"file.ts", false},
				{"javascript.txt", false},
			},
		},
		{
			name:    "Valid pattern - ignore node_modules directory",
			pattern: `node_modules`,
			wantErr: false,
			testStrings: []struct {
				str  string
				want bool
			}{
				{"node_modules/package.json", true},
				{"/root/node_modules/file.js", true},
				{"/path/to/node_modules", true},
				{"mynode_modules", true}, // This would match too as it contains "node_modules"
				{"modules", false},
				{"node", false},
			},
		},
		{
			name:    "Valid pattern - exact node_modules directory",
			pattern: `(^|/)node_modules(/|$)`,
			wantErr: false,
			testStrings: []struct {
				str  string
				want bool
			}{
				{"node_modules/package.json", true},
				{"/root/node_modules/file.js", true},
				{"/path/to/node_modules", true},
				{"mynode_modules", false}, // This would not match now
				{"modules", false},
				{"node", false},
			},
		},
		{
			name:    "Invalid pattern - unclosed parenthesis",
			pattern: `(\w+`,
			wantErr: true,
			testStrings: []struct {
				str  string
				want bool
			}{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CompileIgnorePattern(tt.pattern)
			if (err != nil) != tt.wantErr {
				t.Errorf("CompileIgnorePattern() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// If we expected an error, no need to test pattern matching
			if tt.wantErr {
				return
			}

			// Test pattern matching against test strings
			for _, ts := range tt.testStrings {
				match := got.MatchString(ts.str)
				if match != ts.want {
					t.Errorf("Pattern %q on string %q = %v, want %v", tt.pattern, ts.str, match, ts.want)
				}
			}
		})
	}
}

func TestShouldIgnoreFile(t *testing.T) {
	// Compile some test patterns
	jsPattern, _ := regexp.Compile(`\.js$`)
	nodeModulesPattern, _ := regexp.Compile(`(^|/)node_modules(/|$)`)

	tests := []struct {
		name          string
		filePath      string
		ignorePattern *regexp.Regexp
		want          bool
	}{
		{"Nil pattern", "file.js", nil, false},
		{"JS file with JS pattern", "file.js", jsPattern, true},
		{"Non-JS file with JS pattern", "file.ts", jsPattern, false},
		{"Long path JS file with JS pattern", "/path/to/file.js", jsPattern, true},
		{"node_modules file with node_modules pattern", "node_modules/file.js", nodeModulesPattern, true},
		{"Path with node_modules with node_modules pattern", "/root/node_modules/file.js", nodeModulesPattern, true},
		{"Non-node_modules path with node_modules pattern", "/root/src/file.js", nodeModulesPattern, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ShouldIgnoreFile(tt.filePath, tt.ignorePattern); got != tt.want {
				t.Errorf("ShouldIgnoreFile(%q, %v) = %v, want %v", tt.filePath, tt.ignorePattern, got, tt.want)
			}
		})
	}
}

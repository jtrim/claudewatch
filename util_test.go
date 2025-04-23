package main

import (
	"os"
	"path/filepath"
	"regexp"
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

// foo
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsHiddenOrSpecialFile(tt.filePath); got != tt.want {
				t.Errorf("IsHiddenOrSpecialFile(%q) = %v, want %v", tt.filePath, got, tt.want)
			}
		})
	}
}

func TestLoadIgnorePatterns(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "claudewatch-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a .claudewatchignore file
	ignoreContent := `# This is a comment
\.js$
node_modules/
test_.*\.go

# Empty lines should be ignored
`
	ignoreFilePath := filepath.Join(tempDir, ".claudewatchignore")
	err = os.WriteFile(ignoreFilePath, []byte(ignoreContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write ignore file: %v", err)
	}

	// Load the patterns
	patterns, err := LoadIgnorePatterns(tempDir)
	if err != nil {
		t.Fatalf("LoadIgnorePatterns failed: %v", err)
	}

	// Check the number of patterns
	expectedPatternCount := 3 // 3 non-comment non-empty lines
	if len(patterns) != expectedPatternCount {
		t.Errorf("Expected %d patterns, got %d", expectedPatternCount, len(patterns))
	}

	// Test matching patterns
	testCases := []struct {
		path          string
		shouldIgnore  bool
		patternReason string
	}{
		{"/path/to/file.js", true, "js extension pattern"},
		{"/path/to/file.go", false, "no match"},
		{"/path/to/node_modules/file.txt", true, "node_modules pattern"},
		{"/path/to/test_main.go", true, "test pattern"},
		{"/path/to/main_test.go", false, "not matching test pattern"},
	}

	for _, tc := range testCases {
		result := patterns.MatchesAnyPattern(tc.path)
		if result != tc.shouldIgnore {
			t.Errorf("Path %s: expected ignore=%v, got %v (reason: %s)",
				tc.path, tc.shouldIgnore, result, tc.patternReason)
		}
	}
}

func TestIgnorePatternsMatchesAnyPattern(t *testing.T) {
	// Create test patterns
	patterns := IgnorePatterns{
		regexp.MustCompile(`\.js$`),
		regexp.MustCompile(`node_modules/`),
		regexp.MustCompile(`test_.*\.go`),
	}

	// Empty patterns
	emptyPatterns := IgnorePatterns{}

	tests := []struct {
		name     string
		patterns IgnorePatterns
		filePath string
		want     bool
	}{
		{"JS file with JS pattern", patterns, "/path/to/file.js", true},
		{"Non-JS file with JS pattern", patterns, "/path/to/file.ts", false},
		{"node_modules file", patterns, "/path/to/node_modules/file.txt", true},
		{"Go test file", patterns, "/path/to/test_main.go", true},
		{"Regular Go file", patterns, "/path/to/main.go", false},
		{"Go test file with different naming", patterns, "/path/to/main_test.go", false},
		{"Empty patterns", emptyPatterns, "/path/to/file.js", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.patterns.MatchesAnyPattern(tt.filePath); got != tt.want {
				t.Errorf("IgnorePatterns.MatchesAnyPattern(%q) = %v, want %v", tt.filePath, got, tt.want)
			}
		})
	}
}

func TestShouldIgnoreFileWithConfig(t *testing.T) {
	// Create a Config with ignore pattern and patterns
	config := &Config{
		IgnorePattern: regexp.MustCompile(`\.ignore$`),
		IgnorePatterns: IgnorePatterns{
			regexp.MustCompile(`\.js$`),
			regexp.MustCompile(`temp/`),
		},
	}

	configOnlyPattern := &Config{
		IgnorePattern:  regexp.MustCompile(`\.ignore$`),
		IgnorePatterns: nil,
	}

	configOnlyPatterns := &Config{
		IgnorePattern: nil,
		IgnorePatterns: IgnorePatterns{
			regexp.MustCompile(`\.js$`),
			regexp.MustCompile(`temp/`),
		},
	}

	configEmpty := &Config{
		IgnorePattern:  nil,
		IgnorePatterns: nil,
	}

	tests := []struct {
		name           string
		config         *Config
		filePath       string
		shouldIgnore   bool
		expectedReason string
	}{
		// Tests with both pattern and patterns
		{"Ignore by IgnorePattern", config, "/path/to/file.ignore", true, "ignore pattern (--ignore)"},
		{"Ignore by IgnorePatterns (.js)", config, "/path/to/file.js", true, ".claudewatchignore pattern"},
		{"Ignore by IgnorePatterns (temp/)", config, "/path/to/temp/file.txt", true, ".claudewatchignore pattern"},
		{"No match in any pattern", config, "/path/to/regular.txt", false, ""},

		// Tests with only IgnorePattern
		{"Only IgnorePattern - match", configOnlyPattern, "/path/to/file.ignore", true, "ignore pattern (--ignore)"},
		{"Only IgnorePattern - no match", configOnlyPattern, "/path/to/file.js", false, ""},

		// Tests with only IgnorePatterns
		{"Only IgnorePatterns - match .js", configOnlyPatterns, "/path/to/file.js", true, ".claudewatchignore pattern"},
		{"Only IgnorePatterns - match temp/", configOnlyPatterns, "/path/to/temp/file.txt", true, ".claudewatchignore pattern"},
		{"Only IgnorePatterns - no match", configOnlyPatterns, "/path/to/regular.txt", false, ""},

		// Tests with empty config
		{"Empty config", configEmpty, "/path/to/file.js", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ignore, reason := ShouldIgnoreFileWithConfig(tt.filePath, tt.config)
			if ignore != tt.shouldIgnore {
				t.Errorf("ShouldIgnoreFileWithConfig() ignore = %v, want %v", ignore, tt.shouldIgnore)
			}
			if tt.shouldIgnore && reason != tt.expectedReason {
				t.Errorf("ShouldIgnoreFileWithConfig() reason = %v, want %v", reason, tt.expectedReason)
			}
		})
	}
}

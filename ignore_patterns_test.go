package main

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

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

func TestShouldIgnorePathWithConfig(t *testing.T) {
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
			ignore, reason := ShouldIgnorePathWithConfig(tt.filePath, tt.config)
			if ignore != tt.shouldIgnore {
				t.Errorf("ShouldIgnorePathWithConfig() ignore = %v, want %v", ignore, tt.shouldIgnore)
			}
			if tt.shouldIgnore && reason != tt.expectedReason {
				t.Errorf("ShouldIgnorePathWithConfig() reason = %v, want %v", reason, tt.expectedReason)
			}
		})
	}
}

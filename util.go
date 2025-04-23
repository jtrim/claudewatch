package main

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// isEmacsTemp checks if a filename is an Emacs temporary file
func isEmacsTemp(filename string) bool {
	// Emacs auto-save files: #filename#
	if strings.HasPrefix(filename, "#") && strings.HasSuffix(filename, "#") {
		return true
	}

	// Emacs backup files: filename~
	if strings.HasSuffix(filename, "~") {
		return true
	}

	// Emacs lock files: .#filename
	if strings.HasPrefix(filename, ".#") {
		return true
	}

	return false
}

// supportedAIMarkers contains all the supported AI markers
var supportedAIMarkers = []string{"ai!", "!ai", "ai?"}

// Create common regex patterns once for performance
var (
	markerPattern = buildMarkerPattern()
	ignoreRegex   = regexp.MustCompile(`(?i)ai:ignore`)
	commentStart  = regexp.MustCompile(`(?:\s*\/\/|\s*#|\s*\/\*|\s*\*)`)
)

// buildMarkerPattern builds a regex pattern that matches any of the supported markers
func buildMarkerPattern() *regexp.Regexp {
	// Escape special characters in markers
	escapedMarkers := make([]string, len(supportedAIMarkers))
	for i, marker := range supportedAIMarkers {
		escapedMarkers[i] = regexp.QuoteMeta(marker)
	}

	// Create a pattern that matches any of the markers in case-insensitive mode
	pattern := `(?i)(?:` + strings.Join(escapedMarkers, "|") + `)`
	return regexp.MustCompile(pattern)
}

// hasAIMarker checks if a line contains any AI marker
func hasAIMarker(line string) bool {
	return markerPattern.MatchString(line)
}

// hasIgnoreDirective checks if a line contains the ignore directive
func hasIgnoreDirective(line string) bool {
	return ignoreRegex.MatchString(line)
}

// isComment checks if a line starts with a comment marker
func isComment(line string) bool {
	return commentStart.MatchString(line)
}

// hasBothMarkerAndIgnore checks if a line contains both a marker and ignore directive
func hasBothMarkerAndIgnore(line string) bool {
	return isComment(line) && hasIgnoreDirective(line) && hasAIMarker(line)
}

// AIMarkerLocation represents a line with an AI marker
type AIMarkerLocation struct {
	LineNumber int
	LineText   string
}

// findActiveAIMarkers checks if the content has any non-ignored AI markers
// and returns their locations (line numbers and text)
func findActiveAIMarkers(content string) []AIMarkerLocation {
	lines := strings.Split(content, "\n")
	var markers []AIMarkerLocation

	ignoreNextAI := false

	for i, line := range lines {
		lineNumber := i + 1 // Line numbers start from 1

		if hasBothMarkerAndIgnore(line) {
			continue
		}

		if isComment(line) && hasIgnoreDirective(line) && !hasAIMarker(line) {
			ignoreNextAI = true
			continue
		}

		// Check if this line contains an AI marker
		if isComment(line) && hasAIMarker(line) {
			if ignoreNextAI {
				// This AI marker is ignored
				ignoreNextAI = false // Reset for the next marker
			} else {
				// Found an active AI marker
				markers = append(markers, AIMarkerLocation{
					LineNumber: lineNumber,
					LineText:   line,
				})
			}
		} else {
			// If we see any non-AI line after an ai:ignore line, the ignore is no longer active
			// This ensures that ai:ignore only applies to the very next line with an AI marker
			ignoreNextAI = false
		}
	}

	return markers
}

// hasActiveAIMarkers checks if the content has any non-ignored AI markers
func hasActiveAIMarkers(content string) bool {
	markers := findActiveAIMarkers(content)
	return len(markers) > 0
}

// CompileIgnorePattern creates a regular expression from a pattern string
// It returns the compiled pattern and any error encountered
func CompileIgnorePattern(pattern string) (*regexp.Regexp, error) {
	return regexp.Compile(pattern)
}

// ShouldIgnoreFile checks if a file should be ignored based on the ignore pattern
// Returns true if the file should be ignored
func ShouldIgnoreFile(filePath string, ignorePattern *regexp.Regexp) bool {
	// If no ignore pattern is set, don't ignore any files
	if ignorePattern == nil {
		return false
	}

	// Check if the file path matches the ignore pattern
	return ignorePattern.MatchString(filePath)
}

// IsHiddenOrSpecialFile checks if a file is a hidden file, a special file, or an Emacs temp file
func IsHiddenOrSpecialFile(filePath string) bool {
	// Get the base filename
	baseName := filepath.Base(filePath)

	// Check if it's a hidden file (starts with a dot)
	if strings.HasPrefix(baseName, ".") {
		return true
	}

	// Check if it's an Emacs temporary file
	if isEmacsTemp(baseName) {
		return true
	}

	return false
}

// IgnorePatterns contains compiled regular expressions from .claudewatchignore
type IgnorePatterns []*regexp.Regexp

// LoadIgnorePatterns loads ignore patterns from .claudewatchignore file
func LoadIgnorePatterns(rootDir string) (IgnorePatterns, error) {
	ignoreFilePath := filepath.Join(rootDir, ".claudewatchignore")

	// Check if the ignore file exists
	_, err := os.Stat(ignoreFilePath)
	if os.IsNotExist(err) {
		// No ignore file, return empty patterns
		return nil, nil
	} else if err != nil {
		// Error accessing the file
		return nil, err
	}

	// Open and read the ignore file
	file, err := os.Open(ignoreFilePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var patterns IgnorePatterns
	scanner := bufio.NewScanner(file)

	// Read line by line
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Compile the regular expression
		pattern, err := regexp.Compile(line)
		if err != nil {
			// Continue with other patterns if one fails
			continue
		}

		patterns = append(patterns, pattern)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return patterns, nil
}

// MatchesAnyPattern checks if a file path matches any of the ignore patterns
func (p IgnorePatterns) MatchesAnyPattern(filePath string) bool {
	if len(p) == 0 {
		return false
	}

	for _, pattern := range p {
		if pattern.MatchString(filePath) {
			return true
		}
	}

	return false
}

// ShouldIgnoreFileWithConfig checks if a file should be ignored based on both ignore pattern and ignore patterns
func ShouldIgnoreFileWithConfig(filePath string, config *Config) (bool, string) {
	// Check the single ignore pattern first
	if config.IgnorePattern != nil && config.IgnorePattern.MatchString(filePath) {
		return true, "ignore pattern (--ignore)"
	}

	// Then check patterns from .claudewatchignore
	if config.IgnorePatterns != nil && config.IgnorePatterns.MatchesAnyPattern(filePath) {
		return true, ".claudewatchignore pattern"
	}

	return false, ""
}

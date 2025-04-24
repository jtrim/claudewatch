package main

import (
	"testing"
)

func TestRemoveAIMarkersFromContent(t *testing.T) {
	// Sample content with AI markers
	content := `package main

// This is a normal comment
func foo() {
    // This should be refactored !ai
    doSomething()

    // ai! This needs better error handling
    handleErrors()

    // This should be optimized for performance AI!
    computeData()
}
`

	// Create markers at the lines with AI markers
	markers := []AIMarkerLocation{
		{LineNumber: 5, LineText: "    // This should be refactored !ai"},
		{LineNumber: 8, LineText: "    // ai! This needs better error handling"},
		{LineNumber: 11, LineText: "    // This should be optimized for performance AI!"},
	}

	// Expected content after removing markers
	expectedContent := `package main

// This is a normal comment
func foo() {
    // This should be refactored
    doSomething()

    //  This needs better error handling
    handleErrors()

    // This should be optimized for performance
    computeData()
}
`

	// Expected markers after removal
	expectedMarkers := []AIMarkerLocation{
		{LineNumber: 5, LineText: "    // This should be refactored "},
		{LineNumber: 8, LineText: "    //  This needs better error handling"},
		{LineNumber: 11, LineText: "    // This should be optimized for performance "},
	}

	// Call the function
	updatedContent, updatedMarkers, err := removeAIMarkersFromContent(content, markers)

	// Check for errors
	if err != nil {
		t.Errorf("removeAIMarkersFromContent returned error: %v", err)
	}

	// Check if content was correctly updated
	if updatedContent != expectedContent {
		t.Errorf("removeAIMarkersFromContent content update failed.\nGot:\n%s\nExpected:\n%s",
			updatedContent, expectedContent)
	}

	// Check if markers were correctly updated
	for i, marker := range updatedMarkers {
		if marker.LineNumber != expectedMarkers[i].LineNumber || marker.LineText != expectedMarkers[i].LineText {
			t.Errorf("Marker %d mismatch.\nGot: %+v\nExpected: %+v", i, marker, expectedMarkers[i])
		}
	}
}

func TestRemoveAIMarkersFromContentWithInvalidLineNumber(t *testing.T) {
	content := "line1\nline2\nline3"

	// Create a marker with an invalid line number
	markers := []AIMarkerLocation{
		{LineNumber: 5, LineText: "This is beyond the content bounds"},
	}

	// Call the function
	_, _, err := removeAIMarkersFromContent(content, markers)

	// We expect an error due to invalid line number
	if err == nil {
		t.Error("removeAIMarkersFromContent did not return error for invalid line number")
	}
}

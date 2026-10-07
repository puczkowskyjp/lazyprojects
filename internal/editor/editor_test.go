package editor

import (
	"testing"
)

func TestDetectAvailable(t *testing.T) {
	editors := DetectAvailable()
	// On this system, code or notepad should at least be present
	t.Logf("Detected editors: %v", editors)
}

func TestOpenEmptyEditor(t *testing.T) {
	err := Open("", ".")
	if err == nil {
		t.Errorf("expected error when opening with empty editor")
	}
}

func TestOpenInvalidEditor(t *testing.T) {
	err := Open("non_existent_editor_binary_xyz123", ".")
	if err == nil {
		t.Errorf("expected error when opening with non-existent editor")
	}
}

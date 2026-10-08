package editor

import (
	"reflect"
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

func TestNewWezTermCommand(t *testing.T) {
	t.Setenv("WEZTERM_PANE", "")

	tests := []struct {
		name   string
		target string
		args   []string
		want   []string
	}{
		{
			name:   "tab opens nvim in project directory",
			target: "tab",
			want:   []string{"wezterm", "start", "--new-tab", "--cwd", "/projects/demo", "--", "nvim", "."},
		},
		{
			name:   "window preserves editor arguments",
			target: "window",
			args:   []string{"-c", "Telescope"},
			want:   []string{"wezterm", "start", "--cwd", "/projects/demo", "--", "nvim", "-c", "Telescope"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newWezTermCommand("/projects/demo", "nvim", tt.args, tt.target)
			if !reflect.DeepEqual(cmd.Args, tt.want) {
				t.Errorf("command args = %q, want %q", cmd.Args, tt.want)
			}
		})
	}
}

func TestNewWezTermCommandUsesCLIWithinWezTerm(t *testing.T) {
	t.Setenv("WEZTERM_PANE", "42")

	cmd := newWezTermCommand("/projects/demo", "nvim", nil, "tab")
	want := []string{"wezterm", "cli", "spawn", "--cwd", "/projects/demo", "--", "nvim", "."}
	if !reflect.DeepEqual(cmd.Args, want) {
		t.Errorf("command args = %q, want %q", cmd.Args, want)
	}
}

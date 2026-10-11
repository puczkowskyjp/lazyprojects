package editor

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDetectAvailable(t *testing.T) {
	editors := DetectAvailable()
	// On this system, code or notepad should at least be present
	t.Logf("Detected editors: %v", editors)
}

func TestDetectAvailableUsesConfiguredEditors(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "vs.exe"), nil, 0755); err != nil {
		t.Fatalf("creating Visual Studio executable: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "nvim"), nil, 0755); err != nil {
		t.Fatalf("creating NeoVim executable: %v", err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("EDITOR", "nvim")

	got := DetectAvailable("vs.exe")
	want := []string{"vs.exe"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("DetectAvailable() = %v, want %v", got, want)
	}
}

func TestVisualStudioIsNotTerminalEditor(t *testing.T) {
	if IsTerminalEditor("vs.exe") {
		t.Fatal("Visual Studio should be launched as a GUI editor")
	}
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

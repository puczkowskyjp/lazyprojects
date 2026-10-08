package editor

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// KnownEditors lists common editors to probe for availability.
var KnownEditors = []string{
	"code",
	"nvim",
	"vim",
	"goland",
	"idea",
	"subl",
	"notepad",
}

// TerminalOptions configures how terminal-based editors are launched.
type TerminalOptions struct {
	App    string // "wt" (Windows Terminal), "wezterm", "cmd"
	Target string // "tab" or "window"
}

// DetectAvailable finds which known editors are installed and reachable in PATH.
func DetectAvailable() []string {
	available := make([]string, 0)
	seen := make(map[string]bool)

	if envEditor := os.Getenv("EDITOR"); envEditor != "" {
		available = append(available, envEditor)
		seen[strings.ToLower(envEditor)] = true
	}

	for _, ed := range KnownEditors {
		if seen[strings.ToLower(ed)] {
			continue
		}
		if _, err := exec.LookPath(ed); err == nil {
			available = append(available, ed)
			seen[strings.ToLower(ed)] = true
		}
	}

	return available
}

// IsTerminalEditor checks if the command is a known terminal-based editor.
func IsTerminalEditor(editorCmd string) bool {
	parts := strings.Fields(editorCmd)
	if len(parts) == 0 {
		return false
	}
	base := strings.ToLower(filepath.Base(parts[0]))
	base = strings.TrimSuffix(base, ".exe")
	switch base {
	case "nvim", "vim", "vi", "nano", "hx", "helix":
		return true
	default:
		return false
	}
}

// Open launches the editor with default terminal options.
func Open(editorCmd, projectPath string) error {
	return OpenWithOptions(editorCmd, projectPath, TerminalOptions{
		App:    "wt",
		Target: "tab",
	})
}

// OpenWithOptions launches the editor with specific terminal options.
func OpenWithOptions(editorCmd, projectPath string, termOpts TerminalOptions) error {
	if strings.TrimSpace(editorCmd) == "" {
		return errors.New("no editor command specified")
	}

	parts := strings.Fields(editorCmd)
	cmdName := parts[0]
	args := parts[1:]

	if _, err := exec.LookPath(cmdName); err != nil {
		return fmt.Errorf("editor %q not found in PATH: %w", cmdName, err)
	}

	var cmd *exec.Cmd

	if IsTerminalEditor(cmdName) && strings.EqualFold(termOpts.App, "wezterm") {
		cmd = newWezTermCommand(projectPath, cmdName, args, termOpts.Target)
	} else if runtime.GOOS == "windows" {
		if IsTerminalEditor(cmdName) {
			termApp := strings.ToLower(strings.TrimSpace(termOpts.App))
			if termApp == "" {
				if _, err := exec.LookPath("wt.exe"); err == nil {
					termApp = "wt"
				} else {
					termApp = "cmd"
				}
			}

			switch termApp {
			case "wt", "windowsterminal":
				var wtArgs []string
				if strings.EqualFold(termOpts.Target, "window") {
					wtArgs = []string{"-w", "-1", "-d", projectPath, cmdName}
				} else {
					wtArgs = []string{"-w", "0", "nt", "-d", projectPath, cmdName}
				}
				if len(args) > 0 {
					wtArgs = append(wtArgs, args...)
				} else {
					wtArgs = append(wtArgs, ".")
				}
				cmd = exec.Command("wt.exe", wtArgs...)
			default:
				startArgs := []string{"/c", "start", "", "/d", projectPath, cmdName}
				if len(args) > 0 {
					startArgs = append(startArgs, args...)
				} else {
					startArgs = append(startArgs, ".")
				}
				cmd = exec.Command("cmd.exe", startArgs...)
			}
		} else {
			startArgs := append([]string{"/c", "start", "", cmdName}, append(args, projectPath)...)
			cmd = exec.Command("cmd.exe", startArgs...)
		}
	} else {
		cmd = exec.Command(cmdName, append(args, projectPath)...)
	}

	// Detach process so GUI editors or background instances do not block
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start editor %q: %w", editorCmd, err)
	}

	return nil
}

func newWezTermCommand(projectPath, cmdName string, args []string, target string) *exec.Cmd {
	var wezArgs []string
	if strings.EqualFold(target, "tab") && os.Getenv("WEZTERM_PANE") != "" {
		wezArgs = []string{"cli", "spawn", "--cwd", projectPath, "--", cmdName}
	} else {
		wezArgs = []string{"start"}
		if strings.EqualFold(target, "tab") {
			wezArgs = append(wezArgs, "--new-tab")
		}
		wezArgs = append(wezArgs, "--cwd", projectPath, "--", cmdName)
	}
	if len(args) > 0 {
		wezArgs = append(wezArgs, args...)
	} else {
		wezArgs = append(wezArgs, ".")
	}
	return exec.Command("wezterm", wezArgs...)
}

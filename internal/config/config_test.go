package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg, err := DefaultConfig()
	if err != nil {
		t.Fatalf("DefaultConfig failed: %v", err)
	}
	if len(cfg.SearchPaths) == 0 {
		t.Errorf("expected non-empty SearchPaths")
	}
	if cfg.MaxDepth != DefaultMaxDepth {
		t.Errorf("expected MaxDepth=%d, got %d", DefaultMaxDepth, cfg.MaxDepth)
	}
	if len(cfg.IgnoredDirs) == 0 {
		t.Errorf("expected non-empty IgnoredDirs")
	}
	if cfg.RecentProjectsLimit != DefaultRecentProjects {
		t.Errorf("expected RecentProjectsLimit=%d, got %d", DefaultRecentProjects, cfg.RecentProjectsLimit)
	}
}

func TestSaveAndLoadCustom(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("USERPROFILE")
	if origHome == "" {
		origHome = os.Getenv("HOME")
	}

	t.Setenv("USERPROFILE", tmpDir)
	t.Setenv("HOME", tmpDir)

	cfg := &Config{
		SearchPaths:         []string{tmpDir},
		MaxDepth:            2,
		IgnoredDirs:         []string{".git", "vendor"},
		Editor:              "nvim",
		RecentProjectsLimit: MaxRecentProjects,
	}

	if err := cfg.Save(); err != nil {
		t.Fatalf("cfg.Save failed: %v", err)
	}

	expectedPath := filepath.Join(tmpDir, ".config", ConfigDirName, ConfigLuaFileName)
	if _, err := os.Stat(expectedPath); err != nil {
		t.Fatalf("config file was not created at expected path %s: %v", expectedPath, err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.Editor != "nvim" {
		t.Errorf("expected Editor=nvim, got %s", loaded.Editor)
	}
	if loaded.MaxDepth != 2 {
		t.Errorf("expected MaxDepth=2, got %d", loaded.MaxDepth)
	}
	if loaded.Terminal.App != "wt" {
		t.Errorf("expected Terminal.App=wt, got %s", loaded.Terminal.App)
	}
	if loaded.RecentProjectsLimit != MaxRecentProjects {
		t.Errorf("expected RecentProjectsLimit=%d, got %d", MaxRecentProjects, loaded.RecentProjectsLimit)
	}
}

func TestRecordRecentProject(t *testing.T) {
	cfg := &Config{RecentProjectsLimit: 5}
	for _, path := range []string{"one", "two", "three", "four", "five", "six", "three"} {
		cfg.RecordRecentProject(path)
	}

	want := []string{"three", "six", "five", "four", "two"}
	if !reflect.DeepEqual(cfg.RecentProjects, want) {
		t.Errorf("expected RecentProjects=%v, got %v", want, cfg.RecentProjects)
	}
}

func TestSaveAndLoadJSONRecentProjects(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("USERPROFILE", tmpDir)
	t.Setenv("HOME", tmpDir)

	configDir := filepath.Join(tmpDir, ".config", ConfigDirName)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("creating config directory: %v", err)
	}

	initial := &Config{
		SearchPaths:         []string{tmpDir},
		MaxDepth:            2,
		IgnoredDirs:         []string{".git"},
		Editor:              "nvim",
		RecentProjectsLimit: 6,
		RecentProjects:      []string{"older"},
	}
	data, err := json.Marshal(initial)
	if err != nil {
		t.Fatalf("marshaling initial JSON config: %v", err)
	}
	jsonPath := filepath.Join(configDir, ConfigJSONFileName)
	if err := os.WriteFile(jsonPath, data, 0644); err != nil {
		t.Fatalf("writing JSON config: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	loaded.RecordRecentProject("newer")
	if err := loaded.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(configDir, ConfigLuaFileName)); !os.IsNotExist(err) {
		t.Fatalf("unexpected Lua config after saving JSON config: %v", err)
	}
	var saved Config
	savedData, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("reading saved JSON config: %v", err)
	}
	if err := json.Unmarshal(savedData, &saved); err != nil {
		t.Fatalf("unmarshaling saved JSON config: %v", err)
	}
	want := []string{"newer", "older"}
	if !reflect.DeepEqual(saved.RecentProjects, want) {
		t.Errorf("expected RecentProjects=%v, got %v", want, saved.RecentProjects)
	}
}

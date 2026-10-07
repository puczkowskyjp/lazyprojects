package config

import (
	"os"
	"path/filepath"
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
		SearchPaths: []string{tmpDir},
		MaxDepth:    2,
		IgnoredDirs: []string{".git", "vendor"},
		Editor:      "nvim",
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
}

package config

import (
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
		AvailableEditors:    []string{"nvim", "vs.exe"},
		FavoriteProjects:    []string{filepath.Join(tmpDir, "favorite")},
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
	if !reflect.DeepEqual(loaded.AvailableEditors, cfg.AvailableEditors) {
		t.Errorf("expected AvailableEditors=%v, got %v", cfg.AvailableEditors, loaded.AvailableEditors)
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
	if !reflect.DeepEqual(loaded.FavoriteProjects, cfg.FavoriteProjects) {
		t.Errorf("expected FavoriteProjects=%v, got %v", cfg.FavoriteProjects, loaded.FavoriteProjects)
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

func TestToggleFavoriteProject(t *testing.T) {
	cfg := &Config{}
	if !cfg.ToggleFavoriteProject(filepath.Join("projects", ".", "one")) {
		t.Fatal("expected project to be added to favorites")
	}
	if !cfg.ToggleFavoriteProject("projects/two") {
		t.Fatal("expected second project to be added to favorites")
	}
	if cfg.ToggleFavoriteProject(filepath.Join("projects", "one")) {
		t.Fatal("expected project to be removed from favorites")
	}

	want := []string{filepath.Join("projects", "two")}
	if !reflect.DeepEqual(cfg.FavoriteProjects, want) {
		t.Fatalf("expected FavoriteProjects=%v, got %v", want, cfg.FavoriteProjects)
	}
}

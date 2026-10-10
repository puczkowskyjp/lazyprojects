package ui

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/puczkowskyjp/lazyprojects/internal/config"
	"github.com/puczkowskyjp/lazyprojects/internal/model"
)

func TestVisibleRange(t *testing.T) {
	tests := []struct {
		name      string
		length    int
		selected  int
		height    int
		wantStart int
		wantEnd   int
	}{
		{
			name:      "empty list",
			length:    0,
			selected:  0,
			height:    5,
			wantStart: 0,
			wantEnd:   0,
		},
		{
			name:      "non-positive height",
			length:    10,
			selected:  3,
			height:    0,
			wantStart: 0,
			wantEnd:   0,
		},
		{
			name:      "selection within initial viewport",
			length:    10,
			selected:  2,
			height:    5,
			wantStart: 0,
			wantEnd:   5,
		},
		{
			name:      "selection scrolls down at bottom edge",
			length:    10,
			selected:  5,
			height:    5,
			wantStart: 1,
			wantEnd:   6,
		},
		{
			name:      "selection near list end",
			length:    10,
			selected:  9,
			height:    5,
			wantStart: 5,
			wantEnd:   10,
		},
		{
			name:      "viewport taller than list",
			length:    3,
			selected:  2,
			height:    8,
			wantStart: 0,
			wantEnd:   3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotStart, gotEnd := visibleRange(tt.length, tt.selected, tt.height)
			if gotStart != tt.wantStart || gotEnd != tt.wantEnd {
				t.Fatalf("visibleRange(%d, %d, %d) = (%d, %d), want (%d, %d)", tt.length, tt.selected, tt.height, gotStart, gotEnd, tt.wantStart, tt.wantEnd)
			}
		})
	}
}

func TestFavoritesBrowsingUsesDiscoveredFilteredProjects(t *testing.T) {
	onePath := filepath.Join("projects", "one")
	twoPath := filepath.Join("projects", "two")
	ui := &UI{
		config: &config.Config{
			FavoriteProjects: []string{onePath, filepath.Join("projects", "missing"), twoPath},
		},
		projects: []model.Project{
			{Name: "one", Path: onePath},
			{Name: "two", Path: twoPath},
		},
		activeSection: sectionFavorites,
	}
	ui.applyFilterLocked()
	ui.refreshFavoritesLocked()

	want := []string{"one", "two"}
	got := []string{ui.favorites[0].Name, ui.favorites[1].Name}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("favorites = %v, want %v", got, want)
	}
	if project, ok := ui.currentProjectLocked(); !ok || project.Name != "one" {
		t.Fatalf("current favorite = (%q, %t), want (one, true)", project.Name, ok)
	}
	ui.cursorDown(nil, nil)
	if project, ok := ui.currentProjectLocked(); !ok || project.Name != "two" {
		t.Fatalf("current favorite after navigation = (%q, %t), want (two, true)", project.Name, ok)
	}

	ui.updateSearchQuery("two")
	if len(ui.favorites) != 1 || ui.favorites[0].Name != "two" {
		t.Fatalf("filtered favorites = %v, want only project two", ui.favorites)
	}
}

func TestToggleFavoriteFromProjectListPersists(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	path := filepath.Join(home, "project")
	ui := &UI{
		config: &config.Config{},
		filtered: []model.Project{
			{Name: "project", Path: path},
		},
	}

	if err := ui.toggleFavorite(nil, nil); err != nil {
		t.Fatalf("adding favorite: %v", err)
	}
	if !reflect.DeepEqual(ui.config.FavoriteProjects, []string{path}) {
		t.Fatalf("FavoriteProjects after add = %v, want [%s]", ui.config.FavoriteProjects, path)
	}

	if err := ui.toggleFavorite(nil, nil); err != nil {
		t.Fatalf("removing favorite: %v", err)
	}
	if len(ui.config.FavoriteProjects) != 0 {
		t.Fatalf("FavoriteProjects after remove = %v, want empty", ui.config.FavoriteProjects)
	}

	loaded, err := config.Load()
	if err != nil {
		t.Fatalf("loading persisted config: %v", err)
	}
	if len(loaded.FavoriteProjects) != 0 {
		t.Fatalf("persisted FavoriteProjects = %v, want empty", loaded.FavoriteProjects)
	}
	if _, err := os.Stat(filepath.Join(home, ".config", config.ConfigDirName, config.ConfigLuaFileName)); err != nil {
		t.Fatalf("favorite config was not saved: %v", err)
	}
}

func TestCommitHash(t *testing.T) {
	tests := []struct {
		name   string
		line   string
		want   string
		wantOK bool
	}{
		{
			name:   "short git hash",
			line:   "a83f21c Add project filtering",
			want:   "a83f21c",
			wantOK: true,
		},
		{
			name:   "full git hash",
			line:   "0123456789abcdef0123456789abcdef01234567 Commit message",
			want:   "0123456789abcdef0123456789abcdef01234567",
			wantOK: true,
		},
		{
			name:   "non-hash prefix",
			line:   "HEAD -> main",
			want:   "",
			wantOK: false,
		},
		{
			name:   "too short",
			line:   "abc123 Fix",
			want:   "",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := commitHash(tt.line)
			if got != tt.want || ok != tt.wantOK {
				t.Fatalf("commitHash(%q) = (%q, %t), want (%q, %t)", tt.line, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestColorizeCommitLine(t *testing.T) {
	got := colorizeCommitLine("a83f21c Add project filtering", "a83f21c")
	want := "\x1b[1;35ma83f21c\x1b[0m Add project filtering"
	if got != want {
		t.Fatalf("colorizeCommitLine() = %q, want %q", got, want)
	}
}

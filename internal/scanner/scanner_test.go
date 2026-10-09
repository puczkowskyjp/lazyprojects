package scanner

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/puczkowskyjp/lazyprojects/internal/config"
)

func TestScanner(t *testing.T) {
	root := t.TempDir()

	// 1. Create a Go project
	goDir := filepath.Join(root, "my-go-app")
	if err := os.MkdirAll(goDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(goDir, "go.mod"), []byte("module my-go-app\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// 2. Create a Node project with mock .git
	nodeDir := filepath.Join(root, "my-node-app")
	if err := os.MkdirAll(filepath.Join(nodeDir, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nodeDir, "package.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nodeDir, ".git", "HEAD"), []byte("ref: refs/heads/main\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// 3. Create an ignored directory
	vendorDir := filepath.Join(root, "vendor", "ignore-me")
	if err := os.MkdirAll(vendorDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vendorDir, "go.mod"), []byte("module ignored\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		SearchPaths: []string{root},
		MaxDepth:    3,
		IgnoredDirs: []string{"vendor", "node_modules"},
	}

	s := New(cfg)
	projects, err := s.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan returned unexpected error: %v", err)
	}

	if len(projects) != 2 {
		t.Fatalf("expected 2 projects, got %d: %+v", len(projects), projects)
	}

	names := map[string]string{}
	for _, p := range projects {
		names[p.Name] = p.Type
		if p.Name == "my-node-app" && p.Branch != "main" {
			t.Errorf("expected branch main for my-node-app, got %q", p.Branch)
		}
	}

	if names["my-go-app"] != "Go" {
		t.Errorf("expected my-go-app to be Go, got %q", names["my-go-app"])
	}
	if names["my-node-app"] != "Node" {
		t.Errorf("expected my-node-app to be Node, got %q", names["my-node-app"])
	}
}

func TestScannerDetectsSearchRootDotNetSolutionOnce(t *testing.T) {
	root := t.TempDir()

	if err := os.WriteFile(filepath.Join(root, "MyDotNetProject.sln"), []byte("Microsoft Visual Studio Solution File"), 0644); err != nil {
		t.Fatal(err)
	}

	subprojects := []string{"Api", "Domain", "ClientApp"}
	for _, name := range subprojects {
		dir := filepath.Join(root, name)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name+".csproj"), []byte("<Project Sdk=\"Microsoft.NET.Sdk\"></Project>"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	cfg := &config.Config{
		SearchPaths: []string{root},
		MaxDepth:    3,
		IgnoredDirs: []string{"vendor", "node_modules"},
	}

	s := New(cfg)
	projects, err := s.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan returned unexpected error: %v", err)
	}

	if len(projects) != 1 {
		t.Fatalf("expected 1 project, got %d: %+v", len(projects), projects)
	}

	if projects[0].Name != filepath.Base(root) {
		t.Fatalf("expected project name %q, got %q", filepath.Base(root), projects[0].Name)
	}
	if projects[0].Type != ".NET" {
		t.Fatalf("expected project type .NET, got %q", projects[0].Type)
	}
}

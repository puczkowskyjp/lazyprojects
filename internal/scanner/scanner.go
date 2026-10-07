package scanner

import (
	"context"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"lazyprojects/internal/config"
	"lazyprojects/internal/model"
)

// Scanner handles discovery of projects across configured search paths.
type Scanner struct {
	config *config.Config
}

// New creates a new Scanner instance with the given configuration.
func New(cfg *config.Config) *Scanner {
	return &Scanner{config: cfg}
}

// Scan crawls all configured search paths and returns discovered projects sorted by last modified.
func (s *Scanner) Scan(ctx context.Context) ([]model.Project, error) {
	if s.config == nil {
		return nil, nil
	}

	ignoredMap := make(map[string]bool)
	for _, ign := range s.config.IgnoredDirs {
		ignoredMap[strings.ToLower(ign)] = true
	}

	projects := make([]model.Project, 0)
	seenPaths := make(map[string]bool)

	for _, searchRoot := range s.config.SearchPaths {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		cleanRoot := filepath.Clean(searchRoot)
		info, err := os.Stat(cleanRoot)
		if err != nil || !info.IsDir() {
			continue
		}

		err = filepath.WalkDir(cleanRoot, func(currentPath string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return nil
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			name := d.Name()

			// Skip ignored directory names
			if d.IsDir() {
				if ignoredMap[strings.ToLower(name)] {
					return filepath.SkipDir
				}

				// Check depth relative to search root
				rel, err := filepath.Rel(cleanRoot, currentPath)
				if err == nil && rel != "." {
					depth := len(strings.Split(rel, string(filepath.Separator)))
					if depth > s.config.MaxDepth {
						return filepath.SkipDir
					}
				}
			} else {
				return nil
			}

			// Do not process searchRoot itself as a project unless marked
			if cleanRoot == currentPath {
				return nil
			}

			proj, isProj := s.detectProject(currentPath)
			if isProj {
				if !seenPaths[proj.Path] {
					seenPaths[proj.Path] = true
					projects = append(projects, proj)
				}
				// Stop descending into the project directory
				return filepath.SkipDir
			}

			return nil
		})

		if err != nil && err != context.Canceled {
			return nil, err
		}
	}

	// Sort projects by LastModified descending
	sort.Slice(projects, func(i, j int) bool {
		return projects[i].LastModified.After(projects[j].LastModified)
	})

	return projects, nil
}

// detectProject inspects a directory for indicators of a project.
func (s *Scanner) detectProject(dirPath string) (model.Project, bool) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return model.Project{}, false
	}

	hasGit := false
	projectType := ""
	var latestMod time.Time

	for _, entry := range entries {
		name := entry.Name()
		lower := strings.ToLower(name)

		if info, err := entry.Info(); err == nil {
			if info.ModTime().After(latestMod) {
				latestMod = info.ModTime()
			}
		}

		if entry.IsDir() && lower == ".git" {
			hasGit = true
			continue
		}

		if !entry.IsDir() {
			switch {
			case lower == "go.mod":
				projectType = "Go"
			case lower == "package.json" && projectType == "":
				projectType = "Node"
			case lower == "cargo.toml" && projectType == "":
				projectType = "Rust"
			case (lower == "pyproject.toml" || lower == "requirements.txt") && projectType == "":
				projectType = "Python"
			case (strings.HasSuffix(lower, ".sln") || strings.HasSuffix(lower, ".csproj")) && projectType == "":
				projectType = ".NET"
			}
		}
	}

	if projectType == "" && hasGit {
		projectType = "Git"
	}

	if projectType == "" {
		return model.Project{}, false
	}

	branch := ""
	if hasGit {
		branch = s.resolveGitBranch(dirPath)
	}

	dirInfo, err := os.Stat(dirPath)
	if err == nil && dirInfo.ModTime().After(latestMod) {
		latestMod = dirInfo.ModTime()
	}

	return model.Project{
		Name:         filepath.Base(dirPath),
		Path:         dirPath,
		Type:         projectType,
		Branch:       branch,
		LastModified: latestMod,
	}, true
}

// resolveGitBranch quickly inspects .git/HEAD or falls back to git command.
func (s *Scanner) resolveGitBranch(dirPath string) string {
	headPath := filepath.Join(dirPath, ".git", "HEAD")
	data, err := os.ReadFile(headPath)
	if err == nil {
		content := strings.TrimSpace(string(data))
		if strings.HasPrefix(content, "ref: refs/heads/") {
			return strings.TrimPrefix(content, "ref: refs/heads/")
		}
		if len(content) >= 7 {
			return content[:7]
		}
	}

	// Fallback to git CLI if available
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "-C", dirPath, "branch", "--show-current")
	out, err := cmd.Output()
	if err == nil {
		return strings.TrimSpace(string(out))
	}

	return ""
}

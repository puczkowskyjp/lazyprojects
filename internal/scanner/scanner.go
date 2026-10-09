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

	"github.com/puczkowskyjp/lazyprojects/internal/config"
	"github.com/puczkowskyjp/lazyprojects/internal/model"
)

const nestedProjectProbeMaxDepth = 3

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

		if proj, isProj := s.detectProject(cleanRoot, false, 0); isProj {
			if !seenPaths[proj.Path] {
				seenPaths[proj.Path] = true
				projects = append(projects, proj)
			}
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

			if d.IsDir() {
				if ignoredMap[strings.ToLower(name)] {
					return filepath.SkipDir
				}

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

			if cleanRoot == currentPath {
				return nil
			}

			remainingDepth := s.remainingDepth(cleanRoot, currentPath)
			proj, isProj := s.detectProject(currentPath, true, remainingDepth)
			if isProj {
				if !seenPaths[proj.Path] {
					seenPaths[proj.Path] = true
					projects = append(projects, proj)
				}
				return filepath.SkipDir
			}

			return nil
		})

		if err != nil && err != context.Canceled {
			return nil, err
		}
	}

	sort.Slice(projects, func(i, j int) bool {
		return projects[i].LastModified.After(projects[j].LastModified)
	})

	return projects, nil
}

func (s *Scanner) remainingDepth(cleanRoot, currentPath string) int {
	if s.config == nil {
		return 0
	}

	remainingDepth := s.config.MaxDepth
	rel, err := filepath.Rel(cleanRoot, currentPath)
	if err != nil || rel == "." {
		return remainingDepth
	}

	currentDepth := len(strings.Split(rel, string(filepath.Separator)))
	remainingDepth -= currentDepth
	if remainingDepth < 0 {
		return 0
	}

	return remainingDepth
}

// detectProject inspects a directory for indicators of a project.
func (s *Scanner) detectProject(dirPath string, allowGitOnly bool, remainingDepth int) (model.Project, bool) {
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
	}

	projectType = directProjectType(entries)
	if projectType == "" {
		probeDepth := min(remainingDepth, nestedProjectProbeMaxDepth)
		if nestedType, ok := s.detectNestedProjectType(dirPath, probeDepth); ok {
			projectType = nestedType
		}
	}

	if projectType == "" && hasGit && allowGitOnly {
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

func (s *Scanner) detectNestedProjectType(dirPath string, remainingDepth int) (string, bool) {
	if remainingDepth <= 0 {
		return "", false
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return "", false
	}

	if projectType := directProjectType(entries); projectType != "" {
		return projectType, true
	}

	childDirs := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		name := entry.Name()
		lower := strings.ToLower(name)
		if lower == ".git" || strings.HasPrefix(name, ".") || s.isIgnoredDir(lower) {
			continue
		}

		childDirs = append(childDirs, filepath.Join(dirPath, name))
	}

	if len(childDirs) == 1 {
		return s.detectNestedProjectType(childDirs[0], remainingDepth-1)
	}

	if len(childDirs) < 2 {
		return "", false
	}

	return s.detectConsistentChildProjectType(childDirs, remainingDepth-1)
}

func (s *Scanner) detectConsistentChildProjectType(childDirs []string, remainingDepth int) (string, bool) {
	if remainingDepth <= 0 {
		return "", false
	}

	projectType := ""
	matchCount := 0

	for _, childDir := range childDirs {
		childType, ok := s.detectNestedProjectType(childDir, remainingDepth)
		if !ok {
			continue
		}
		if projectType == "" {
			projectType = childType
		} else if childType != projectType {
			return "", false
		}
		matchCount++
	}

	if projectType == "" || matchCount < 2 {
		return "", false
	}

	return projectType, true
}

func directProjectType(entries []os.DirEntry) string {
	projectType := ""

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		lower := strings.ToLower(entry.Name())
		switch {
		case lower == "go.mod":
			return "Go"
		case lower == "package.json" && projectType == "":
			projectType = "Node"
		case lower == "cargo.toml" && projectType == "":
			projectType = "Rust"
		case (lower == "pyproject.toml" || lower == "requirements.txt") && projectType == "":
			projectType = "Python"
		case (strings.HasSuffix(lower, ".sln") || strings.HasSuffix(lower, ".slnx") || strings.HasSuffix(lower, ".csproj")) && projectType == "":
			projectType = ".NET"
		}
	}

	return projectType
}

func (s *Scanner) isIgnoredDir(name string) bool {
	if s.config == nil {
		return false
	}

	for _, ignored := range s.config.IgnoredDirs {
		if strings.EqualFold(name, ignored) {
			return true
		}
	}

	return false
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

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "-C", dirPath, "branch", "--show-current")
	out, err := cmd.Output()
	if err == nil {
		return strings.TrimSpace(string(out))
	}

	return ""
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

const (
	ConfigDirName     = "lazyprojects"
	ConfigLuaFileName = "config.lua"
	ConfigJSONFileName = "config.json"
	DefaultMaxDepth   = 4
)

// TerminalConfig configures how terminal-based editors are launched.
type TerminalConfig struct {
	App    string `json:"app"`    // e.g. "wt" (Windows Terminal), "wezterm", "cmd"
	Target string `json:"target"` // "tab" (new tab in active window) or "window" (new window)
}

// Config holds user configuration for lazyprojects.
type Config struct {
	SearchPaths []string       `json:"search_paths"`
	MaxDepth    int            `json:"max_depth"`
	IgnoredDirs []string       `json:"ignored_dirs"`
	Editor      string         `json:"editor"`
	Terminal    TerminalConfig `json:"terminal"`
}

// DefaultIgnoredDirs returns a standard slice of directory names to skip.
func DefaultIgnoredDirs() []string {
	return []string{
		".git",
		"node_modules",
		"vendor",
		"bin",
		"obj",
		"dist",
		"build",
		"target",
		".vscode",
		".idea",
	}
}

// GetConfigDir returns the path to ~/.config/lazyprojects.
func GetConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to determine user home directory: %w", err)
	}
	return filepath.Join(home, ".config", ConfigDirName), nil
}

// GetLuaConfigFilePath returns the path to ~/.config/lazyprojects/config.lua.
func GetLuaConfigFilePath() (string, error) {
	dir, err := GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, ConfigLuaFileName), nil
}

// DefaultConfig generates a default configuration based on the environment.
func DefaultConfig() (*Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to determine user home directory: %w", err)
	}

	searchPaths := []string{}
	projectsDir := filepath.Join(home, "Documents", "Projects")
	if info, err := os.Stat(projectsDir); err == nil && info.IsDir() {
		searchPaths = append(searchPaths, projectsDir)
	} else {
		searchPaths = append(searchPaths, home)
	}

	editor := "nvim"
	if envEditor := os.Getenv("EDITOR"); envEditor != "" {
		editor = envEditor
	}

	return &Config{
		SearchPaths: searchPaths,
		MaxDepth:    DefaultMaxDepth,
		IgnoredDirs: DefaultIgnoredDirs(),
		Editor:      editor,
		Terminal: TerminalConfig{
			App:    "wt",
			Target: "tab",
		},
	}, nil
}

// Load loads the configuration from ~/.config/lazyprojects/config.lua (falling back to config.json).
// If neither exists, a default config.lua is generated and saved.
func Load() (*Config, error) {
	luaPath, err := GetLuaConfigFilePath()
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(luaPath); err == nil {
		return loadFromLua(luaPath)
	}

	// Check legacy JSON config fallback
	dir, _ := GetConfigDir()
	jsonPath := filepath.Join(dir, ConfigJSONFileName)
	if _, err := os.Stat(jsonPath); err == nil {
		return loadFromJSON(jsonPath)
	}

	// Generate default config.lua
	cfg, defErr := DefaultConfig()
	if defErr != nil {
		return nil, defErr
	}
	if saveErr := cfg.Save(); saveErr != nil {
		return nil, saveErr
	}
	return cfg, nil
}

func loadFromLua(path string) (*Config, error) {
	L := lua.NewState()
	defer L.Close()

	if err := L.DoFile(path); err != nil {
		return nil, fmt.Errorf("failed to execute lua config: %w", err)
	}

	// Check return value or global 'config'
	var tbl *lua.LTable
	ret := L.Get(-1)
	if t, ok := ret.(*lua.LTable); ok {
		tbl = t
	} else {
		glob := L.GetGlobal("config")
		if t, ok := glob.(*lua.LTable); ok {
			tbl = t
		}
	}

	if tbl == nil {
		return nil, fmt.Errorf("lua config must return a table or define a global 'config' table")
	}

	def, _ := DefaultConfig()
	cfg := *def

	if paths := getLuaStringSlice(tbl, "search_paths"); len(paths) > 0 {
		cfg.SearchPaths = paths
	}
	if maxDepth := getLuaInt(tbl, "max_depth"); maxDepth > 0 {
		cfg.MaxDepth = maxDepth
	}
	if ignored := getLuaStringSlice(tbl, "ignored_dirs"); len(ignored) > 0 {
		cfg.IgnoredDirs = ignored
	}
	if ed := getLuaString(tbl, "editor"); ed != "" {
		cfg.Editor = ed
	}

	if termVal := tbl.RawGetString("terminal"); termVal != lua.LNil {
		if termTbl, ok := termVal.(*lua.LTable); ok {
			if app := getLuaString(termTbl, "app"); app != "" {
				cfg.Terminal.App = app
			}
			if target := getLuaString(termTbl, "target"); target != "" {
				cfg.Terminal.Target = target
			}
		}
	}

	return &cfg, nil
}

func loadFromJSON(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if cfg.MaxDepth <= 0 {
		cfg.MaxDepth = DefaultMaxDepth
	}
	if len(cfg.IgnoredDirs) == 0 {
		cfg.IgnoredDirs = DefaultIgnoredDirs()
	}
	if cfg.Terminal.App == "" {
		cfg.Terminal.App = "wt"
	}
	if cfg.Terminal.Target == "" {
		cfg.Terminal.Target = "tab"
	}
	return &cfg, nil
}

func getLuaString(tbl *lua.LTable, key string) string {
	v := tbl.RawGetString(key)
	if str, ok := v.(lua.LString); ok {
		return string(str)
	}
	return ""
}

func getLuaInt(tbl *lua.LTable, key string) int {
	v := tbl.RawGetString(key)
	if num, ok := v.(lua.LNumber); ok {
		return int(num)
	}
	return 0
}

func getLuaStringSlice(tbl *lua.LTable, key string) []string {
	v := tbl.RawGetString(key)
	sliceTbl, ok := v.(*lua.LTable)
	if !ok {
		return nil
	}
	var res []string
	sliceTbl.ForEach(func(_, val lua.LValue) {
		if str, ok := val.(lua.LString); ok {
			res = append(res, string(str))
		}
	})
	return res
}

// Save writes the configuration to ~/.config/lazyprojects/config.lua.
func (c *Config) Save() error {
	dir, err := GetConfigDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory %q: %w", dir, err)
	}

	path := filepath.Join(dir, ConfigLuaFileName)

	var sb strings.Builder
	sb.WriteString("-- lazyprojects configuration\n")
	sb.WriteString("-- Location: ~/.config/lazyprojects/config.lua\n\n")
	sb.WriteString("return {\n")

	// search_paths
	sb.WriteString("  -- Base directories to scan for projects\n")
	sb.WriteString("  search_paths = {\n")
	for _, p := range c.SearchPaths {
		escaped := strings.ReplaceAll(p, "\\", "\\\\")
		sb.WriteString(fmt.Sprintf("    %q,\n", escaped))
	}
	sb.WriteString("  },\n\n")

	// max_depth
	sb.WriteString(fmt.Sprintf("  -- Maximum directory recursion depth\n  max_depth = %d,\n\n", c.MaxDepth))

	// ignored_dirs
	sb.WriteString("  -- Directories to skip during scanning\n  ignored_dirs = {\n")
	for _, ign := range c.IgnoredDirs {
		sb.WriteString(fmt.Sprintf("    %q,\n", ign))
	}
	sb.WriteString("  },\n\n")

	// editor
	sb.WriteString(fmt.Sprintf("  -- Default editor command (\"nvim\", \"code\", \"notepad\", etc.)\n  editor = %q,\n\n", c.Editor))

	// terminal
	sb.WriteString("  -- Terminal settings for terminal-based editors (e.g. nvim, vim)\n")
	sb.WriteString("  terminal = {\n")
	sb.WriteString(fmt.Sprintf("    -- Terminal emulator: \"wt\" (Windows Terminal), \"wezterm\", \"cmd\"\n    app = %q,\n\n", c.Terminal.App))
	sb.WriteString(fmt.Sprintf("    -- Target launch mode: \"tab\" (new tab in current window) or \"window\" (new window)\n    target = %q,\n", c.Terminal.Target))
	sb.WriteString("  },\n")
	sb.WriteString("}\n")

	if err := os.WriteFile(path, []byte(sb.String()), 0644); err != nil {
		return fmt.Errorf("failed to write lua config file %q: %w", path, err)
	}

	return nil
}

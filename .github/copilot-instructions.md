# Copilot Instructions for lazyprojects

## Build, Test, and Lint Commands

- **Build binary**: `go build -o bin/lazyprojects.exe .`
- **Build all packages**: `go build ./...`
- **Run application**: `go run .`
- **Run all tests**: `go test ./...`
- **Run tests with verbose output**: `go test -v ./...`
- **Run single test package**: `go test -v ./internal/<pkg>`
- **Run single specific test**: `go test -v -run ^TestFunctionName$ ./internal/<pkg>`
- **Lint / Static analysis**:
  - `go vet ./...`
  - `staticcheck ./...` (available locally via `go/bin`)
- **Manage dependencies**: `go mod tidy`

## High-Level Architecture

`lazyprojects` is a Go-based CLI/TUI project management and repository navigation tool.

- **Entry Point (`main.go`)**: Parses CLI flags (`-list`, `-open`), initializes configuration, and triggers project discovery or TUI event loop.
- **`internal/`**:
  - **`config/`**: Manages configuration loaded from `~/.config/lazyprojects/config.lua` via embedded `gopher-lua`. Configures `search_paths`, `max_depth`, `ignored_dirs`, `editor`, and `terminal` (`app` and `target` for tab vs window).
  - **`scanner/`**: Traverses search paths up to `MaxDepth`, detecting project markers (Go, Node, Rust, Python, .NET, Git), resolving git branches from `.git/HEAD`, and sorting projects by last modified.
  - **`editor/`**: Probes available editors (`code`, `nvim`, `notepad`, `$EDITOR`) and launches them for a selected project path. For terminal editors (`nvim`), delegates to configured terminal emulator (`wt`, `wezterm`, `cmd`) in a new tab or window.
  - **`model/`**: Shared domain models (`Project`).
  - **`ui/`**: Interactive terminal views and keybinding handlers using `gocui`.

## Key Conventions

- **Path Handling**: Target environment is Windows. Never construct filesystem paths with manual string concatenation or hardcoded separators. Always use `path/filepath` (`filepath.Join`, `filepath.Clean`, `filepath.ToSlash`) to properly handle Windows drive letters and separators (`\`).
- **Context & Subprocess Management**: Execute external tools (e.g., `git`) using `exec.CommandContext(ctx, ...)`. Always wire cancellation contexts and propagate timeouts so hung processes or interrupted commands cleanly terminate.
- **Error Handling**: Follow standard Go error wrapping conventions: `fmt.Errorf("context message: %w", err)`. Inspect errors with `errors.Is` and `errors.As`. Do not use `panic` in operational or worker paths.
- **TUI/Terminal State**: Ensure terminal state restoration is deferred immediately upon screen initialization to prevent leaving the user's terminal in raw or unusable states upon exit or error.

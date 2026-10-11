# lazyprojects

`lazyprojects` is a terminal UI for discovering, searching, and opening local projects.

Configure one or more project directories, choose your preferred terminal and editor, then quickly find and open a project without navigating through your filesystem.

Git metadata such as the current branch and recent commits is also available while browsing projects.


## Features

* Discover projects across configurable directories
* Quickly search and filter projects
* Track recently opened projects
* Favorite projects for quick access
* Open projects with your preferred editor and terminal
* View Git metadata at a glance
* Configure with Lua


## Installation

`lazyprojects` requires [Go](https://go.dev/doc/install).

Install the latest version with:

```bash
go install github.com/puczkowskyjp/lazyprojects@latest
```

Make sure your Go binary directory is in your `PATH`, then run:

```bash
lazyprojects
```


## Configuration

On first launch, the application writes `~/.config/lazyprojects/config.lua`.

Use [`config.example.lua`](config.example.lua) as a starting point.

Set `recent_projects_limit` to retain between 5 and 10 recently opened projects; the default is 8. The application maintains the newest-first `recent_projects` list automatically after each successful project launch.

Set `available_editors` to limit the editors offered by the `e` key to installed editors in that list. Omit the setting or use an empty list to allow all supported editors. Supported commands include `code`, `vs.exe`, `nvim`, `vim`, `goland`, `idea`, `subl`, and `notepad`; Visual Studio's `vs.exe` must be reachable in `PATH`.


## Navigation

| Key                     | Action                                                   |
| ----------------------- | -------------------------------------------------------- |
| `[` / `]`               | Focus the filter, Recently Opened, Favorites, or Projects |
| `j` / `k` or arrow keys | Move within the selected project list                    |
| `Enter` / `o`           | Open the selected project                                |
| `f`                     | Add or remove the selected project from favorites        |
| `/`                     | Focus the filter                                         |

Recently Opened, Favorites, and Projects are shown in separate panes. The Favorites pane stays visible while browsing.


## Configuration Example

```lua
return {
  search_paths = {
    "~/work",
    "~/personal",
    "~/open-source",
  },

  max_depth = 4,

  ignored_dirs = {
    ".git",
    "node_modules",
    "vendor",
    "bin",
    "obj",
    "dist",
    "build",
    "target",
  },

  editor = "nvim",

  available_editors = {
    "nvim",
    "code",
    "vs.exe",
  },

  terminal = {
    app = "wezterm",
    target = "tab",
  },

  recent_projects_limit = 8,

  -- Managed automatically when a project is opened.
  recent_projects = {},
}
```

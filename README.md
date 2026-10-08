# lazyprojects

`lazyprojects` is a terminal UI for discovering, searching, and opening local projects.

Configure one or more project directories, choose your preferred terminal and editor, then quickly find and open a project without navigating through your filesystem.

Git metadata such as the current branch and recent commits is also available while browsing projects.

## Features

* Discover projects across configurable directories
* Quickly search and filter projects
* Track recently opened projects
* Open projects with your preferred editor and terminal
* View Git metadata at a glance
* Configure with Lua or JSON

## Configuration

On first launch, the application writes `~/.config/lazyprojects/config.lua`.

JSON is also supported at `~/.config/lazyprojects/config.json` when no Lua configuration exists.

Use [`config.example.lua`](config.example.lua) or [`config.example.json`](config.example.json) as a starting point.

Set `recent_projects_limit` to retain between 5 and 10 recently opened projects; the default is 8. The application maintains the newest-first `recent_projects` list automatically after each successful project launch.

## Navigation

| Key                     | Action                                                   |
| ----------------------- | -------------------------------------------------------- |
| `[` / `]`               | Switch between filter, recent projects, and all projects |
| `j` / `k` or arrow keys | Move within the selected project list                    |
| `Enter` / `o`           | Open the selected project                                |
| `/`                     | Focus the filter                                         |

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

  terminal = {
    app = "wezterm",
    target = "tab",
  },

  recent_projects_limit = 8,

  -- Managed automatically when a project is opened.
  recent_projects = {},
}
```

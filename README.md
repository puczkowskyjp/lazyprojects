# lazyprojects

`lazyprojects` discovers, searches, and opens local projects from a terminal UI.

## Configuration

On first launch, the application writes `~/.config/lazyprojects/config.lua`. JSON is also supported at `~/.config/lazyprojects/config.json` when no Lua configuration exists. Use [`config.example.lua`](config.example.lua) or [`config.example.json`](config.example.json) as a starting point.

Set `recent_projects_limit` to retain between 5 and 10 recently opened projects; the default is 8. The application maintains the newest-first `recent_projects` list automatically after each successful project launch.

## Navigation

- `[` / `]`: switch between the filter, recently opened projects, and all projects.
- `j` / `k` or arrow keys: move within the selected project list.
- `Enter` or `o`: open the selected project.
- `/`: focus the filter. Results update as you type.

-- Copy to ~/.config/lazyprojects/config.lua and replace the example path.
return {
  search_paths = {
    "C:/Users/your-user/Documents/Projects",
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

  editor = "code",

  available_editors = {
    "nvim",
    "code",
    "vs.exe",
  },

  terminal = {
    app = "wt",
    target = "tab",
  },

  -- Retain from 5 to 10 recently opened projects.
  recent_projects_limit = 8,

  -- Managed automatically when a project is opened.
  recent_projects = {},
}

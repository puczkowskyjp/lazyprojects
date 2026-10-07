package ui

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/jroimartin/gocui"
	"lazyprojects/internal/config"
	"lazyprojects/internal/editor"
	"lazyprojects/internal/model"
	"lazyprojects/internal/scanner"
)

const (
	ViewProjects = "projects"
	ViewDetails  = "details"
	ViewSearch   = "search"
	ViewStatus   = "status"
)

// UI manages the terminal interface and application state.
type UI struct {
	gui           *gocui.Gui
	config        *config.Config
	scanner       *scanner.Scanner
	projects      []model.Project
	filtered      []model.Project
	selectedIndex int
	searchQuery   string
	searching     bool
	statusMsg     string
	availableEds  []string
	currentEdIdx  int
	gitCache      map[string]string
	loadingGit    map[string]bool
	mu            sync.Mutex
}

// New creates and initializes a UI instance.
func New(cfg *config.Config) *UI {
	available := editor.DetectAvailable()
	edIdx := 0
	for i, ed := range available {
		if strings.EqualFold(ed, cfg.Editor) {
			edIdx = i
			break
		}
	}

	return &UI{
		config:       cfg,
		scanner:      scanner.New(cfg),
		availableEds: available,
		currentEdIdx: edIdx,
		gitCache:     make(map[string]string),
		loadingGit:   make(map[string]bool),
		statusMsg:    "Ready. Use ↑/↓ or j/k to browse, Enter to open.",
	}
}

// Run starts the gocui event loop.
func (u *UI) Run() error {
	g, err := gocui.NewGui(gocui.OutputNormal)
	if err != nil {
		return fmt.Errorf("failed to create GUI: %w", err)
	}
	defer g.Close()

	u.gui = g
	g.Cursor = false
	g.Highlight = true
	g.SelFgColor = gocui.ColorGreen | gocui.AttrBold

	u.refreshProjects()

	g.SetManagerFunc(u.layout)

	if err := u.setKeybindings(); err != nil {
		return fmt.Errorf("failed to set keybindings: %w", err)
	}

	if err := g.MainLoop(); err != nil && err != gocui.ErrQuit {
		return err
	}

	return nil
}

func (u *UI) refreshProjects() {
	projects, err := u.scanner.Scan(context.Background())
	u.mu.Lock()
	defer u.mu.Unlock()
	if err != nil {
		u.statusMsg = fmt.Sprintf("Error scanning: %v", err)
		return
	}
	u.projects = projects
	u.applyFilterLocked()
	u.statusMsg = fmt.Sprintf("Ready. Found %d projects. Active editor: [%s]", len(u.projects), u.currentEditor())
}

func (u *UI) currentEditor() string {
	if len(u.availableEds) > 0 {
		return u.availableEds[u.currentEdIdx]
	}
	if u.config.Editor != "" {
		return u.config.Editor
	}
	return "code"
}

func (u *UI) applyFilterLocked() {
	if strings.TrimSpace(u.searchQuery) == "" {
		u.filtered = make([]model.Project, len(u.projects))
		copy(u.filtered, u.projects)
	} else {
		query := strings.ToLower(strings.TrimSpace(u.searchQuery))
		filtered := make([]model.Project, 0)
		for _, p := range u.projects {
			if strings.Contains(strings.ToLower(p.Name), query) ||
				strings.Contains(strings.ToLower(p.Type), query) ||
				strings.Contains(strings.ToLower(p.Branch), query) ||
				strings.Contains(strings.ToLower(p.Path), query) {
				filtered = append(filtered, p)
			}
		}
		u.filtered = filtered
	}

	if u.selectedIndex >= len(u.filtered) {
		u.selectedIndex = len(u.filtered) - 1
	}
	if u.selectedIndex < 0 {
		u.selectedIndex = 0
	}
}

func (u *UI) layout(g *gocui.Gui) error {
	maxX, maxY := g.Size()
	if maxX < 40 || maxY < 10 {
		return nil
	}

	searchHeight := 3
	statusHeight := 4
	mainHeight := maxY - searchHeight - statusHeight

	// Search View
	if v, err := g.SetView(ViewSearch, 0, 0, maxX-1, searchHeight-1); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = " Filter Projects [/ to type, Enter/Esc to finish] "
		v.Editable = true
	}

	// Projects List View (left panel)
	listWidth := maxX * 45 / 100
	if listWidth < 30 {
		listWidth = 30
	}
	if v, err := g.SetView(ViewProjects, 0, searchHeight, listWidth, searchHeight+mainHeight); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = fmt.Sprintf(" Projects (%d) ", len(u.filtered))
		v.Highlight = true
		v.SelBgColor = gocui.ColorBlue
		v.SelFgColor = gocui.ColorWhite | gocui.AttrBold
	}

	// Details View (right panel)
	if v, err := g.SetView(ViewDetails, listWidth+1, searchHeight, maxX-1, searchHeight+mainHeight); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = " Project Details & Git Commits "
		v.Wrap = true
	}

	// Status & Keybindings View (bottom)
	if v, err := g.SetView(ViewStatus, 0, maxY-statusHeight, maxX-1, maxY-1); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = " Quick Keys & Status "
	}

	u.renderProjects(g)
	u.renderDetails(g)
	u.renderStatus(g)

	if !u.searching {
		if _, err := g.SetCurrentView(ViewProjects); err != nil {
			return err
		}
		g.Cursor = false
	} else {
		if _, err := g.SetCurrentView(ViewSearch); err != nil {
			return err
		}
		g.Cursor = true
	}

	return nil
}

func (u *UI) renderProjects(g *gocui.Gui) {
	v, err := g.View(ViewProjects)
	if err != nil {
		return
	}
	v.Clear()
	v.Title = fmt.Sprintf(" Projects (%d) ", len(u.filtered))

	u.mu.Lock()
	defer u.mu.Unlock()

	for i, p := range u.filtered {
		cursor := "  "
		if i == u.selectedIndex {
			cursor = "▶ "
		}
		branch := ""
		if p.Branch != "" {
			branch = fmt.Sprintf(" (%s)", p.Branch)
		}
		fmt.Fprintf(v, "%s%-20s [%-4s]%s\n", cursor, truncate(p.Name, 20), p.Type, branch)
	}
}

func (u *UI) renderDetails(g *gocui.Gui) {
	v, err := g.View(ViewDetails)
	if err != nil {
		return
	}
	v.Clear()

	u.mu.Lock()
	if len(u.filtered) == 0 || u.selectedIndex >= len(u.filtered) {
		u.mu.Unlock()
		fmt.Fprintln(v, "No projects matching criteria.")
		return
	}

	p := u.filtered[u.selectedIndex]
	ed := u.currentEditor()
	cachedGit, hasGitLog := u.gitCache[p.Path]
	isLoading := u.loadingGit[p.Path]
	u.mu.Unlock()

	fmt.Fprintf(v, "Action:   ▶ Press Enter or 'o' to open in [%s]\n", ed)
	fmt.Fprintf(v, "Project:  %s\n", p.Name)
	fmt.Fprintf(v, "Type:     %s\n", p.Type)
	fmt.Fprintf(v, "Path:     %s\n", p.Path)
	if p.Branch != "" {
		fmt.Fprintf(v, "Branch:   %s\n", p.Branch)
	}
	if !p.LastModified.IsZero() {
		fmt.Fprintf(v, "Modified: %s\n", p.LastModified.Format("2006-01-02 15:04:05"))
	}
	fmt.Fprintln(v, strings.Repeat("─", 50))

	if hasGitLog {
		if cachedGit != "" {
			fmt.Fprintln(v, "Recent Commits:")
			fmt.Fprint(v, cachedGit)
		} else {
			fmt.Fprintln(v, "(No git commit history)")
		}
	} else if isLoading {
		fmt.Fprintln(v, "Loading git history...")
	} else {
		// Fetch git history asynchronously
		u.mu.Lock()
		u.loadingGit[p.Path] = true
		u.mu.Unlock()

		go func(path string) {
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()

			cmd := exec.CommandContext(ctx, "git", "-C", path, "log", "-n", "5", "--oneline")
			out, _ := cmd.Output()
			result := string(out)

			u.mu.Lock()
			u.gitCache[path] = result
			delete(u.loadingGit, path)
			u.mu.Unlock()

			if u.gui != nil {
				u.gui.Update(func(g *gocui.Gui) error {
					return nil
				})
			}
		}(p.Path)
		fmt.Fprintln(v, "Loading git history...")
	}
}

func (u *UI) renderStatus(g *gocui.Gui) {
	v, err := g.View(ViewStatus)
	if err != nil {
		return
	}
	v.Clear()

	u.mu.Lock()
	msg := u.statusMsg
	ed := u.currentEditor()
	u.mu.Unlock()

	keys := fmt.Sprintf("[j/k or ↑/↓] Move | [Enter/o] Open (%s) | [/] Filter | [e] Switch Editor | [r] Rescan | [q] Quit", ed)
	fmt.Fprintf(v, "• %s\n• %s", msg, keys)
}

func (u *UI) setKeybindings() error {
	// Global Quit
	if err := u.gui.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, u.quit); err != nil {
		return err
	}

	// Projects View navigation
	navKeys := []interface{}{
		gocui.KeyArrowDown, 'j',
	}
	for _, k := range navKeys {
		if err := u.gui.SetKeybinding(ViewProjects, k, gocui.ModNone, u.cursorDown); err != nil {
			return err
		}
	}

	upKeys := []interface{}{
		gocui.KeyArrowUp, 'k',
	}
	for _, k := range upKeys {
		if err := u.gui.SetKeybinding(ViewProjects, k, gocui.ModNone, u.cursorUp); err != nil {
			return err
		}
	}

	// Open Project
	openKeys := []interface{}{
		gocui.KeyEnter, 'o',
	}
	for _, k := range openKeys {
		if err := u.gui.SetKeybinding(ViewProjects, k, gocui.ModNone, u.openProject); err != nil {
			return err
		}
	}

	// Start Search
	if err := u.gui.SetKeybinding(ViewProjects, '/', gocui.ModNone, u.startSearch); err != nil {
		return err
	}

	// Rescan
	if err := u.gui.SetKeybinding(ViewProjects, 'r', gocui.ModNone, u.rescan); err != nil {
		return err
	}

	// Switch Editor
	if err := u.gui.SetKeybinding(ViewProjects, 'e', gocui.ModNone, u.cycleEditor); err != nil {
		return err
	}

	// Quit with q
	if err := u.gui.SetKeybinding(ViewProjects, 'q', gocui.ModNone, u.quit); err != nil {
		return err
	}

	// Search View Keybindings
	if err := u.gui.SetKeybinding(ViewSearch, gocui.KeyEsc, gocui.ModNone, u.stopSearch); err != nil {
		return err
	}
	if err := u.gui.SetKeybinding(ViewSearch, gocui.KeyEnter, gocui.ModNone, u.confirmSearch); err != nil {
		return err
	}

	return nil
}

func (u *UI) cursorDown(g *gocui.Gui, v *gocui.View) error {
	u.mu.Lock()
	if u.selectedIndex < len(u.filtered)-1 {
		u.selectedIndex++
	}
	u.mu.Unlock()
	return nil
}

func (u *UI) cursorUp(g *gocui.Gui, v *gocui.View) error {
	u.mu.Lock()
	if u.selectedIndex > 0 {
		u.selectedIndex--
	}
	u.mu.Unlock()
	return nil
}

func (u *UI) openProject(g *gocui.Gui, v *gocui.View) error {
	u.mu.Lock()
	if len(u.filtered) == 0 || u.selectedIndex >= len(u.filtered) {
		u.mu.Unlock()
		return nil
	}
	proj := u.filtered[u.selectedIndex]
	ed := u.currentEditor()
	termOpts := editor.TerminalOptions{
		App:    u.config.Terminal.App,
		Target: u.config.Terminal.Target,
	}
	u.statusMsg = fmt.Sprintf("Opening %s in %s...", proj.Name, ed)
	u.mu.Unlock()

	// Launch editor completely asynchronously to avoid freezing TUI
	go func(name, path, editorCmd string, to editor.TerminalOptions) {
		err := editor.OpenWithOptions(editorCmd, path, to)
		if u.gui != nil {
			u.gui.Update(func(g *gocui.Gui) error {
				u.mu.Lock()
				if err != nil {
					u.statusMsg = fmt.Sprintf("Error opening %s: %v", name, err)
				} else {
					u.statusMsg = fmt.Sprintf("Successfully opened %s in %s", name, editorCmd)
				}
				u.mu.Unlock()
				return nil
			})
		}
	}(proj.Name, proj.Path, ed, termOpts)

	return nil
}

func (u *UI) cycleEditor(g *gocui.Gui, v *gocui.View) error {
	u.mu.Lock()
	defer u.mu.Unlock()
	if len(u.availableEds) > 1 {
		u.currentEdIdx = (u.currentEdIdx + 1) % len(u.availableEds)
		u.config.Editor = u.availableEds[u.currentEdIdx]
		_ = u.config.Save()
		u.statusMsg = fmt.Sprintf("Switched active editor to: [%s]", u.config.Editor)
	}
	return nil
}

func (u *UI) startSearch(g *gocui.Gui, v *gocui.View) error {
	u.mu.Lock()
	u.searching = true
	u.mu.Unlock()
	return nil
}

func (u *UI) stopSearch(g *gocui.Gui, v *gocui.View) error {
	u.mu.Lock()
	u.searching = false
	u.searchQuery = ""
	u.applyFilterLocked()
	u.statusMsg = "Search cleared. Browsing all projects."
	u.mu.Unlock()

	sv, err := g.View(ViewSearch)
	if err == nil {
		sv.Clear()
	}
	return nil
}

func (u *UI) confirmSearch(g *gocui.Gui, v *gocui.View) error {
	buf := v.Buffer()
	u.mu.Lock()
	u.searchQuery = strings.TrimSpace(buf)
	u.applyFilterLocked()
	u.searching = false
	u.statusMsg = fmt.Sprintf("Filter: %q (%d matches)", u.searchQuery, len(u.filtered))
	u.mu.Unlock()
	return nil
}

func (u *UI) rescan(g *gocui.Gui, v *gocui.View) error {
	u.mu.Lock()
	u.statusMsg = "Rescanning projects..."
	u.gitCache = make(map[string]string)
	u.mu.Unlock()

	go func() {
		u.refreshProjects()
		if u.gui != nil {
			u.gui.Update(func(g *gocui.Gui) error {
				return nil
			})
		}
	}()
	return nil
}

func (u *UI) quit(g *gocui.Gui, v *gocui.View) error {
	return gocui.ErrQuit
}

func truncate(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen-1] + "…"
	}
	return s
}

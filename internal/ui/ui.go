package ui

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/jroimartin/gocui"
	"github.com/puczkowskyjp/lazyprojects/internal/config"
	"github.com/puczkowskyjp/lazyprojects/internal/editor"
	"github.com/puczkowskyjp/lazyprojects/internal/model"
	"github.com/puczkowskyjp/lazyprojects/internal/scanner"
)

const (
	ViewProjects  = "projects"
	ViewRecent    = "recent"
	ViewFavorites = "favorites"
	ViewDetails   = "details"
	ViewSearch    = "search"
	ViewStatus    = "status"
)

type section int

const (
	sectionSearch section = iota
	sectionRecent
	sectionFavorites
	sectionProjects
	sectionCount
)

// UI manages the terminal interface and application state.
type UI struct {
	gui                   *gocui.Gui
	config                *config.Config
	scanner               *scanner.Scanner
	projects              []model.Project
	filtered              []model.Project
	recent                []model.Project
	favorites             []model.Project
	selectedIndex         int
	selectedRecentIndex   int
	selectedFavoriteIndex int
	searchQuery           string
	activeSection         section
	statusMsg             string
	availableEds          []string
	currentEdIdx          int
	gitCache              map[string]string
	loadingGit            map[string]bool
	Version               string
	mu                    sync.Mutex
}

// New creates and initializes a UI instance.
func New(cfg *config.Config, version string) *UI {
	available := editor.DetectAvailable()
	edIdx := 0
	for i, ed := range available {
		if strings.EqualFold(ed, cfg.Editor) {
			edIdx = i
			break
		}
	}

	return &UI{
		config:        cfg,
		scanner:       scanner.New(cfg),
		availableEds:  available,
		currentEdIdx:  edIdx,
		activeSection: sectionProjects,
		gitCache:      make(map[string]string),
		loadingGit:    make(map[string]bool),
		statusMsg:     "Ready. Use ↑/↓ or j/k to browse, Enter to open.",
		Version:       version,
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
	u.refreshRecentLocked()
	u.refreshFavoritesLocked()
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

func (u *UI) refreshRecentLocked() {
	projectsByPath := make(map[string]model.Project, len(u.projects))
	for _, project := range u.projects {
		projectsByPath[filepath.Clean(project.Path)] = project
	}

	u.recent = u.recent[:0]
	for _, path := range u.config.RecentProjects {
		if project, ok := projectsByPath[filepath.Clean(path)]; ok {
			u.recent = append(u.recent, project)
		}
	}

	if u.selectedRecentIndex >= len(u.recent) {
		u.selectedRecentIndex = len(u.recent) - 1
	}
	if u.selectedRecentIndex < 0 {
		u.selectedRecentIndex = 0
	}
}

func (u *UI) refreshFavoritesLocked() {
	projectsByPath := make(map[string]model.Project, len(u.filtered))
	for _, project := range u.filtered {
		projectsByPath[filepath.Clean(project.Path)] = project
	}

	u.favorites = u.favorites[:0]
	seen := make(map[string]struct{}, len(u.config.FavoriteProjects))
	for _, path := range u.config.FavoriteProjects {
		path = filepath.Clean(path)
		if _, exists := seen[path]; exists {
			continue
		}
		seen[path] = struct{}{}
		if project, ok := projectsByPath[path]; ok {
			u.favorites = append(u.favorites, project)
		}
	}

	if u.selectedFavoriteIndex >= len(u.favorites) {
		u.selectedFavoriteIndex = len(u.favorites) - 1
	}
	if u.selectedFavoriteIndex < 0 {
		u.selectedFavoriteIndex = 0
	}
}

func (u *UI) layout(g *gocui.Gui) error {
	maxX, maxY := g.Size()
	if maxX < 40 || maxY < 14 {
		return nil
	}

	searchHeight := 3
	statusHeight := 4
	mainHeight := maxY - searchHeight - statusHeight
	recentHeight := mainHeight / 3
	if recentHeight < 3 {
		recentHeight = 3
	}

	// Search View
	if v, err := g.SetView(ViewSearch, 0, 0, maxX-1, searchHeight-1); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = " Filter Projects ([ or ] focus, filters as you type, Enter/Esc to finish) "
		v.Editable = true
		v.Editor = gocui.EditorFunc(u.editSearch)
	}

	// Projects List View (left panel)
	listWidth := maxX * 45 / 100
	if listWidth < 30 {
		listWidth = 30
	}
	recentWidth := listWidth / 2
	if v, err := g.SetView(ViewRecent, 0, searchHeight, recentWidth-1, searchHeight+recentHeight-1); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = fmt.Sprintf(" Recently Opened (%d) ", len(u.recent))
	}
	if v, err := g.SetView(ViewFavorites, recentWidth, searchHeight, listWidth, searchHeight+recentHeight-1); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = fmt.Sprintf(" Favorites (%d) ", len(u.favorites))
	}
	if v, err := g.SetView(ViewProjects, 0, searchHeight+recentHeight, listWidth, maxY-statusHeight-1); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = fmt.Sprintf(" Projects (%d) ", len(u.filtered))
	}

	// Details View (right panel)
	if v, err := g.SetView(ViewDetails, listWidth+1, searchHeight, maxX-1, maxY-statusHeight-1); err != nil {
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
	u.renderRecent(g)
	u.renderFavorites(g)
	u.renderDetails(g)
	u.renderStatus(g)

	switch u.activeSection {
	case sectionSearch:
		if _, err := g.SetCurrentView(ViewSearch); err != nil {
			return err
		}
		g.Cursor = true
	case sectionRecent:
		if _, err := g.SetCurrentView(ViewRecent); err != nil {
			return err
		}
		g.Cursor = false
	case sectionFavorites:
		if _, err := g.SetCurrentView(ViewFavorites); err != nil {
			return err
		}
		g.Cursor = false
	default:
		if _, err := g.SetCurrentView(ViewProjects); err != nil {
			return err
		}
		g.Cursor = false
	}

	return nil
}

func (u *UI) renderProjects(g *gocui.Gui) {
	v, err := g.View(ViewProjects)
	if err != nil {
		return
	}
	v.Clear()

	u.mu.Lock()
	defer u.mu.Unlock()

	v.Title = fmt.Sprintf(" Projects (%d) ", len(u.filtered))

	_, height := v.Size()
	start, end := visibleRange(len(u.filtered), u.selectedIndex, height)
	for i := start; i < end; i++ {
		p := u.filtered[i]
		cursor := "  "
		if i == u.selectedIndex {
			cursor = "▶ "
		}
		favoriteMarker := "  "
		if u.isFavoriteLocked(p.Path) {
			favoriteMarker = favoriteStarMarker()
		}
		typeLabel := colorize(fmt.Sprintf("[%-4s]", p.Type), "1", "36")
		line := fmt.Sprintf("%s%s%-20s %s", cursor, favoriteMarker, truncate(p.Name, 20), typeLabel)

		if p.Branch != "" {
			line += colorize(fmt.Sprintf(" (%s)", p.Branch), "33")
		}

		fmt.Fprintln(v, line)
	}
}

func (u *UI) renderFavorites(g *gocui.Gui) {
	v, err := g.View(ViewFavorites)
	if err != nil {
		return
	}
	v.Clear()

	u.mu.Lock()
	defer u.mu.Unlock()
	v.Title = fmt.Sprintf(" Favorites (%d) ", len(u.favorites))

	if len(u.favorites) == 0 {
		fmt.Fprintln(v, "No favorites.")
		return
	}

	_, height := v.Size()
	start, end := visibleRange(len(u.favorites), u.selectedFavoriteIndex, height)
	for i := start; i < end; i++ {
		project := u.favorites[i]
		cursor := "  "
		if i == u.selectedFavoriteIndex {
			cursor = "▶ "
		}
		fmt.Fprintf(v, "%s%s%s\n", cursor, favoriteStarMarker(), truncate(project.Name, 18))
	}
}

func favoriteStarMarker() string {
	return colorize("★ ", "33")
}

func (u *UI) isFavoriteLocked(projectPath string) bool {
	projectPath = filepath.Clean(projectPath)
	for _, path := range u.config.FavoriteProjects {
		if filepath.Clean(path) == projectPath {
			return true
		}
	}
	return false
}

func (u *UI) renderRecent(g *gocui.Gui) {
	v, err := g.View(ViewRecent)
	if err != nil {
		return
	}
	v.Clear()
	v.Title = fmt.Sprintf(" Recently Opened (%d) ", len(u.recent))

	u.mu.Lock()
	defer u.mu.Unlock()

	if len(u.recent) == 0 {
		fmt.Fprintln(v, "No recently opened projects.")
		return
	}

	_, height := v.Size()
	start, end := visibleRange(len(u.recent), u.selectedRecentIndex, height)
	for i := start; i < end; i++ {
		project := u.recent[i]
		cursor := "  "
		if i == u.selectedRecentIndex {
			cursor = "▶ "
		}
		favoriteMarker := "  "
		if u.isFavoriteLocked(project.Path) {
			favoriteMarker = favoriteStarMarker()
		}
		fmt.Fprintf(v, "%s%s%s\n", cursor, favoriteMarker, truncate(project.Name, 28))
	}
}

func (u *UI) renderDetails(g *gocui.Gui) {
	v, err := g.View(ViewDetails)
	if err != nil {
		return
	}
	v.Clear()

	u.mu.Lock()
	p, ok := u.currentProjectLocked()
	if !ok {
		u.mu.Unlock()
		fmt.Fprintln(v, "No projects matching criteria.")
		return
	}

	ed := u.currentEditor()
	cachedGit, hasGitLog := u.gitCache[p.Path]
	isLoading := u.loadingGit[p.Path]
	u.mu.Unlock()

	fmt.Fprintf(v, "Action:   ▶ Press Enter or 'o' to open in [%s]\n", ed)
	fmt.Fprintf(v, "Project:  %s\n", p.Name)
	fmt.Fprintf(v, "Type:     %s\n", colorize(p.Type, "1", "36"))
	fmt.Fprintf(v, "Path:     %s\n", p.Path)
	if p.Branch != "" {
		fmt.Fprintf(v, "Branch:   %s\n", colorize(p.Branch, "33"))
	}
	if !p.LastModified.IsZero() {
		fmt.Fprintf(v, "Modified: %s\n", p.LastModified.Format("2006-01-02 15:04:05"))
	}
	fmt.Fprintln(v, strings.Repeat("─", 50))

	if hasGitLog {
		if cachedGit != "" {
			fmt.Fprintln(v, "Recent Commits:")
			for _, commitLine := range strings.Split(strings.TrimRight(cachedGit, "\n"), "\n") {
				if hash, ok := commitHash(commitLine); ok {
					fmt.Fprintln(v, colorizeCommitLine(commitLine, hash))
					continue
				}
				fmt.Fprintln(v, commitLine)
			}
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

func colorize(text string, codes ...string) string {
	if text == "" || len(codes) == 0 {
		return text
	}
	return fmt.Sprintf("\x1b[%sm%s\x1b[0m", strings.Join(codes, ";"), text)
}

func colorizeCommitLine(line, hash string) string {
	if hash == "" || !strings.HasPrefix(line, hash) {
		return line
	}
	return colorize(hash, "1", "35") + line[len(hash):]
}

func commitHash(line string) (string, bool) {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return "", false
	}

	hash := fields[0]
	if len(hash) < 7 || len(hash) > 40 {
		return "", false
	}

	for _, ch := range hash {
		if !unicode.IsDigit(ch) && (ch < 'a' || ch > 'f') && (ch < 'A' || ch > 'F') {
			return "", false
		}
	}

	return hash, true
}

func (u *UI) currentProjectLocked() (model.Project, bool) {
	if u.activeSection == sectionRecent {
		if u.selectedRecentIndex >= 0 && u.selectedRecentIndex < len(u.recent) {
			return u.recent[u.selectedRecentIndex], true
		}
		return model.Project{}, false
	}
	if u.activeSection == sectionFavorites {
		if u.selectedFavoriteIndex >= 0 && u.selectedFavoriteIndex < len(u.favorites) {
			return u.favorites[u.selectedFavoriteIndex], true
		}
		return model.Project{}, false
	}
	if u.selectedIndex >= 0 && u.selectedIndex < len(u.filtered) {
		return u.filtered[u.selectedIndex], true
	}
	return model.Project{}, false
}

func visibleRange(length, selected, height int) (int, int) {
	if height <= 0 || length == 0 {
		return 0, 0
	}
	if selected < height {
		return 0, min(length, height)
	}
	end := min(length, selected+1)
	return end - height, end
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
	ver := u.Version
	u.mu.Unlock()

	// include favorite key in help keys and render version at lower-right
	keys := fmt.Sprintf("[ or ] Switch section | [j/k or ↑/↓] Move | [Enter/o] Open (%s) | [f] Favorite | [/] Filter | [e] Switch Editor | [r] Rescan | [q] Quit", ed)

	w, h := v.Size()
	lines := []string{fmt.Sprintf("• %s", msg), fmt.Sprintf("• %s", keys)}
	// Reserve last line for version at lower-right
	contentMax := h - 1
	if contentMax < 1 {
		contentMax = 1
	}
	// If there are more content lines than fit, keep the last contentMax
	if len(lines) > contentMax {
		lines = lines[len(lines)-contentMax:]
	}
	filler := contentMax - len(lines)
	for i := 0; i < filler; i++ {
		lines = append(lines, "")
	}

	verText := ver
	if verText == "" {
		verText = "development"
	}
	// Trim if longer than width, keep rightmost chars
	if len(verText) > w {
		verText = verText[len(verText)-w:]
	}
	pad := w - len(verText)
	if pad < 0 {
		pad = 0
	}
	lastLine := fmt.Sprintf("%s%s", strings.Repeat(" ", pad), verText)
	// Print content lines then the final version line, ensuring total printed lines == contentMax + 1
	for i := range lines {
		fmt.Fprintln(v, lines[i])
	}
	// write the final line without adding an extra newline to keep it at the bottom
	fmt.Fprint(v, lastLine)
}

func (u *UI) setKeybindings() error {
	// Global Quit
	if err := u.gui.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, u.quit); err != nil {
		return err
	}

	// Switch focus between filter, recent projects, and project list.
	if err := u.gui.SetKeybinding("", '[', gocui.ModNone, u.previousSection); err != nil {
		return err
	}
	if err := u.gui.SetKeybinding("", ']', gocui.ModNone, u.nextSection); err != nil {
		return err
	}

	// Project list navigation
	navKeys := []interface{}{
		gocui.KeyArrowDown, 'j',
	}
	for _, view := range []string{ViewProjects, ViewRecent, ViewFavorites} {
		for _, k := range navKeys {
			if err := u.gui.SetKeybinding(view, k, gocui.ModNone, u.cursorDown); err != nil {
				return err
			}
		}
	}

	upKeys := []interface{}{
		gocui.KeyArrowUp, 'k',
	}
	for _, view := range []string{ViewProjects, ViewRecent, ViewFavorites} {
		for _, k := range upKeys {
			if err := u.gui.SetKeybinding(view, k, gocui.ModNone, u.cursorUp); err != nil {
				return err
			}
		}
	}

	for _, view := range []string{ViewProjects, ViewRecent, ViewFavorites} {
		if err := u.gui.SetKeybinding(view, 'f', gocui.ModNone, u.toggleFavorite); err != nil {
			return err
		}
	}

	// Open Project
	openKeys := []interface{}{
		gocui.KeyEnter, 'o',
	}
	for _, view := range []string{ViewProjects, ViewRecent, ViewFavorites} {
		for _, k := range openKeys {
			if err := u.gui.SetKeybinding(view, k, gocui.ModNone, u.openProject); err != nil {
				return err
			}
		}
	}

	// Start Search
	for _, view := range []string{ViewProjects, ViewRecent, ViewFavorites} {
		if err := u.gui.SetKeybinding(view, '/', gocui.ModNone, u.startSearch); err != nil {
			return err
		}
	}

	// Rescan
	for _, view := range []string{ViewProjects, ViewRecent, ViewFavorites} {
		if err := u.gui.SetKeybinding(view, 'r', gocui.ModNone, u.rescan); err != nil {
			return err
		}
	}

	// Switch Editor
	for _, view := range []string{ViewProjects, ViewRecent, ViewFavorites} {
		if err := u.gui.SetKeybinding(view, 'e', gocui.ModNone, u.cycleEditor); err != nil {
			return err
		}
	}

	// Quit with q
	for _, view := range []string{ViewProjects, ViewRecent, ViewFavorites} {
		if err := u.gui.SetKeybinding(view, 'q', gocui.ModNone, u.quit); err != nil {
			return err
		}
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
	if u.activeSection == sectionRecent && u.selectedRecentIndex < len(u.recent)-1 {
		u.selectedRecentIndex++
	} else if u.activeSection == sectionFavorites && u.selectedFavoriteIndex < len(u.favorites)-1 {
		u.selectedFavoriteIndex++
	} else if u.activeSection == sectionProjects && u.selectedIndex < len(u.filtered)-1 {
		u.selectedIndex++
	}
	u.mu.Unlock()
	return nil
}

func (u *UI) cursorUp(g *gocui.Gui, v *gocui.View) error {
	u.mu.Lock()
	if u.activeSection == sectionRecent && u.selectedRecentIndex > 0 {
		u.selectedRecentIndex--
	} else if u.activeSection == sectionFavorites && u.selectedFavoriteIndex > 0 {
		u.selectedFavoriteIndex--
	} else if u.activeSection == sectionProjects && u.selectedIndex > 0 {
		u.selectedIndex--
	}
	u.mu.Unlock()
	return nil
}

func (u *UI) toggleFavorite(g *gocui.Gui, v *gocui.View) error {
	u.mu.Lock()
	project, ok := u.currentProjectLocked()
	if !ok {
		u.mu.Unlock()
		return nil
	}
	favorited := u.config.ToggleFavoriteProject(project.Path)
	u.refreshFavoritesLocked()
	if favorited {
		u.statusMsg = fmt.Sprintf("Added %s to favorites", project.Name)
	} else {
		u.statusMsg = fmt.Sprintf("Removed %s from favorites", project.Name)
	}
	if err := u.config.Save(); err != nil {
		u.statusMsg = fmt.Sprintf("Updated favorite status, but failed to save: %v", err)
	}
	u.mu.Unlock()
	return nil
}

func (u *UI) openProject(g *gocui.Gui, v *gocui.View) error {
	u.mu.Lock()
	proj, ok := u.currentProjectLocked()
	if !ok {
		u.mu.Unlock()
		return nil
	}
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
					u.config.RecordRecentProject(path)
					u.refreshRecentLocked()
					if saveErr := u.config.Save(); saveErr != nil {
						u.statusMsg = fmt.Sprintf("Opened %s in %s, but failed to save recent projects: %v", name, editorCmd, saveErr)
					} else {
						u.statusMsg = fmt.Sprintf("Successfully opened %s in %s", name, editorCmd)
					}
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
	u.activeSection = sectionSearch
	u.mu.Unlock()
	return nil
}

func (u *UI) previousSection(g *gocui.Gui, v *gocui.View) error {
	return u.switchSection(-1)
}

func (u *UI) nextSection(g *gocui.Gui, v *gocui.View) error {
	return u.switchSection(1)
}

func (u *UI) switchSection(direction section) error {
	u.mu.Lock()
	u.activeSection = (u.activeSection + direction + sectionCount) % sectionCount
	u.mu.Unlock()
	return nil
}

func (u *UI) stopSearch(g *gocui.Gui, v *gocui.View) error {
	u.mu.Lock()
	u.activeSection = sectionProjects
	u.searchQuery = ""
	u.applyFilterLocked()
	u.refreshFavoritesLocked()
	u.statusMsg = "Search cleared. Browsing all projects."
	u.mu.Unlock()

	sv, err := g.View(ViewSearch)
	if err == nil {
		sv.Clear()
	}
	return nil
}

func (u *UI) confirmSearch(g *gocui.Gui, v *gocui.View) error {
	u.updateSearchQuery(v.Buffer())
	u.mu.Lock()
	u.activeSection = sectionProjects
	u.statusMsg = fmt.Sprintf("Filter: %q (%d matches)", u.searchQuery, len(u.filtered))
	u.mu.Unlock()
	return nil
}

func (u *UI) editSearch(v *gocui.View, key gocui.Key, ch rune, mod gocui.Modifier) {
	gocui.DefaultEditor.Edit(v, key, ch, mod)
	u.updateSearchQuery(v.Buffer())
}

func (u *UI) updateSearchQuery(query string) {
	u.mu.Lock()
	defer u.mu.Unlock()

	u.searchQuery = strings.TrimSpace(query)
	u.applyFilterLocked()
	u.refreshFavoritesLocked()
	u.statusMsg = fmt.Sprintf("Filter: %q (%d matches)", u.searchQuery, len(u.filtered))
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

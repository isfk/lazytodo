// Package integration drives the real GUI against a fake screen. A test says
// "press space here, expect this list" instead of poking at internal state --
// the same bargain lazygit's integration suite makes, scaled down to one file.
package integration

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/shen/lazytodo/pkg/commands"
	"github.com/shen/lazytodo/pkg/config"
	"github.com/shen/lazytodo/pkg/gui"
	"github.com/shen/lazytodo/pkg/i18n"
	"github.com/shen/lazytodo/pkg/tui"
)

const (
	// Default fake terminal size. Wide enough for the three-pane layout, so the
	// tests exercise the real arrangement; the narrow-terminal tests shrink it
	// explicitly.
	defaultWidth  = 80
	defaultHeight = 16
)

// TestCase is one integration scenario.
type TestCase struct {
	// Setup seeds the todo file before the GUI starts. Optional.
	Setup func(*Shell)
	// Run drives the GUI and asserts on what ends up on screen.
	Run func(*TestDriver)
	// Size overrides the fake terminal size, as [width, height]. Optional.
	Size [2]int
	// Language overrides the interface language. Tests default to English so
	// they don't depend on the developer's locale.
	Language string
}

// RunTest builds a throwaway store, starts the GUI and runs the case.
func RunTest(t *testing.T, tc TestCase) {
	t.Helper()

	store := commands.NewStore(filepath.Join(t.TempDir(), "todos.json"))
	if err := store.Load(); err != nil {
		t.Fatalf("loading empty store: %v", err)
	}

	if tc.Setup != nil {
		tc.Setup(&Shell{t: t, store: store})
		if err := store.Save(); err != nil {
			t.Fatalf("saving seeded store: %v", err)
		}
	}

	width, height := defaultWidth, defaultHeight
	if tc.Size[0] > 0 {
		width, height = tc.Size[0], tc.Size[1]
	}

	screen := tui.NewFakeScreen(width, height)
	cfg := config.Default()
	// Tests assert English strings, so pin the language instead of inheriting
	// whatever locale the developer's shell happens to export.
	cfg.Language = i18n.English
	if tc.Language != "" {
		cfg.Language = tc.Language
	}
	g := gui.New(screen, store, cfg)

	g.Layout()
	g.Refresh()
	if err := g.Render(); err != nil {
		t.Fatalf("initial render: %v", err)
	}

	driver := &TestDriver{t: t, screen: screen, gui: g, store: store, Keys: &cfg.Keybinding}
	if tc.Run != nil {
		tc.Run(driver)
	}
}

// Shell seeds the store before the GUI starts.
type Shell struct {
	t     *testing.T
	store *commands.Store
}

// Add appends todos in order, one per title.
func (s *Shell) Add(titles ...string) {
	for _, title := range titles {
		s.store.Add(title, "")
	}
}

// AddWithDetail appends one todo that already has a description.
func (s *Shell) AddWithDetail(title, detail string) {
	s.store.Add(title, detail)
}

// Cycle advances the todos at the given 1-based positions one status step
// (todo -> doing -> done -> todo).
func (s *Shell) Cycle(positions ...int) {
	s.t.Helper()

	if err := s.store.CycleStatus(s.idsAt(positions)...); err != nil {
		s.t.Fatalf("cycling %v: %v", positions, err)
	}
}

func (s *Shell) idsAt(positions []int) []int {
	s.t.Helper()

	ids := make([]int, 0, len(positions))
	for _, position := range positions {
		if position < 1 || position > len(s.store.Todos) {
			s.t.Fatalf("position %d is out of range: only %d todo(s) exist", position, len(s.store.Todos))
		}
		ids = append(ids, s.store.Todos[position-1].ID)
	}
	return ids
}

// TestDriver is the test's handle on a running GUI.
type TestDriver struct {
	t      *testing.T
	screen *tui.FakeScreen
	gui    *gui.Gui
	store  *commands.Store

	// Keys is the live configuration, so a test presses the same keys the user
	// would without hardcoding them. Changing a default key can't break a test.
	Keys *config.Keybindings
}

// Press sends keys through the real dispatch path and then redraws.
func (d *TestDriver) Press(keys ...tui.Key) *TestDriver {
	d.t.Helper()

	for _, key := range keys {
		if err := d.gui.HandleKey(key); err != nil {
			d.t.Fatalf("handling %q: %v", key.Label(), err)
		}
	}

	d.redraw()
	return d
}

// Click sends a left click through the real dispatch path and then redraws.
func (d *TestDriver) Click(x, y int) *TestDriver {
	d.t.Helper()

	if err := d.gui.HandleClick(x, y); err != nil {
		d.t.Fatalf("click at (%d,%d): %v", x, y, err)
	}

	d.redraw()
	return d
}

// Type presses one key per character, for filling in a prompt.
func (d *TestDriver) Type(text string) *TestDriver {
	d.t.Helper()

	for _, r := range text {
		if err := d.gui.HandleKey(tui.KeyOfRune(r)); err != nil {
			d.t.Fatalf("typing %q: %v", r, err)
		}
	}

	d.redraw()
	return d
}

func (d *TestDriver) redraw() {
	d.t.Helper()

	if err := d.gui.Render(); err != nil {
		d.t.Fatalf("render: %v", err)
	}
}

// Todos is the list pane.
func (d *TestDriver) Todos() *ViewDriver { return d.view("todos", d.gui.Views().Todos) }

// Status is the status bar.
func (d *TestDriver) Status() *ViewDriver { return d.view("status", d.gui.Views().Status) }

// Options is the keybinding hint bar.
func (d *TestDriver) Options() *ViewDriver { return d.view("options", d.gui.Views().Options) }

// Detail is the description pane.
func (d *TestDriver) Detail() *ViewDriver { return d.view("detail", d.gui.Views().Detail) }

// Popup is the modal editor.
func (d *TestDriver) Popup() *ViewDriver { return d.view("popup", d.gui.Views().Popup) }

func (d *TestDriver) view(name string, view *tui.View) *ViewDriver {
	return &ViewDriver{driver: d, name: name, view: view}
}

// Store is the command layer, for asserting on persisted state.
func (d *TestDriver) Store() *commands.Store { return d.store }

// Clipboard is what the last copy put on the system clipboard.
func (d *TestDriver) Clipboard() string { return d.screen.Clipboard() }

// ViewDriver reads one pane back off the screen.
type ViewDriver struct {
	driver *TestDriver
	name   string
	view   *tui.View
}

// Content is what is painted inside the pane right now, with trailing blank
// rows trimmed so a list of three todos reads as three lines.
func (v *ViewDriver) Content() []string {
	x0, y0, x1, y1 := v.view.ContentRegion()
	return trimTrailingBlank(v.driver.screen.Region(x0, y0, x1, y1))
}

// Lines asserts the pane content line by line. The count must match exactly, so
// a test can't silently pass while extra rows appear.
func (v *ViewDriver) Lines(matchers ...*LineMatcher) *ViewDriver {
	v.driver.t.Helper()

	content := v.Content()
	if len(content) != len(matchers) {
		v.driver.t.Fatalf("%s: expected %d line(s), got %d:\n%s",
			v.name, len(matchers), len(content), quoteLines(content))
	}

	for i, matcher := range matchers {
		if !matcher.fn(content[i]) {
			v.driver.t.Errorf("%s line %d: expected %s, got %q",
				v.name, i+1, matcher.desc, content[i])
		}
		if matcher.selected && i != v.view.SelectedLine {
			v.driver.t.Errorf("%s line %d: expected the cursor here, it is on line %d",
				v.name, i+1, v.view.SelectedLine+1)
		}
	}

	return v
}

// Content is the current selection row, 0-based.
func (v *ViewDriver) Cursor() int { return v.view.SelectedLine }

// Includes asserts that each substring appears somewhere in the pane. Handy for
// popups, where the exact line count is an implementation detail.
func (v *ViewDriver) Includes(subs ...string) *ViewDriver {
	v.driver.t.Helper()

	content := strings.Join(v.Content(), "\n")
	for _, sub := range subs {
		if !strings.Contains(content, sub) {
			v.driver.t.Errorf("%s: expected to contain %q, got:\n%s", v.name, sub, content)
		}
	}

	return v
}

// Footer is the text drawn in the bottom border: popup hints, or the reason the
// last submit was rejected.
func (v *ViewDriver) Footer() string { return v.view.Footer }

// Title is the text drawn in the top border.
func (v *ViewDriver) Title() string { return v.view.Title }

// TopBorder is the rendered top border row, where the title and the filter tabs
// live.
func (v *ViewDriver) TopBorder() string {
	lines := v.driver.screen.Region(v.view.X0, v.view.Y0, v.view.X1, v.view.Y0)
	if len(lines) == 0 {
		return ""
	}
	return lines[0]
}

// TabCenter is the screen coordinate of a tab's middle, for clicking it.
func (v *ViewDriver) TabCenter(label string) (int, int) {
	v.driver.t.Helper()

	for _, span := range v.view.TabSpans() {
		if v.view.Tabs[span.Index].Label == label {
			return (span.X0 + span.X1) / 2, v.view.Y0
		}
	}

	v.driver.t.Fatalf("%s: no tab labelled %q", v.name, label)
	return 0, 0
}

// Region is the pane's inner area in screen coordinates.
func (v *ViewDriver) Region() (x0, y0, x1, y1 int) { return v.view.ContentRegion() }

// ContentPoint is a screen coordinate inside the inner area for the given
// content row (0-based), for clicking a row.
func (v *ViewDriver) ContentPoint(row int) (int, int) {
	x0, y0, _, _ := v.view.ContentRegion()
	return x0 + 1, y0 + row
}

// LineMatcher describes an expected line. Build one with Equals, Contains or
// Matches, then optionally chain IsSelected.
type LineMatcher struct {
	desc     string
	fn       func(string) bool
	selected bool
}

// Equals matches a line exactly.
func Equals(want string) *LineMatcher {
	return &LineMatcher{
		desc: fmt.Sprintf("exactly %q", want),
		fn:   func(got string) bool { return got == want },
	}
}

// Contains matches a line that includes sub.
func Contains(sub string) *LineMatcher {
	return &LineMatcher{
		desc: fmt.Sprintf("to contain %q", sub),
		fn:   func(got string) bool { return strings.Contains(got, sub) },
	}
}

// Matches matches a line against a regular expression.
func Matches(pattern string) *LineMatcher {
	re := regexp.MustCompile(pattern)
	return &LineMatcher{
		desc: "to match /" + pattern + "/",
		fn:   re.MatchString,
	}
}

// IsSelected additionally requires the cursor to be on this line.
func (m *LineMatcher) IsSelected() *LineMatcher {
	m.selected = true
	return m
}

func trimTrailingBlank(lines []string) []string {
	end := len(lines)
	for end > 0 && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	return lines[:end]
}

func quoteLines(lines []string) string {
	var b strings.Builder
	for _, line := range lines {
		b.WriteString("  ")
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}

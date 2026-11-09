package tui

import (
	"strings"
	"testing"
)

func newTestView(rows int) *View {
	return &View{
		Visible:    true,
		Selectable: true,
		X0:         0,
		Y0:         0,
		X1:         9,
		Y1:         rows - 1,
	}
}

func TestSelectionScrollsIntoView(t *testing.T) {
	view := newTestView(5)
	view.SetContent([]string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9"})

	if got := view.InnerHeight(); got != 5 {
		t.Fatalf("InnerHeight = %d, want 5", got)
	}

	view.MoveSelection(3)
	if view.OriginY != 0 {
		t.Errorf("OriginY = %d, want 0 (selection still on screen)", view.OriginY)
	}

	view.MoveSelection(3) // now on line 6, one past the window
	if view.SelectedLine != 6 || view.OriginY != 2 {
		t.Errorf("selection = %d, origin = %d; want 6 and 2", view.SelectedLine, view.OriginY)
	}

	view.SelectFirst()
	if view.SelectedLine != 0 || view.OriginY != 0 {
		t.Errorf("after SelectFirst: selection = %d, origin = %d", view.SelectedLine, view.OriginY)
	}

	view.SelectLast()
	if view.SelectedLine != 9 || view.OriginY != 5 {
		t.Errorf("after SelectLast: selection = %d, origin = %d; want 9 and 5", view.SelectedLine, view.OriginY)
	}
}

func TestSelectionClampsToContent(t *testing.T) {
	view := newTestView(3)
	view.SetContent([]string{"only"})

	view.MoveSelection(10)
	if view.SelectedLine != 0 {
		t.Errorf("SelectedLine = %d, want 0", view.SelectedLine)
	}

	view.MoveSelection(-10)
	if view.SelectedLine != 0 {
		t.Errorf("SelectedLine = %d, want 0", view.SelectedLine)
	}

	view.SetContent(nil)
	if view.SelectedLine != 0 || view.SelectedText() != "" {
		t.Errorf("empty content left selection at %d (%q)", view.SelectedLine, view.SelectedText())
	}
}

func TestRenderDrawsFrameTitleAndClippedContent(t *testing.T) {
	screen := NewFakeScreen(12, 5)
	view := &View{
		Visible: true,
		Frame:   true,
		Title:   "T",
		X0:      0, Y0: 0, X1: 11, Y1: 4,
	}
	view.SetContent([]string{"hello world", "x"})

	view.Render(screen)

	want := []string{
		"╭─T────────╮",
		"│hello worl│",
		"│x         │",
		"│          │",
		"╰──────────╯",
	}

	got := screen.Text()
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("row %d = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestFrameIsAlwaysTerminalDefaultColour pins the rule that the border is never
// coloured: it is decoration, and a hard-coded colour fights the user's palette.
func TestFrameIsAlwaysTerminalDefaultColour(t *testing.T) {
	for _, focused := range []bool{true, false} {
		screen := NewFakeScreen(10, 5)
		view := &View{
			Visible: true,
			Frame:   true,
			Title:   "T",
			X0:      0, Y0: 0, X1: 9, Y1: 4,
			Focused: focused,
		}
		view.SetContent([]string{"x"})

		view.Render(screen)

		for _, cell := range [][2]int{{0, 0}, {9, 0}, {0, 4}, {0, 1}} {
			if got := screen.CellAt(cell[0], cell[1]).Style.Fg; got != ColorDefault {
				t.Errorf("focused=%v: border cell %v = %v, want the terminal default", focused, cell, got)
			}
		}
	}
}

func TestRenderPlacesWideCharactersOnTwoCells(t *testing.T) {
	screen := NewFakeScreen(10, 3)
	view := &View{Visible: true, X0: 0, Y0: 0, X1: 9, Y1: 2}
	view.SetContent([]string{"中文ab"})

	view.Render(screen)

	if got := screen.CellAt(0, 0).Ch; got != '中' {
		t.Errorf("cell 0 = %q, want 中", got)
	}
	// 文 has to start at cell 2: writing it at cell 1 would cut 中 in half.
	if got := screen.CellAt(2, 0).Ch; got != '文' {
		t.Errorf("cell 2 = %q, want 文 (the wide char was overwritten)", got)
	}
	if got := screen.CellAt(4, 0).Ch; got != 'a' {
		t.Errorf("cell 4 = %q, want a", got)
	}
	if got := screen.Text()[0]; got != "中文ab" {
		t.Errorf("text = %q, want 中文ab", got)
	}
}

func TestRenderRefusesToSplitAWideCharacterAtTheEdge(t *testing.T) {
	screen := NewFakeScreen(3, 2)
	view := &View{Visible: true, X0: 0, Y0: 0, X1: 2, Y1: 1}
	view.SetContent([]string{"中中"})

	view.Render(screen)

	if got := screen.Text()[0]; got != "中" {
		t.Errorf("text = %q, want a single 中: only one fits in three cells", got)
	}
}

func TestRenderSkipsInvisibleViews(t *testing.T) {
	screen := NewFakeScreen(5, 2)
	view := &View{Visible: false, X0: 0, Y0: 0, X1: 4, Y1: 1}
	view.SetContent([]string{"nope"})

	view.Render(screen)

	if got := screen.Text()[0]; got != "" {
		t.Errorf("invisible view painted %q", got)
	}
}

func TestRuneOfKey(t *testing.T) {
	if r, ok := RuneOfKey(KeyOfRune('j')); !ok || r != 'j' {
		t.Errorf("RuneOfKey(j) = %q, %v", r, ok)
	}
	if _, ok := RuneOfKey(KeyEnter); ok {
		t.Error("RuneOfKey(enter) should not be printable")
	}
}

// TestSpaceBarRoundTrip guards the seam that broke once already: a terminal
// reports the space bar as an ordinary rune, so KeyOfRune has to normalise it
// onto KeySpace (what the configuration binds), and RuneOfKey has to give the
// space character back (what a text field needs).
func TestSpaceBarRoundTrip(t *testing.T) {
	if got := KeyOfRune(' '); got != KeySpace {
		t.Errorf("KeyOfRune(' ') = %q, want %q", got, KeySpace)
	}

	r, ok := RuneOfKey(KeySpace)
	if !ok || r != ' ' {
		t.Errorf("RuneOfKey(KeySpace) = %q, %v; want ' ', true", r, ok)
	}
}

func TestKeyOfRunePassesThroughOtherRunes(t *testing.T) {
	for _, r := range []rune{'a', 'A', '/', '✓', '存'} {
		if got := KeyOfRune(r); got != Key(r) {
			t.Errorf("KeyOfRune(%q) = %q, want %q", r, got, Key(r))
		}
	}
}

func TestKeyLabel(t *testing.T) {
	if got := Key(KeySpace).Label(); got != "␣" {
		t.Errorf("space label = %q", got)
	}
	if got := Key("a").Label(); got != "a" {
		t.Errorf("letter label = %q", got)
	}
}

// ---------------------------------------------------------------------------
// tab strip
// ---------------------------------------------------------------------------

func newTabbedView() *View {
	view := &View{
		Visible: true,
		Frame:   true,
		Title:   "Todos",
		X0:      0, Y0: 0, X1: 39, Y1: 5,
		SelFg: ColorBlack,
		SelBg: ColorCyan,
	}
	view.Tabs = []Tab{{Label: "All"}, {Label: "Doing", Active: true}, {Label: "Done"}}
	return view
}

func TestTabSpansStartAfterTheTitle(t *testing.T) {
	view := newTabbedView()

	spans := view.TabSpans()
	if len(spans) != 3 {
		t.Fatalf("got %d spans, want 3", len(spans))
	}

	// "Todos" is five cells starting at X0+2, plus one border cell, so the strip
	// begins at column 8. Tabs are separated by " - ", three cells each.
	want := []TabSpan{
		{X0: 8, X1: 10, Index: 0},
		{X0: 14, X1: 18, Index: 1},
		{X0: 22, X1: 25, Index: 2},
	}
	for i := range want {
		if spans[i] != want[i] {
			t.Errorf("span %d = %+v, want %+v", i, spans[i], want[i])
		}
	}
}

func TestTabAtIgnoresGapsAndOtherRows(t *testing.T) {
	view := newTabbedView()

	if idx, ok := view.TabAt(15, 0); !ok || idx != 1 {
		t.Errorf("TabAt(15,0) = %d, %v; want 1, true", idx, ok)
	}
	if _, ok := view.TabAt(12, 0); ok {
		t.Error("the gap between tabs should not be a hit")
	}
	if _, ok := view.TabAt(15, 1); ok {
		t.Error("tabs only live on the border row")
	}
}

func TestTabsAreDroppedWhenTheyDoNotFit(t *testing.T) {
	view := newTabbedView()
	view.X1 = 18 // only "All" fits before the border ends

	if spans := view.TabSpans(); len(spans) != 1 {
		t.Fatalf("got %d spans, want only the first tab", len(spans))
	}
}

// TestActiveTabIsHighlightedWithoutABackground pins the styling rule: the
// applied filter is lit up with text only, so it never paints a block that
// fights the terminal's palette.
func TestActiveTabIsHighlightedWithoutABackground(t *testing.T) {
	screen := NewFakeScreen(30, 3)
	view := &View{
		Visible: true,
		Frame:   true,
		Title:   "T",
		X0:      0, Y0: 0, X1: 29, Y1: 2,
		TitleFg: ColorGreen,
		SelFg:   ColorBlack,
		SelBg:   ColorCyan,
	}
	view.Tabs = []Tab{{Label: "All", Active: true}, {Label: "Done"}}

	view.Render(screen)

	row := screen.Text()[0]
	for _, want := range []string{"All", "Done"} {
		if !strings.Contains(row, want) {
			t.Errorf("border row %q is missing %q", row, want)
		}
	}

	// " T " ends at column 4, plus the gap, so the first tab starts at 7.
	active := screen.CellAt(4, 0).Style
	if active.Bg != ColorDefault {
		t.Errorf("active tab background = %v, want none", active.Bg)
	}
	if active.Fg != ColorGreen || active.Attr&AttrBold == 0 {
		t.Errorf("active tab = %+v, want the highlight colour and bold", active)
	}

	inactive := screen.CellAt(10, 0).Style
	if inactive.Bg != ColorDefault {
		t.Errorf("inactive tab background = %v, want none", inactive.Bg)
	}
	if inactive.Attr&AttrDim == 0 {
		t.Error("the inactive tab should be dimmed, not lit up")
	}
}

// TestTitleIsLitOnlyWhileFocused covers the other half of the rule: an inactive
// pane's title goes back to plain border text.
func TestTitleIsLitOnlyWhileFocused(t *testing.T) {
	render := func(focused bool) Style {
		screen := NewFakeScreen(20, 5)
		view := &View{
			Visible: true,
			Frame:   true,
			Title:   "Todos",
			X0:      0, Y0: 0, X1: 19, Y1: 4,
			TitleFg: ColorGreen,
			Focused: focused,
		}
		view.SetContent([]string{"x"})
		view.Render(screen)
		return screen.CellAt(3, 0).Style // inside the title text
	}

	if got := render(true); got.Fg != ColorGreen || got.Attr&AttrBold == 0 {
		t.Errorf("focused title = %+v, want it lit up", got)
	}
	if got := render(false); got.Fg != ColorDefault || got.Attr&AttrBold != 0 {
		t.Errorf("unfocused title = %+v, want plain border text", got)
	}
}

// TestSelectionIsHighlightedOnlyWhileFocused keeps an inactive pane from looking
// like it has the cursor.
func TestSelectionIsHighlightedOnlyWhileFocused(t *testing.T) {
	render := func(focused bool) Style {
		screen := NewFakeScreen(20, 5)
		view := &View{
			Visible:    true,
			Selectable: true,
			X0:         0, Y0: 0, X1: 19, Y1: 4,
			SelFg:   ColorBlack,
			SelBg:   ColorCyan,
			Focused: focused,
		}
		view.SetContent([]string{"row one"})
		view.Render(screen)
		return screen.CellAt(0, 0).Style
	}

	if got := render(true); got.Bg != ColorCyan {
		t.Errorf("focused selection = %+v, want the selection colour", got)
	}
	if got := render(false); got.Bg == ColorCyan {
		t.Error("an inactive pane should not show a highlighted row")
	}
}

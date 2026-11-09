package tui

import "github.com/clipperhouse/displaywidth"

// View is a rectangular, framed, scrollable buffer of text lines. It owns the
// rendering rules for panes (borders, titles, selection highlight, scrolling) so
// that pkg/gui only has to hand it lines of text.
//
// Coordinates are inclusive on both ends: X0,Y0 is the top-left cell and X1,Y1
// the bottom-right one.
type View struct {
	Name  string
	Title string

	X0, Y0, X1, Y1 int

	Frame   bool
	Visible bool

	// Lines is the content, one string per row.
	Lines []string

	// OriginY is the index of the line shown at the top of the inner area.
	OriginY int
	// SelectedLine is the index into Lines of the highlighted row. It is kept
	// inside the scroll window by clamp().
	SelectedLine int

	Fg, Bg       Color
	SelFg, SelBg Color
	TitleFg      Color

	// Focused picks the bright selection colour for the cursor row.
	Focused bool

	// Selectable says whether a "selection" means anything for this view. Plain
	// output panes (status bar, keybinding hints) leave it false so their only
	// line isn't painted as if the cursor were on it.
	Selectable bool

	// LineStyles optionally overrides the base style per row, so presentation can
	// say "this row is done, dim it" without the view knowing what a todo is. The
	// selection style still wins over these.
	LineStyles []Style

	// Footer is drawn in the bottom border, symmetric with Title. It is where a
	// modal says "Enter submits, Esc cancels" without stealing a content row.
	Footer   string
	FooterFg Color

	// Tabs are drawn along the top border after the title, like a row of tabs on
	// a filing cabinet. The view only draws them and reports where they landed
	// (TabAt); deciding what a click means is the caller's job.
	Tabs []Tab
}

// Tab is one clickable label in a view's top border.
type Tab struct {
	Label  string
	Active bool
}

// TabSpan is where one tab sits on the top border, in screen coordinates
// (inclusive on both ends).
type TabSpan struct {
	X0, X1 int
	Index  int
}

const (
	// tabSeparator sits between two tabs: a short dash between spaces, the way
	// lazygit draws its panel tabs. The border rune is painted over with it.
	tabSeparator = " - "
	// tabStartGap is the border cell between the title and the first tab.
	tabStartGap = 1
)

// DisplayWidth is how many terminal cells a string occupies: CJK characters and
// emoji take two, everything else one. Column maths has to use this instead of
// len() or a rune count, or a list of Chinese titles stops lining up and
// truncation overflows the pane.
func DisplayWidth(s string) int { return displaywidth.String(s) }

// RuneWidth is the cell width of one character.
func RuneWidth(r rune) int { return displaywidth.Rune(r) }

// SetBounds moves and resizes the view. The GUI calls this from layout.
func (v *View) SetBounds(x0, y0, x1, y1 int) {
	v.X0, v.Y0, v.X1, v.Y1 = x0, y0, x1, y1
}

// Inner returns the content area, i.e. the bounds minus the frame.
func (v *View) inner() (x0, y0, x1, y1 int) {
	if v.Frame {
		return v.X0 + 1, v.Y0 + 1, v.X1 - 1, v.Y1 - 1
	}
	return v.X0, v.Y0, v.X1, v.Y1
}

// InnerWidth is how many columns of content fit.
func (v *View) InnerWidth() int {
	x0, _, x1, _ := v.inner()
	return x1 - x0 + 1
}

// InnerHeight is how many rows of content fit.
func (v *View) InnerHeight() int {
	_, y0, _, y1 := v.inner()
	return y1 - y0 + 1
}

// ContentRegion returns the inner rectangle in screen coordinates. Tests read
// this back as "what the user actually sees in this pane".
func (v *View) ContentRegion() (x0, y0, x1, y1 int) { return v.inner() }

// Contains reports whether a cell is inside the view's bounds.
func (v *View) Contains(x, y int) bool {
	return x >= v.X0 && x <= v.X1 && y >= v.Y0 && y <= v.Y1
}

// RowAt maps a screen row to a content line index, if that row is inside the
// scroll window and actually has a line under it.
func (v *View) RowAt(y int) (int, bool) {
	_, iy0, _, iy1 := v.inner()
	if y < iy0 || y > iy1 {
		return 0, false
	}

	idx := v.OriginY + (y - iy0)
	if idx < 0 || idx >= len(v.Lines) {
		return 0, false
	}
	return idx, true
}

// titleLabel is the text painted in the top border. It sits flush against the
// border runes on either side, the way lazygit draws its panel titles -- no
// padding, so the line runs straight into the title.
func (v *View) titleLabel() string { return v.Title }

// TabSpans returns where each tab sits. It is derived from the title and the
// labels alone -- never from render state -- so hit-testing and drawing cannot
// disagree. Tabs that would overflow the border are dropped.
func (v *View) TabSpans() []TabSpan {
	if !v.Frame || len(v.Tabs) == 0 {
		return nil
	}

	x := v.X0 + 2
	if label := v.titleLabel(); label != "" {
		x += DisplayWidth(label) + tabStartGap
	}

	separator := DisplayWidth(tabSeparator)

	spans := make([]TabSpan, 0, len(v.Tabs))
	for i, tab := range v.Tabs {
		if i > 0 {
			x += separator
		}

		width := DisplayWidth(tab.Label)
		if width == 0 {
			continue
		}
		if x+width-1 >= v.X1 {
			break
		}

		spans = append(spans, TabSpan{X0: x, X1: x + width - 1, Index: i})
		x += width
	}

	return spans
}

// TabAt reports which tab covers a cell, if any.
func (v *View) TabAt(x, y int) (int, bool) {
	if !v.Visible || !v.Frame || y != v.Y0 {
		return 0, false
	}

	for _, span := range v.TabSpans() {
		if x >= span.X0 && x <= span.X1 {
			return span.Index, true
		}
	}

	return 0, false
}

// SetContent replaces the content and keeps the selection in range.
func (v *View) SetContent(lines []string) {
	v.Lines = lines
	v.clamp()
}

// SetLineStyles replaces the per-line style overrides. Pass nil to clear them.
func (v *View) SetLineStyles(styles []Style) {
	v.LineStyles = styles
}

// SelectedText returns the currently highlighted line, or "" when empty.
func (v *View) SelectedText() string {
	if v.SelectedLine < 0 || v.SelectedLine >= len(v.Lines) {
		return ""
	}
	return v.Lines[v.SelectedLine]
}

// MoveSelection moves the cursor by delta rows, clamped to the content.
func (v *View) MoveSelection(delta int) {
	if len(v.Lines) == 0 {
		return
	}
	v.SelectedLine += delta
	v.clamp()
}

// SetSelected puts the cursor on idx, clamped to the content.
func (v *View) SetSelected(idx int) {
	v.SelectedLine = idx
	v.clamp()
}

// SelectFirst / SelectLast are what "gg" and "G" bind to.
func (v *View) SelectFirst() { v.SetSelected(0) }
func (v *View) SelectLast()  { v.SetSelected(len(v.Lines) - 1) }

func (v *View) clamp() {
	n := len(v.Lines)
	if n == 0 {
		v.SelectedLine, v.OriginY = 0, 0
		return
	}
	if v.SelectedLine < 0 {
		v.SelectedLine = 0
	}
	if v.SelectedLine > n-1 {
		v.SelectedLine = n - 1
	}
	v.ensureSelectionVisible()
}

// ensureSelectionVisible keeps the cursor on screen.
func (v *View) ensureSelectionVisible() {
	if len(v.Lines) == 0 {
		v.OriginY = 0
		return
	}
	v.ScrollToLine(v.SelectedLine)
}

// ScrollToLine scrolls the window the minimum amount needed to make idx visible.
// It serves both the cursor and, in popups, a caret that isn't a selection.
func (v *View) ScrollToLine(idx int) {
	height := v.InnerHeight()
	if height <= 0 || idx < 0 {
		return
	}

	if idx < v.OriginY {
		v.OriginY = idx
	}
	if idx >= v.OriginY+height {
		v.OriginY = idx - height + 1
	}
	v.clampOrigin()
}

// clampOrigin keeps the scroll window inside the content.
func (v *View) clampOrigin() {
	maxOrigin := len(v.Lines) - v.InnerHeight()
	if maxOrigin < 0 {
		maxOrigin = 0
	}
	if v.OriginY > maxOrigin {
		v.OriginY = maxOrigin
	}
	if v.OriginY < 0 {
		v.OriginY = 0
	}
}

// lineStyle is the base style for one row: the per-line override when there is
// one, otherwise the view's own colours.
func (v *View) lineStyle(idx int) Style {
	if idx >= 0 && idx < len(v.LineStyles) {
		return v.LineStyles[idx]
	}
	return Style{Fg: v.Fg, Bg: v.Bg}
}

// Render paints the view onto the screen. Content is clipped, not wrapped: a
// pane is a window, and the caller is responsible for keeping lines short
// (pkg/gui/presentation does that).
func (v *View) Render(s Screen) {
	if !v.Visible {
		return
	}

	ix0, iy0, ix1, iy1 := v.inner()
	width, height := ix1-ix0+1, iy1-iy0+1
	if width <= 0 || height <= 0 {
		return
	}

	if v.Frame {
		v.renderFrame(s)
	}

	selected := Style{Fg: v.SelFg, Bg: v.SelBg}

	for row := 0; row < height; row++ {
		idx := v.OriginY + row
		if idx >= len(v.Lines) {
			break
		}

		style := v.lineStyle(idx)
		if v.Focused && v.Selectable && idx == v.SelectedLine {
			style = selected
		}

		// Paint by display width, not by rune index: a Chinese character covers
		// two cells, so the next one starts two columns later. The continuation
		// cell is deliberately left untouched -- the screen cleared it, and
		// writing into it would cut the wide character in half.
		col := 0
		for _, r := range v.Lines[idx] {
			w := max(RuneWidth(r), 1)
			if col+w > width {
				break
			}
			s.SetCell(ix0+col, iy0+row, r, style)
			col += w
		}
		for ; col < width; col++ {
			s.SetCell(ix0+col, iy0+row, ' ', style)
		}
	}
}

func (v *View) renderFrame(s Screen) {
	// The frame is always drawn in the terminal's own foreground: colouring it
	// fights with whatever palette the user has, and the border is decoration.
	style := Style{Fg: ColorDefault, Bg: v.Bg}
	width, height := v.X1-v.X0+1, v.Y1-v.Y0+1
	if width < 2 || height < 2 {
		return
	}

	for x := v.X0 + 1; x < v.X1; x++ {
		s.SetCell(x, v.Y0, '─', style)
		s.SetCell(x, v.Y1, '─', style)
	}
	for y := v.Y0 + 1; y < v.Y1; y++ {
		s.SetCell(v.X0, y, '│', style)
		s.SetCell(v.X1, y, '│', style)
	}

	// Rounded corners read softer than square ones at small sizes.
	s.SetCell(v.X0, v.Y0, '╭', style)
	s.SetCell(v.X1, v.Y0, '╮', style)
	s.SetCell(v.X0, v.Y1, '╰', style)
	s.SetCell(v.X1, v.Y1, '╯', style)

	if v.Title != "" {
		// Only the active pane's title is lit up; the inactive one would
		// otherwise compete for attention with the cursor.
		titleStyle := Style{Fg: ColorDefault, Bg: v.Bg}
		if v.Focused {
			titleStyle = Style{Fg: v.TitleFg, Bg: v.Bg, Attr: AttrBold}
		}
		v.paintBorderRunes(s, v.X0+2, v.Y0, v.Title, titleStyle)
	}
	v.renderTabs(s)

	if v.Footer != "" {
		v.renderBorderText(s, v.Footer, v.Y1, Style{Fg: v.FooterFg, Bg: v.Bg})
	}
}

// renderTabs paints the tab strip. The active tab is lit up with text only -- no
// background, so it doesn't fight the terminal's palette -- and the rest are
// dimmed border text.
func (v *View) renderTabs(s Screen) {
	active := Style{Fg: v.TitleFg, Bg: v.Bg, Attr: AttrBold}
	inactive := Style{Fg: ColorDefault, Bg: v.Bg, Attr: AttrDim}

	for _, span := range v.TabSpans() {
		tab := v.Tabs[span.Index]

		// The frame already drew a border rune here; replace it with the dash
		// that separates the tabs.
		if span.Index > 0 {
			v.paintBorderRunes(s, span.X0-DisplayWidth(tabSeparator), v.Y0, tabSeparator, inactive)
		}

		style := inactive
		if tab.Active {
			style = active
		}
		v.paintBorderRunes(s, span.X0, v.Y0, tab.Label, style)
	}
}

// renderBorderText writes text into a horizontal border row, starting just after
// the corner. Overflow is clipped rather than spilling into the content area.
func (v *View) renderBorderText(s Screen, text string, y int, style Style) {
	v.paintBorderRunes(s, v.X0+2, y, " "+text+" ", style)
}

// paintBorderRunes writes text into a border row from x, stopping at the border.
func (v *View) paintBorderRunes(s Screen, x, y int, text string, style Style) {
	for _, r := range text {
		w := max(RuneWidth(r), 1)
		if x+w > v.X1 {
			return
		}
		s.SetCell(x, y, r, style)
		x += w
	}
}

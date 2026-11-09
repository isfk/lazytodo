// Package tui is our own miniature terminal layer, the counterpart of lazygit's
// vendored pkg/gocui. It has two halves:
//
//   - Screen: a dumb cell grid (SetCell + Show + PollEvent). Implemented by the
//     tcell driver for real terminals and by FakeScreen for tests.
//   - View: a scrollable, framed buffer of text lines that paints itself onto a
//     Screen. Views know nothing about tcell or about our models, which is why
//     every rendering rule here is testable without a terminal.
package tui

// Color follows the gocui/termbox convention: zero means "whatever the terminal
// default is", 1..256 are palette indexes. The driver translates.
type Color int16

const (
	ColorDefault Color = 0
	ColorBlack   Color = 1
	ColorRed     Color = 2
	ColorGreen   Color = 3
	ColorYellow  Color = 4
	ColorBlue    Color = 5
	ColorMagenta Color = 6
	ColorCyan    Color = 7
	ColorWhite   Color = 8
)

// Attr is a bitfield of font effects, OR-able into a Style.
type Attr uint8

const (
	AttrNone      Attr = 0
	AttrBold      Attr = 1 << 0
	AttrUnderline Attr = 1 << 1
	AttrReverse   Attr = 1 << 2
	AttrDim       Attr = 1 << 3
)

// Style is the colour + effects for one cell.
type Style struct {
	Fg   Color
	Bg   Color
	Attr Attr
}

// WithAttr returns a copy with extra effects, so call sites read as
// style.WithAttr(AttrBold) rather than rebuilding the struct.
func (s Style) WithAttr(a Attr) Style {
	s.Attr |= a
	return s
}

// Package tcelldriver implements tui.Screen on a real terminal using tcell/v3.
// It is the only package in the app that knows a terminal library exists: the
// GUI talks to tui.Screen, which is also what the tests fake.
package tcelldriver

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/gdamore/tcell/v3"

	"github.com/isfk/lazytodo/pkg/tui"
)

// Screen is a tui.Screen backed by a real terminal.
type Screen struct {
	screen tcell.Screen
}

var _ tui.Screen = &Screen{}

// New returns an uninitialised screen. Call Init before drawing anything.
func New() *Screen { return &Screen{} }

// Init enters raw mode and switches to the alternate screen.
func (s *Screen) Init() error {
	screen, err := tcell.NewScreen()
	if err != nil {
		return fmt.Errorf("create terminal screen: %w", err)
	}
	if err := screen.Init(); err != nil {
		return fmt.Errorf("init terminal screen: %w", err)
	}

	s.screen = screen
	screen.HideCursor()
	// Click events only: motion and drag would just add noise to a key-driven UI.
	screen.EnableMouse(tcell.MouseButtonEvents)
	return nil
}

// Close restores the terminal. It is safe after a failed Init and safe twice.
func (s *Screen) Close() {
	if s.screen == nil {
		return
	}
	s.screen.Fini()
	s.screen = nil
}

// Size is the current terminal size in cells.
func (s *Screen) Size() (int, int) {
	if s.screen == nil {
		return 0, 0
	}
	return s.screen.Size()
}

// SetCell paints one cell. tcell takes the rune plus a list of combining
// characters; we never use combining marks, so that list is nil.
func (s *Screen) SetCell(x, y int, ch rune, style tui.Style) {
	if s.screen == nil {
		return
	}
	s.screen.SetContent(x, y, ch, nil, toTcellStyle(style))
}

// Show flushes the painted grid. tcell's Show reports nothing, so neither do we.
func (s *Screen) Show() error {
	if s.screen == nil {
		return nil
	}
	s.screen.Show()
	return nil
}

// SetClipboard puts text on the system clipboard.
//
// It prefers the platform's own helper (pbcopy, wl-copy, xclip, xsel) because
// that is silent and always works, and falls back to the terminal's OSC 52
// channel, which is what carries a copy over ssh. Reading HasClipboard() alone
// is not enough: tcell infers it from a Device Attributes reply that plenty of
// terminals never send, so a terminal that supports the clipboard perfectly well
// would report otherwise.
func (s *Screen) SetClipboard(text string) error {
	if cmd, args, ok := clipboardCommand(); ok {
		// cmd and args come from the fixed table below, never from user input,
		// and the text travels over stdin: there is no shell here, so nothing
		// can be injected.
		c := exec.Command(cmd, args...)
		c.Stdin = strings.NewReader(text)
		if err := c.Run(); err != nil {
			return fmt.Errorf("%s: %w", cmd, err)
		}
		return nil
	}

	if s.screen != nil && s.screen.HasClipboard() {
		s.screen.SetClipboard([]byte(text))
		return nil
	}

	return fmt.Errorf("no clipboard tool found (looked for a platform helper, then OSC 52)")
}

// clipboardCommand picks a platform clipboard tool, reporting false when there
// isn't one so the caller can fall back to OSC 52.
func clipboardCommand() (cmd string, args []string, ok bool) {
	switch runtime.GOOS {
	case "darwin":
		return "pbcopy", nil, true

	case "windows":
		// No dependable built-in; OSC 52 is supported by Windows Terminal.
		return "", nil, false

	default:
		// Wayland first, then X11.
		candidates := [][]string{
			{"wl-copy"},
			{"xclip", "-selection", "clipboard"},
			{"xsel", "--clipboard", "--input"},
		}
		for _, candidate := range candidates {
			if _, err := exec.LookPath(candidate[0]); err == nil {
				return candidate[0], candidate[1:], true
			}
		}
		return "", nil, false
	}
}

// reloadEvent is our own tcell event, used to wake the loop when the file on
// disk changed. tcell v3 has no PostEvent, but the event channel is public.
type reloadEvent struct{ tcell.EventTime }

// PostEvent injects an event into tcell's queue. Only EventReload is supported.
func (s *Screen) PostEvent(event tui.Event) error {
	if s.screen == nil {
		return fmt.Errorf("screen is not initialised")
	}
	if event.Type != tui.EventReload {
		return fmt.Errorf("cannot synthesise event type %d", event.Type)
	}

	select {
	case s.screen.EventQ() <- &reloadEvent{}:
	default:
	}
	return nil
}

// PollEvent reads tcell's event channel and translates the events we model.
// Mouse, paste and focus events are skipped rather than surfacing as zero values.
//
// tcell v3 replaced PollEvent with a plain channel (EventQ), which closes when
// the screen shuts down -- that closure is how a closed terminal becomes an
// error here.
func (s *Screen) PollEvent() (tui.Event, error) {
	if s.screen == nil {
		return tui.Event{}, fmt.Errorf("screen is not initialised")
	}

	for event := range s.screen.EventQ() {
		if translated, ok := toEvent(event); ok {
			return translated, nil
		}
	}

	return tui.Event{}, fmt.Errorf("terminal closed")
}

// toEvent translates one tcell event. ok is false for events we deliberately
// drop: key releases, mouse motion and release, focus changes, pastes.
func toEvent(event tcell.Event) (tui.Event, bool) {
	switch event := event.(type) {
	case *reloadEvent:
		return tui.Event{Type: tui.EventReload}, true

	case *tcell.EventResize:
		return tui.Event{Type: tui.EventResize}, true

	case *tcell.EventKey:
		// Key releases and auto-repeats carry no new intent for us.
		if !event.Pressed() {
			return tui.Event{}, false
		}
		return tui.Event{Type: tui.EventKey, Key: toKey(event)}, true

	case *tcell.EventMouse:
		// Only a button *press* is a click; motion and release would fire twice
		// for one click.
		if event.Buttons()&tcell.Button1 == 0 {
			return tui.Event{}, false
		}
		x, y := event.Position()
		return tui.Event{Type: tui.EventClick, X: x, Y: y}, true
	}

	return tui.Event{}, false
}

// keyNames maps tcell's special keys onto our own vocabulary.
var keyNames = map[tcell.Key]tui.Key{
	tcell.KeyEnter:     tui.KeyEnter,
	tcell.KeyEscape:    tui.KeyEsc,
	tcell.KeyTab:       tui.KeyTab,
	tcell.KeyBackspace: tui.KeyBackspace,
	tcell.KeyDelete:    tui.KeyDelete,
	tcell.KeyUp:        tui.KeyUp,
	tcell.KeyDown:      tui.KeyDown,
	tcell.KeyLeft:      tui.KeyLeft,
	tcell.KeyRight:     tui.KeyRight,
	tcell.KeyHome:      tui.KeyHome,
	tcell.KeyEnd:       tui.KeyEnd,
	tcell.KeyPgUp:      tui.KeyPgUp,
	tcell.KeyPgDn:      tui.KeyPgDown,
	tcell.KeyCtrlC:     tui.KeyCtrlC,
	tcell.KeyCtrlD:     tui.KeyCtrlD,
	tcell.KeyCtrlU:     tui.KeyCtrlU,
	tcell.KeyCtrlS:     tui.KeyCtrlS,
	tcell.KeyBacktab:   tui.KeyBacktab,
}

// toKey translates one key event. Printable input arrives as KeyRune with the
// text in Str; everything else is a special key we either model or ignore.
func toKey(event *tcell.EventKey) tui.Key {
	// Shift-Tab reaches us either as KeyBacktab (legacy encoding) or as KeyTab
	// plus ModShift (advanced encoding). Normalise both onto one key.
	if event.Key() == tcell.KeyTab && event.Modifiers()&tcell.ModShift != 0 {
		return tui.KeyBacktab
	}

	if event.Key() != tcell.KeyRune {
		return keyNames[event.Key()]
	}

	// Str can be more than one rune (composed sequences, pastes). Our key model
	// is single-character, so a composed sequence has no binding and is dropped.
	str := []rune(event.Str())
	if len(str) != 1 {
		return tui.Key("")
	}
	return tui.KeyOfRune(str[0])
}

// toTcellStyle translates our style into tcell's fluent one. Our Color values
// are 1-based palette indexes, matching the gocui/termbox convention that
// lazygit's own view layer uses.
func toTcellStyle(style tui.Style) tcell.Style {
	out := tcell.StyleDefault.Foreground(toTcellColor(style.Fg)).Background(toTcellColor(style.Bg))

	if style.Attr&tui.AttrBold != 0 {
		out = out.Bold(true)
	}
	if style.Attr&tui.AttrUnderline != 0 {
		out = out.Underline(true)
	}
	if style.Attr&tui.AttrReverse != 0 {
		out = out.Reverse(true)
	}
	if style.Attr&tui.AttrDim != 0 {
		out = out.Dim(true)
	}

	return out
}

func toTcellColor(c tui.Color) tcell.Color {
	if c == tui.ColorDefault {
		return tcell.ColorDefault
	}
	return tcell.PaletteColor(int(c) - 1)
}

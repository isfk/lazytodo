package tcelldriver

import (
	"runtime"
	"testing"

	"github.com/gdamore/tcell/v3"

	"github.com/shen/lazytodo/pkg/tui"
)

func TestToKeyMapsPrintableRunes(t *testing.T) {
	event := tcell.NewEventKey(tcell.KeyRune, "j", tcell.ModNone)

	if got := toKey(event); got != tui.Key("j") {
		t.Errorf("toKey(j) = %q, want %q", got, tui.Key("j"))
	}
}

// TestToKeyMapsTheSpaceBar pins the translation that makes the space bar reach
// its binding: tcell reports space as KeyRune with a plain " " string.
func TestToKeyMapsTheSpaceBar(t *testing.T) {
	event := tcell.NewEventKey(tcell.KeyRune, " ", tcell.ModNone)

	if got := toKey(event); got != tui.KeySpace {
		t.Errorf("toKey(space) = %q, want %q", got, tui.KeySpace)
	}
}

func TestToKeyMapsSpecialKeys(t *testing.T) {
	cases := []struct {
		name  string
		event *tcell.EventKey
		want  tui.Key
	}{
		{"enter", tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone), tui.KeyEnter},
		{"escape", tcell.NewEventKey(tcell.KeyEscape, "", tcell.ModNone), tui.KeyEsc},
		{"tab", tcell.NewEventKey(tcell.KeyTab, "", tcell.ModNone), tui.KeyTab},
		{"backspace", tcell.NewEventKey(tcell.KeyBackspace, "", tcell.ModNone), tui.KeyBackspace},
		{"delete", tcell.NewEventKey(tcell.KeyDelete, "", tcell.ModNone), tui.KeyDelete},
		{"up", tcell.NewEventKey(tcell.KeyUp, "", tcell.ModNone), tui.KeyUp},
		{"down", tcell.NewEventKey(tcell.KeyDown, "", tcell.ModNone), tui.KeyDown},
		{"pgup", tcell.NewEventKey(tcell.KeyPgUp, "", tcell.ModNone), tui.KeyPgUp},
		{"pgdn", tcell.NewEventKey(tcell.KeyPgDn, "", tcell.ModNone), tui.KeyPgDown},
		{"ctrl-c", tcell.NewEventKey(tcell.KeyCtrlC, "c", tcell.ModCtrl), tui.KeyCtrlC},
		{"ctrl-d", tcell.NewEventKey(tcell.KeyCtrlD, "d", tcell.ModCtrl), tui.KeyCtrlD},
		{"ctrl-s", tcell.NewEventKey(tcell.KeyCtrlS, "s", tcell.ModCtrl), tui.KeyCtrlS},
		{"backtab", tcell.NewEventKey(tcell.KeyBacktab, "", tcell.ModNone), tui.KeyBacktab},
	}

	for _, c := range cases {
		if got := toKey(c.event); got != c.want {
			t.Errorf("%s: toKey = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestToKeyDropsComposedSequences(t *testing.T) {
	// A multi-rune sequence has no single-character key to map to.
	event := tcell.NewEventKey(tcell.KeyRune, "ab", tcell.ModNone)

	if got := toKey(event); got != tui.Key("") {
		t.Errorf("toKey(ab) = %q, want empty", got)
	}
}

// TestToKeyNormalisesShiftTab covers the two encodings tcell uses for
// Shift-Tab: a dedicated key on legacy terminals, KeyTab+ModShift on advanced
// ones. Both have to reach the GUI as the same key.
func TestToKeyNormalisesShiftTab(t *testing.T) {
	legacy := tcell.NewEventKey(tcell.KeyBacktab, "", tcell.ModNone)
	if got := toKey(legacy); got != tui.KeyBacktab {
		t.Errorf("legacy shift-tab = %q, want %q", got, tui.KeyBacktab)
	}

	advanced := tcell.NewEventKey(tcell.KeyTab, "", tcell.ModShift)
	if got := toKey(advanced); got != tui.KeyBacktab {
		t.Errorf("advanced shift-tab = %q, want %q", got, tui.KeyBacktab)
	}

	plainTab := tcell.NewEventKey(tcell.KeyTab, "", tcell.ModNone)
	if got := toKey(plainTab); got != tui.KeyTab {
		t.Errorf("plain tab = %q, want %q", got, tui.KeyTab)
	}
}

func TestToEventMapsClicks(t *testing.T) {
	got, ok := toEvent(tcell.NewEventMouse(12, 4, tcell.Button1, tcell.ModNone))
	if !ok {
		t.Fatal("a left click should be translated")
	}
	if got.Type != tui.EventClick || got.X != 12 || got.Y != 4 {
		t.Errorf("got %+v, want a click at 12,4", got)
	}
}

func TestToEventSkipsNonClicks(t *testing.T) {
	cases := map[string]*tcell.EventMouse{
		"motion/release": tcell.NewEventMouse(12, 4, tcell.ButtonNone, tcell.ModNone),
		"right button":   tcell.NewEventMouse(12, 4, tcell.Button2, tcell.ModNone),
	}

	for name, event := range cases {
		if _, ok := toEvent(event); ok {
			t.Errorf("%s should be skipped", name)
		}
	}
}

func TestToEventMapsResize(t *testing.T) {
	got, ok := toEvent(tcell.NewEventResize(80, 24))
	if !ok || got.Type != tui.EventResize {
		t.Errorf("got %+v, %v; want a resize", got, ok)
	}
}

func TestClipboardCommandOnDarwin(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS only")
	}

	cmd, args, ok := clipboardCommand()
	if !ok || cmd != "pbcopy" || len(args) != 0 {
		t.Errorf("clipboardCommand() = %q, %v, %v; want pbcopy", cmd, args, ok)
	}
}

func TestToTcellColorTreatsZeroAsDefault(t *testing.T) {
	if got := toTcellColor(tui.ColorDefault); got != tcell.ColorDefault {
		t.Errorf("ColorDefault = %v, want tcell default", got)
	}
	if got := toTcellColor(tui.ColorRed); got != tcell.PaletteColor(1) {
		t.Errorf("ColorRed = %v, want palette index 1", got)
	}
}

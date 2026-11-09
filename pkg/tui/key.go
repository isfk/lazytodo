package tui

// Key is a canonical, driver-independent key name. Single printable characters
// are just themselves ("j", "a", "/"), everything else uses one of the constants
// below. Keeping keys as strings means the whole binding layer is comparable and
// plain data, with no dependency on the terminal library's key codes.
type Key string

const (
	KeyEsc       Key = "esc"
	KeyEnter     Key = "enter"
	KeySpace     Key = "space"
	KeyTab       Key = "tab"
	KeyBackspace Key = "backspace"
	KeyDelete    Key = "delete"
	KeyUp        Key = "up"
	KeyDown      Key = "down"
	KeyLeft      Key = "left"
	KeyRight     Key = "right"
	KeyHome      Key = "home"
	KeyEnd       Key = "end"
	KeyPgUp      Key = "pgup"
	KeyPgDown    Key = "pgdown"
	KeyCtrlC     Key = "ctrl+c"
	KeyCtrlD     Key = "ctrl+d"
	KeyCtrlU     Key = "ctrl+u"
	KeyCtrlS     Key = "ctrl+s"
	KeyBacktab   Key = "backtab"
)

var specialKeys = map[Key]bool{
	KeyEsc: true, KeyEnter: true, KeyTab: true,
	KeyBackspace: true, KeyDelete: true, KeyUp: true, KeyDown: true,
	KeyLeft: true, KeyRight: true, KeyHome: true, KeyEnd: true,
	KeyPgUp: true, KeyPgDown: true, KeyCtrlC: true, KeyCtrlD: true,
	KeyCtrlU: true, KeyCtrlS: true, KeyBacktab: true,
}

// RuneOfKey returns the printable character for a key, and false for everything
// that isn't one. Text inputs use it to append typed characters.
//
// KeySpace is printable: a terminal reports the space bar as an ordinary rune,
// and both a text field and a binding have to agree on what it means.
func RuneOfKey(k Key) (rune, bool) {
	if k == KeySpace {
		return ' ', true
	}
	if specialKeys[k] {
		return 0, false
	}

	runes := []rune(string(k))
	if len(runes) != 1 {
		return 0, false
	}
	return runes[0], true
}

// KeyOfRune is the inverse of RuneOfKey, used by the drivers and the tests.
//
// Space is normalised onto KeySpace here, which is what keeps the space bar
// working: without it the driver would emit Key(" ") while the configuration
// binds KeySpace, and the two would never compare equal.
func KeyOfRune(r rune) Key {
	if r == ' ' {
		return KeySpace
	}
	return Key(r)
}

// keyLabels are the symbols the options bar shows for keys that have no
// printable character of their own.
var keyLabels = map[Key]string{
	KeyEsc:       "esc",
	KeyEnter:     "⏎",
	KeySpace:     "␣",
	KeyTab:       "⇥",
	KeyBackspace: "⌫",
	KeyDelete:    "del",
	KeyUp:        "↑",
	KeyDown:      "↓",
	KeyLeft:      "←",
	KeyRight:     "→",
	KeyHome:      "home",
	KeyEnd:       "end",
	KeyPgUp:      "pgup",
	KeyPgDown:    "pgdn",
	KeyCtrlC:     "^c",
	KeyCtrlD:     "^d",
	KeyCtrlU:     "^u",
	KeyCtrlS:     "^s",
	KeyBacktab:   "⇧⇥",
}

// Label is how the key should be shown to the user.
func (k Key) Label() string {
	if label, ok := keyLabels[k]; ok {
		return label
	}
	return string(k)
}

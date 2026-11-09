package tui

// EventType distinguishes what the user did.
type EventType int

const (
	EventKey EventType = iota
	EventResize
	EventClick
)

// Event is what a Screen hands to the main loop. Only the fields relevant to
// Type are meaningful.
type Event struct {
	Type EventType
	Key  Key // EventKey only
	X, Y int // EventClick only: the cell that was clicked
}

// Screen is the entire surface the GUI needs from a terminal: a grid we can
// paint cells into, a flush, and a blocking event read. Swapping the real driver
// for FakeScreen is what makes headless integration tests possible.
type Screen interface {
	// Init enters raw mode, switches to the alternate screen and starts
	// reporting clicks. Safe to call once.
	Init() error
	// Close restores the terminal. Must be safe to call after a failed Init.
	Close()
	// Size is the current grid size in cells.
	Size() (width, height int)
	// SetCell paints one cell. Coordinates outside the grid are ignored.
	SetCell(x, y int, ch rune, style Style)
	// Show flushes the painted grid to the terminal.
	Show() error
	// PollEvent blocks until the user does something.
	PollEvent() (Event, error)
	// SetClipboard posts text to the system clipboard. Drivers that can't must
	// return an error rather than silently dropping the text.
	SetClipboard(text string) error
}

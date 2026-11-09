package tui

import (
	"fmt"
	"strings"
	"sync"
)

// Cell is one painted character plus its style, as recorded by FakeScreen.
type Cell struct {
	Ch    rune
	Style Style
}

// FakeScreen is a Screen backed by an in-memory grid. It is how integration
// tests drive the real GUI without a terminal: they press keys, the GUI renders
// into this grid, and the test asserts on the text the user would have seen.
type FakeScreen struct {
	mu     sync.Mutex
	width  int
	height int
	grid   [][]Cell

	// events is the queue PollEvent drains. Tests push with Press / Resize.
	events chan Event
	closed bool

	// Shows counts flushes, so tests can assert that a render actually happened.
	Shows int

	// clipboard is whatever SetClipboard was last handed.
	clipboard string
}

// NewFakeScreen builds an empty screen of the given size.
func NewFakeScreen(width, height int) *FakeScreen {
	s := &FakeScreen{width: width, height: height, events: make(chan Event, 64)}
	s.allocate()
	return s
}

func (s *FakeScreen) allocate() {
	s.grid = make([][]Cell, s.height)
	for y := range s.grid {
		s.grid[y] = make([]Cell, s.width)
		for x := range s.grid[y] {
			s.grid[y][x] = Cell{Ch: ' ', Style: Style{}}
		}
	}
}

func (s *FakeScreen) Init() error { return nil }

func (s *FakeScreen) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	close(s.events)
}

func (s *FakeScreen) Size() (int, int) { return s.width, s.height }

func (s *FakeScreen) SetCell(x, y int, ch rune, style Style) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if x < 0 || y < 0 || x >= s.width || y >= s.height {
		return
	}
	s.grid[y][x] = Cell{Ch: ch, Style: style}

	// A double-width character covers the next cell. Mark it so Text and Region
	// skip it instead of emitting a phantom space, the way a terminal does.
	if RuneWidth(ch) > 1 && x+1 < s.width {
		s.grid[y][x+1] = Cell{Ch: 0}
	}
}

func (s *FakeScreen) Show() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Shows++
	return nil
}

func (s *FakeScreen) PollEvent() (Event, error) {
	ev, ok := <-s.events
	if !ok {
		return Event{}, fmt.Errorf("screen closed")
	}
	return ev, nil
}

// Press queues a key event, the test-side equivalent of the user typing.
func (s *FakeScreen) Press(key Key) {
	s.events <- Event{Type: EventKey, Key: key}
}

// Resize changes the grid size and queues a resize event.
func (s *FakeScreen) Resize(width, height int) {
	s.mu.Lock()
	s.width, s.height = width, height
	s.allocate()
	s.mu.Unlock()

	s.events <- Event{Type: EventResize}
}

// SetClipboard records the text, so a test can assert what a copy put on the
// system clipboard.
func (s *FakeScreen) SetClipboard(text string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clipboard = text
	return nil
}

// Clipboard is the last text handed to SetClipboard.
func (s *FakeScreen) Clipboard() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.clipboard
}

// Click queues a left click at the given cell, the test-side equivalent of the
// user reaching for the mouse.
func (s *FakeScreen) Click(x, y int) {
	s.events <- Event{Type: EventClick, X: x, Y: y}
}

// Text returns the whole grid as plain lines, trailing spaces trimmed. This is
// what a test asserts on: exactly the characters the user would see.
func (s *FakeScreen) Text() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	lines := make([]string, s.height)
	for y, row := range s.grid {
		var b strings.Builder
		for _, c := range row {
			if c.Ch == 0 {
				continue // continuation of a wide character
			}
			b.WriteRune(c.Ch)
		}
		lines[y] = strings.TrimRight(b.String(), " ")
	}
	return lines
}

// CellAt returns the cell at the given position; the zero value when out of
// bounds. Tests use it to check a specific style was applied.
func (s *FakeScreen) CellAt(x, y int) Cell {
	s.mu.Lock()
	defer s.mu.Unlock()
	if x < 0 || y < 0 || x >= s.width || y >= s.height {
		return Cell{}
	}
	return s.grid[y][x]
}

// Region returns the text inside the given inclusive rectangle, one string per
// row, trailing spaces trimmed. Tests use it to read a single pane.
func (s *FakeScreen) Region(x0, y0, x1, y1 int) []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	var out []string
	for y := y0; y <= y1 && y < s.height; y++ {
		if y < 0 {
			continue
		}
		var b strings.Builder
		for x := x0; x <= x1 && x < s.width; x++ {
			if x < 0 {
				continue
			}
			if c := s.grid[y][x]; c.Ch != 0 {
				b.WriteRune(c.Ch)
			}
		}
		out = append(out, strings.TrimRight(b.String(), " "))
	}
	return out
}

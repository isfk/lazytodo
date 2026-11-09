package presentation

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/isfk/lazytodo/pkg/gui/types"
	"github.com/isfk/lazytodo/pkg/i18n"
	"github.com/isfk/lazytodo/pkg/models"
	"github.com/isfk/lazytodo/pkg/tui"
)

// Markers for the status column: hollow circle, half-filled circle, tick.
// NOTE: ○ and ◐ are East Asian "ambiguous width" characters -- most terminals
// give them one cell, some CJK-configured ones give them two. Our column maths
// assumes one cell, so on such a terminal the list can shift by a column.
const (
	MarkerTodo  = "○"
	MarkerDoing = "◐"
	MarkerDone  = "✓"
)

// listIndent is the left gap before the status marker, so the list doesn't look
// glued to the pane border. Two columns; four also reads fine if the terminal is
// wide.
const listIndent = " "

// MarkerFor is the glyph the list shows for a status.
func MarkerFor(status models.Status) string {
	switch status {
	case models.StatusDoing:
		return MarkerDoing
	case models.StatusDone:
		return MarkerDone
	default:
		return MarkerTodo
	}
}

// StatusLabel is the human name of a status.
func StatusLabel(status models.Status, tr *i18n.TranslationSet) string {
	switch status {
	case models.StatusDoing:
		return tr.FilterDoing
	case models.StatusDone:
		return tr.FilterDone
	case models.StatusAll:
		return tr.FilterAll
	default:
		return tr.FilterTodo
	}
}

// StyleFor is the colour a row gets for its status. Rows are styled whole,
// which is enough to read three states apart and keeps the view free of escape
// codes. (lazygit goes further and puts ANSI codes inside the display strings;
// that is the upgrade path once per-cell colour is needed.)
func StyleFor(status models.Status) tui.Style {
	switch status {
	case models.StatusDoing:
		return tui.Style{Fg: tui.ColorYellow, Attr: tui.AttrBold}
	case models.StatusDone:
		return tui.Style{Fg: tui.ColorGreen, Attr: tui.AttrDim}
	default:
		return tui.Style{Fg: tui.ColorDefault}
	}
}

// StatusTabs is the filter strip drawn on the list pane's top border, in
// display order. The set comes straight from the sketch: All / Doing / Done.
// Adding models.StatusTodo here is all it takes to get a fourth tab.
var statusTabStatuses = []models.Status{
	models.StatusAll,
	models.StatusDoing,
	models.StatusDone,
}

// StatusTabCount is how many filter tabs there are.
func StatusTabCount() int { return len(statusTabStatuses) }

// StatusTabs builds the strip, marking the active filter.
func StatusTabs(active models.Status, tr *i18n.TranslationSet) []tui.Tab {
	tabs := make([]tui.Tab, 0, len(statusTabStatuses))
	for _, status := range statusTabStatuses {
		tabs = append(tabs, tui.Tab{Label: StatusLabel(status, tr), Active: status == active})
	}
	return tabs
}

// StatusTabIndex is the tab row a status sits on. Statuses that have no tab
// (such as Todo) map to the first row, which is "All".
func StatusTabIndex(status models.Status) int {
	for i, tabStatus := range statusTabStatuses {
		if tabStatus == status {
			return i
		}
	}
	return 0
}

// StatusAtTab maps a tab index to the status it filters on.
func StatusAtTab(idx int) models.Status {
	if idx < 0 || idx >= len(statusTabStatuses) {
		return models.StatusAll
	}
	return statusTabStatuses[idx]
}

// TodoLines renders the list plus the per-row style that goes with it, so a
// finished todo can be dimmed without the view knowing what a todo is.
func TodoLines(todos []*models.Todo, width int) (lines []string, styles []tui.Style) {
	rows := make([][]string, 0, len(todos))
	styles = make([]tui.Style, 0, len(todos))

	for _, todo := range todos {
		rows = append(rows, []string{listIndent + MarkerFor(todo.Status), todo.Title})
		styles = append(styles, StyleFor(todo.Status))
	}

	return RenderColumns(rows, width), styles
}

// DetailSourceLines is the description split into rows, untruncated: exactly
// what a copy puts on the clipboard. Empty when there is nothing to copy.
func DetailSourceLines(todo *models.Todo) []string {
	if todo == nil {
		return nil
	}

	detail := strings.TrimRight(todo.Detail, "\n")
	if strings.TrimSpace(detail) == "" {
		return nil
	}
	return strings.Split(detail, "\n")
}

// DetailLines renders the description for the pane, truncating each row so the
// display is honest about what fits. Rows line up 1:1 with DetailSourceLines.
func DetailLines(todo *models.Todo, width int, tr *i18n.TranslationSet) []string {
	if todo == nil {
		return nil
	}

	source := DetailSourceLines(todo)
	if len(source) == 0 {
		return []string{Truncate(tr.EmptyDetail, width)}
	}

	lines := make([]string, 0, len(source))
	for _, line := range source {
		lines = append(lines, Truncate(line, width))
	}
	return lines
}

// PopupLines renders an open popup, and returns the index of the line holding
// the caret so the caller can scroll it into view (-1 means "no caret").
//
// The shape is one label line per field, the field's text indented under it, and
// a caret block appended to the last line of whichever field has focus. Multi-line
// text simply becomes several content lines, which is all "supports multiple
// lines" needs from the view.
func PopupLines(popup *types.PopupState, width int) (lines []string, caretLine int) {
	// A confirmation has no fields: the body is just the question, wrapped to
	// the popup so the user can read what they are about to destroy.
	if len(popup.Fields) == 0 {
		return WrapText(popup.Prompt, width), -1
	}

	caretLine = -1
	textWidth := width - 2

	for i := range popup.Fields {
		field := &popup.Fields[i]
		focused := i == popup.Focus

		if focused {
			lines = append(lines, "▸ "+field.Label)
		} else {
			lines = append(lines, "  "+field.Label)
		}

		for _, textLine := range strings.Split(field.Text, "\n") {
			lines = append(lines, "  "+Truncate(textLine, fieldWidth(focused, textWidth)))
		}

		if focused {
			caretLine = len(lines) - 1
			lines[caretLine] += "█"
		}

		lines = append(lines, "")
	}

	if len(lines) > 0 {
		lines = lines[:len(lines)-1]
	}

	return lines, caretLine
}

// WrapText breaks text into lines of at most width characters, honouring any
// newlines already in it. Long words (and CJK text, which has no spaces) are
// hard-broken so nothing silently disappears.
func WrapText(text string, width int) []string {
	if width <= 0 {
		return []string{text}
	}

	var lines []string
	for _, paragraph := range strings.Split(text, "\n") {
		lines = append(lines, wrapLine(paragraph, width)...)
	}
	return lines
}

func wrapLine(line string, width int) []string {
	if tui.DisplayWidth(line) <= width {
		return []string{line}
	}

	var out []string
	current := ""

	for _, word := range strings.Split(line, " ") {
		// Hard-break a word that can't fit on a line of its own; CJK text has no
		// spaces, so this is the path it takes.
		for tui.DisplayWidth(word) > width {
			if current != "" {
				out = append(out, current)
				current = ""
			}

			head, tail := CutToWidth(word, width)
			if head == "" {
				// Width is smaller than one character; take it anyway so the loop
				// always makes progress.
				r, size := utf8.DecodeRuneInString(word)
				head, tail = string(r), word[size:]
			}

			out = append(out, head)
			word = tail
		}

		candidate := word
		if current != "" {
			candidate = current + " " + word
		}
		if tui.DisplayWidth(candidate) <= width {
			current = candidate
			continue
		}

		if current != "" {
			out = append(out, current)
		}
		current = word
	}

	if current != "" {
		out = append(out, current)
	}
	return out
}

// fieldWidth is how much text fits: the focused field gives up one column for
// the caret block so the caret is never the thing that gets clipped.
func fieldWidth(focused bool, textWidth int) int {
	if focused {
		return textWidth - 1
	}
	return textWidth
}

// StatusLine is the bottom status bar. Parts that would read as zero are
// dropped so an empty list isn't a wall of noughts.
func StatusLine(counts models.ByStatus, textFilter string, statusFilter models.Status, tr *i18n.TranslationSet) string {
	line := fmt.Sprintf(tr.DoneCount, counts.Done, counts.Total())

	if counts.Doing > 0 {
		line += " · " + fmt.Sprintf(tr.DoingCount, counts.Doing)
	}
	if statusFilter != models.StatusAll {
		line += " · " + fmt.Sprintf(tr.OnlyStatus, StatusLabel(statusFilter, tr))
	}
	if textFilter != "" {
		line += " · " + fmt.Sprintf(tr.TextFilter, textFilter)
	}

	return line
}

// OptionsLine builds the keybinding hint bar from the bindings that asked to be
// advertised, so adding a binding never means editing a hardcoded string.
func OptionsLine(hints []string) string {
	return strings.Join(hints, " ")
}

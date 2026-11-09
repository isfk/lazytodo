// Package presentation turns models into display rows. Nothing here touches a
// view or a screen, so every rule (column alignment, truncation, the done
// marker) is a pure function that can be unit tested -- which is exactly how
// lazygit's own presentation package is structured.
package presentation

import (
	"strings"

	"github.com/isfk/lazytodo/pkg/tui"
)

// RenderColumns lays out rows of cells into fixed-width lines.
//
// Each row is a slice of column values. All but the last column are padded to
// the widest value in that column so the list reads as a table; the last column
// takes the remaining width and is truncated with an ellipsis if it overflows.
// A width <= 0 means "unbounded".
func RenderColumns(rows [][]string, width int) []string {
	if len(rows) == 0 {
		return nil
	}

	cols := 1
	for _, row := range rows {
		if len(row) > cols {
			cols = len(row)
		}
	}

	widths := make([]int, cols)
	for _, row := range rows {
		for i, cell := range row {
			if n := tui.DisplayWidth(cell); n > widths[i] {
				widths[i] = n
			}
		}
	}

	lines := make([]string, len(rows))
	for r, row := range rows {
		var b strings.Builder
		for i := 0; i < cols; i++ {
			cell := ""
			if i < len(row) {
				cell = row[i]
			}

			if i < cols-1 {
				b.WriteString(PadRight(cell, widths[i]+1))
				continue
			}
			b.WriteString(cell)
		}

		lines[r] = strings.TrimRight(b.String(), " ")
		if width > 0 {
			lines[r] = Truncate(lines[r], width)
		}
	}

	return lines
}

// PadRight appends spaces until s occupies at least width cells.
func PadRight(s string, width int) string {
	if n := tui.DisplayWidth(s); n < width {
		return s + strings.Repeat(" ", width-n)
	}
	return s
}

// Truncate shortens s to at most width cells, marking the cut with an ellipsis so
// the user can tell text was dropped. A wide character is never split across the
// boundary, so the result may be one cell short of width.
func Truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if tui.DisplayWidth(s) <= width {
		return s
	}
	if width == 1 {
		return "…"
	}

	head, _ := CutToWidth(s, width-1)
	return head + "…"
}

// CutToWidth splits s into the longest prefix that fits in width cells and
// whatever is left. A character that would straddle the boundary starts the
// remainder instead of being halved.
func CutToWidth(s string, width int) (head string, tail string) {
	used := 0
	for i, r := range s {
		w := tui.RuneWidth(r)
		if used+w > width {
			return s[:i], s[i:]
		}
		used += w
	}
	return s, ""
}

package presentation

import (
	"strings"
	"testing"

	"github.com/shen/lazytodo/pkg/gui/types"
	"github.com/shen/lazytodo/pkg/i18n"
	"github.com/shen/lazytodo/pkg/models"
	"github.com/shen/lazytodo/pkg/tui"
)

func TestRenderColumnsAlignsColumns(t *testing.T) {
	rows := [][]string{
		{"✓", "short"},
		{"·", "a much longer title"},
	}

	got := RenderColumns(rows, 0)
	want := []string{"✓ short", "· a much longer title"}

	if len(got) != len(want) {
		t.Fatalf("got %d lines, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestRenderColumnsTruncatesToWidth(t *testing.T) {
	rows := [][]string{{"·", "write the store"}}

	got := RenderColumns(rows, 10)
	if want := "· write t…"; got[0] != want {
		t.Errorf("got %q, want %q", got[0], want)
	}
	if tui.DisplayWidth(got[0]) != 10 {
		t.Errorf("line is %d chars wide, want exactly 10", tui.DisplayWidth(got[0]))
	}
}

func TestRenderColumnsEmpty(t *testing.T) {
	if got := RenderColumns(nil, 20); got != nil {
		t.Errorf("got %v, want nil", got)
	}
}

func TestTruncate(t *testing.T) {
	cases := []struct {
		in    string
		width int
		want  string
	}{
		{"hello", 10, "hello"},
		{"hello", 5, "hello"},
		{"hello", 4, "hel…"},
		{"hello", 1, "…"},
		{"hello", 0, ""},
		{"保存记事", 3, "保…"},
	}

	for _, c := range cases {
		if got := Truncate(c.in, c.width); got != c.want {
			t.Errorf("Truncate(%q, %d) = %q, want %q", c.in, c.width, got, c.want)
		}
	}
}

// TestDisplayWidthTellsChineseFromEnglish is the whole point of the width helper:
// a Chinese character costs two cells, a Latin one costs one.
func TestDisplayWidthTellsChineseFromEnglish(t *testing.T) {
	cases := map[string]int{
		"abc":      3,
		"保存":       4,
		"保存 abc":   8,
		"":         0,
		"✓":        1,
		"buy milk": 8,
	}

	for in, want := range cases {
		if got := tui.DisplayWidth(in); got != want {
			t.Errorf("tui.DisplayWidth(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestTodoLinesMarkTheThreeStates(t *testing.T) {
	todos := []*models.Todo{
		{ID: 1, Title: "done", Status: models.StatusDone},
		{ID: 2, Title: "doing", Status: models.StatusDoing},
		{ID: 3, Title: "todo", Status: models.StatusTodo},
	}

	lines, styles := TodoLines(todos, 40)

	if len(lines) != 3 || len(styles) != 3 {
		t.Fatalf("got %d lines and %d styles, want 3 of each", len(lines), len(styles))
	}

	wantPrefixes := []string{MarkerDone, MarkerDoing, MarkerTodo}
	for i, marker := range wantPrefixes {
		want := listIndent + marker + " "
		if !strings.HasPrefix(lines[i], want) {
			t.Errorf("line %d = %q, want it to start with %q", i, lines[i], want)
		}
	}

	if styles[1].Fg != tui.ColorYellow {
		t.Errorf("doing row colour = %v, want yellow", styles[1].Fg)
	}
	if styles[2].Fg != tui.ColorDefault {
		t.Errorf("todo row colour = %v, want default", styles[2].Fg)
	}
}

func TestTodoLinesIndentTheMarker(t *testing.T) {
	lines, _ := TodoLines([]*models.Todo{{ID: 1, Title: "x", Status: models.StatusTodo}}, 20)

	if got := len(lines[0]) - len(strings.TrimLeft(lines[0], " ")); got != len(listIndent) {
		t.Errorf("left gap is %d columns, want %d: %q", got, len(listIndent), lines[0])
	}
}

var tr = i18n.EnglishSet()

func TestDetailLines(t *testing.T) {
	if got := DetailLines(nil, 20, tr); got != nil {
		t.Errorf("nil todo = %v, want nil", got)
	}

	got := DetailLines(&models.Todo{Detail: "a\nb"}, 20, tr)
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("got %v, want [a b]", got)
	}

	blank := DetailLines(&models.Todo{Detail: "   \n\n"}, 20, tr)
	if len(blank) != 1 || blank[0] != "(no description)" {
		t.Errorf("got %v, want the placeholder", blank)
	}
}

func TestDetailSourceLinesAreUntruncated(t *testing.T) {
	long := strings.Repeat("x", 50)
	todo := &models.Todo{Detail: long + "\nsecond"}

	source := DetailSourceLines(todo)
	if len(source) != 2 || source[0] != long {
		t.Fatalf("source = %v, want the full untruncated line", source)
	}

	rendered := DetailLines(todo, 10, tr)
	if tui.DisplayWidth(rendered[0]) != 10 {
		t.Errorf("rendered line is %d wide, want it truncated to 10", tui.DisplayWidth(rendered[0]))
	}

	if got := DetailSourceLines(nil); got != nil {
		t.Errorf("nil todo = %v, want nil", got)
	}
	if got := DetailSourceLines(&models.Todo{Detail: "  \n"}); got != nil {
		t.Errorf("blank detail = %v, want nil", got)
	}
}

func TestPopupLinesMarkFocusAndCaret(t *testing.T) {
	popup := &types.PopupState{Fields: []types.Field{
		{Label: "Title", Text: "buy milk"},
		{Label: "Description", Text: "line one\nline two", Multiline: true},
	}}

	lines, caret := PopupLines(popup, 40)

	want := []string{
		"▸ Title",
		"  buy milk█",
		"",
		"  Description",
		"  line one",
		"  line two",
	}
	assertLines(t, lines, want)
	if caret != 1 {
		t.Errorf("caret line = %d, want 1", caret)
	}

	// Focusing the multi-line field moves both the marker and the caret.
	popup.Focus = 1
	lines, caret = PopupLines(popup, 40)

	want = []string{
		"  Title",
		"  buy milk",
		"",
		"▸ Description",
		"  line one",
		"  line two█",
	}
	assertLines(t, lines, want)
	if caret != 5 {
		t.Errorf("caret line = %d, want 5", caret)
	}
}

func TestPopupLinesClipsToThePopupWidth(t *testing.T) {
	popup := &types.PopupState{Fields: []types.Field{
		{Label: "Title", Text: "a fairly long title that will not fit"},
	}}

	lines, _ := PopupLines(popup, 12)
	if got := tui.DisplayWidth(lines[1]); got != 12 {
		t.Errorf("text line is %d wide, want 12: %q", got, lines[1])
	}
}

func TestStatusLine(t *testing.T) {
	cases := []struct {
		name   string
		counts models.ByStatus
		text   string
		status models.Status
		want   string
	}{
		{"plain", models.ByStatus{Todo: 2, Done: 1}, "", models.StatusAll, "1/3 done"},
		{"with doing", models.ByStatus{Todo: 1, Doing: 1}, "", models.StatusAll, "0/2 done · 1 doing"},
		{"status filter", models.ByStatus{Todo: 1, Doing: 2}, "", models.StatusDoing, "0/3 done · 2 doing · only Doing"},
		{"text filter", models.ByStatus{Todo: 1}, "gui", models.StatusAll, "0/1 done · filter: \"gui\""},
	}

	for _, c := range cases {
		if got := StatusLine(c.counts, c.text, c.status, tr); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

func TestPopupLinesRendersAPrompt(t *testing.T) {
	popup := &types.PopupState{Prompt: `Delete "a fairly long title here"?`}

	lines, caret := PopupLines(popup, 20)
	if caret != -1 {
		t.Errorf("caret line = %d, want -1 for a confirmation", caret)
	}
	if len(lines) < 2 {
		t.Fatalf("expected the question to wrap, got %v", lines)
	}
	for _, line := range lines {
		if tui.DisplayWidth(line) > 20 {
			t.Errorf("line %q is %d wide, want <= 20", line, tui.DisplayWidth(line))
		}
	}
}

func TestWrapText(t *testing.T) {
	// Packs as many words as fit, without wasting a full line.
	assertLines(t, WrapText("aaa bbb ccc", 7), []string{"aaa bbb", "ccc"})

	// A word longer than the line is hard-broken rather than dropped.
	assertLines(t, WrapText("abcdefghij", 4), []string{"abcd", "efgh", "ij"})

	// CJK has no spaces and every character is two cells wide, so a two-cell
	// line holds exactly one of them.
	assertLines(t, WrapText("保存记事本", 2), []string{"保", "存", "记", "事", "本"})

	// Existing newlines are honoured.
	assertLines(t, WrapText("a\nb", 10), []string{"a", "b"})

	// A non-positive width means "don't wrap".
	assertLines(t, WrapText("aaaa", 0), []string{"aaaa"})
}

func TestStatusTabs(t *testing.T) {
	tabs := StatusTabs(models.StatusDoing, tr)

	wantLabels := []string{"All", "Doing", "Done"}
	if len(tabs) != len(wantLabels) {
		t.Fatalf("got %d tabs, want %d", len(tabs), len(wantLabels))
	}
	for i, want := range wantLabels {
		if tabs[i].Label != want {
			t.Errorf("tab %d = %q, want %q", i, tabs[i].Label, want)
		}
	}

	if !tabs[1].Active {
		t.Error("the Doing tab should carry the active chip")
	}
	if tabs[0].Active || tabs[2].Active {
		t.Error("only one tab may be active")
	}
}

func TestStatusAtTab(t *testing.T) {
	if got := StatusAtTab(0); got != models.StatusAll {
		t.Errorf("tab 0 = %q, want All", got)
	}
	if got := StatusAtTab(1); got != models.StatusDoing {
		t.Errorf("tab 1 = %q, want Doing", got)
	}
	if got := StatusAtTab(2); got != models.StatusDone {
		t.Errorf("tab 2 = %q, want Done", got)
	}
	if got := StatusAtTab(99); got != models.StatusAll {
		t.Errorf("out-of-range tab = %q, want All", got)
	}
	if got := StatusAtTab(-1); got != models.StatusAll {
		t.Errorf("negative tab = %q, want All", got)
	}
}

func TestOptionsLineJoinsHints(t *testing.T) {
	if got, want := OptionsLine([]string{"[a] Add", "[/] Filter"}), "[a] Add [/] Filter"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if got := OptionsLine(nil); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func assertLines(t *testing.T, got, want []string) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("got %d lines, want %d:\n%s", len(got), len(want), strings.Join(got, "\n"))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d = %q, want %q", i, got[i], want[i])
		}
	}
}

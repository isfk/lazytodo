package integration_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/isfk/lazytodo/pkg/commands"
	"github.com/isfk/lazytodo/pkg/i18n"
	. "github.com/isfk/lazytodo/pkg/integration"
	"github.com/isfk/lazytodo/pkg/models"
	"github.com/isfk/lazytodo/pkg/tui"
)

// ---------------------------------------------------------------------------
// three-state cycle
// ---------------------------------------------------------------------------

func TestCyclingWalksThroughAllThreeStates(t *testing.T) {
	RunTest(t, TestCase{
		Setup: func(shell *Shell) { shell.Add("write the store") },
		Run: func(d *TestDriver) {
			d.Todos().Lines(Contains("○ write the store").IsSelected())
			d.Status().Lines(Equals("0/1 done"))

			d.Press(d.Keys.Todos.CycleStatus)
			d.Todos().Lines(Contains("◐ write the store").IsSelected())
			d.Status().Lines(Equals("0/1 done · 1 doing"))

			d.Press(d.Keys.Todos.CycleStatus)
			d.Todos().Lines(Contains("✓ write the store").IsSelected())
			d.Status().Lines(Equals("1/1 done"))

			d.Press(d.Keys.Todos.CycleStatus)
			d.Todos().Lines(Contains("○ write the store").IsSelected())
			d.Status().Lines(Equals("0/1 done"))
		},
	})
}

// TestSpaceBarFromTheTerminalCyclesStatus guards the seam between the driver and
// the bindings: a terminal reports the space bar as a plain rune, so the
// translation has to land on the key the configuration binds. (This broke once:
// the driver emitted Key(" ") while the config bound KeySpace.)
func TestSpaceBarFromTheTerminalCyclesStatus(t *testing.T) {
	RunTest(t, TestCase{
		Setup: func(shell *Shell) { shell.Add("one") },
		Run: func(d *TestDriver) {
			d.Press(tui.KeyOfRune(' '))

			if got := d.Store().Todos[0].Status; got != models.StatusDoing {
				t.Fatalf("status = %q, want %q: the space bar never reached its binding", got, models.StatusDoing)
			}
			d.Todos().Lines(Contains("\u25d0 one").IsSelected())
		},
	})
}

func TestSpaceBarTypesASpaceInsideTheForm(t *testing.T) {
	RunTest(t, TestCase{
		Run: func(d *TestDriver) {
			d.Press(d.Keys.Todos.Add)
			d.Type("buy milk")
			d.Popup().Includes("  buy milk\u2588")

			d.Press(tui.KeyEnter)
			if got := d.Store().Todos[0].Title; got != "buy milk" {
				t.Errorf("title = %q, want %q", got, "buy milk")
			}
		},
	})
}

func TestStatusChangeIsPersisted(t *testing.T) {
	RunTest(t, TestCase{
		Setup: func(shell *Shell) { shell.Add("write the store") },
		Run: func(d *TestDriver) {
			d.Press(d.Keys.Todos.CycleStatus)

			if got := d.Store().Todos[0].Status; got != models.StatusDoing {
				t.Errorf("stored status = %q, want %q", got, models.StatusDoing)
			}
		},
	})
}

// ---------------------------------------------------------------------------
// navigation
// ---------------------------------------------------------------------------

func TestArrowKeysMoveTheSelection(t *testing.T) {
	RunTest(t, TestCase{
		Setup: func(shell *Shell) { shell.Add("one", "two", "three") },
		Run: func(d *TestDriver) {
			d.Press(tui.KeyDown)
			if got := d.Todos().Cursor(); got != 1 {
				t.Fatalf("cursor after ↓ = %d, want 1", got)
			}

			d.Press(tui.KeyDown)
			if got := d.Todos().Cursor(); got != 2 {
				t.Fatalf("cursor after ↓↓ = %d, want 2", got)
			}

			// Arrows stop at the ends, same as j/k.
			d.Press(tui.KeyDown)
			if got := d.Todos().Cursor(); got != 2 {
				t.Fatalf("cursor past the end = %d, want 2", got)
			}

			d.Press(tui.KeyUp)
			if got := d.Todos().Cursor(); got != 1 {
				t.Fatalf("cursor after ↑ = %d, want 1", got)
			}
		},
	})
}

func TestNavigationClampsAtBothEnds(t *testing.T) {
	RunTest(t, TestCase{
		Setup: func(shell *Shell) { shell.Add("one", "two") },
		Run: func(d *TestDriver) {
			d.Press(d.Keys.Todos.Next, d.Keys.Todos.Next, d.Keys.Todos.Next)
			if got := d.Todos().Cursor(); got != 1 {
				t.Errorf("cursor = %d, want 1 (last row)", got)
			}

			d.Press(d.Keys.Todos.Prev, d.Keys.Todos.Prev, d.Keys.Todos.Prev)
			if got := d.Todos().Cursor(); got != 0 {
				t.Errorf("cursor = %d, want 0 (first row)", got)
			}

			d.Press(d.Keys.Todos.Bottom)
			if got := d.Todos().Cursor(); got != 1 {
				t.Errorf("cursor after jump-to-bottom = %d, want 1", got)
			}
		},
	})
}

// ---------------------------------------------------------------------------
// filter tabs on the title row
// ---------------------------------------------------------------------------

func TestChineseInterface(t *testing.T) {
	RunTest(t, TestCase{
		Language: i18n.Chinese,
		Setup:    func(shell *Shell) { shell.Add("写存储层") },
		Run: func(d *TestDriver) {
			border := d.Todos().TopBorder()
			for _, want := range []string{"待办", "全部", "进行中", "完成"} {
				if !strings.Contains(border, want) {
					t.Errorf("top border %q is missing %q", border, want)
				}
			}
			if got := d.Detail().Title(); got != "详情" {
				t.Errorf("detail title = %q, want 详情", got)
			}

			d.Status().Lines(Contains("完成"))
			d.Options().Includes("新增", "编辑", "删除")

			d.Press(d.Keys.Todos.Add)
			if got := d.Popup().Title(); got != "新增待办" {
				t.Errorf("popup title = %q, want 新增待办", got)
			}
			d.Popup().Includes("标题", "详情")
			d.Press(tui.KeyEsc)
		},
	})
}

// TestOutsideWriteAppearsAfterReload is the sync path an agent uses: it writes
// .lazytodo.json itself, and the TUI picks the file up on the next reload.
func TestOutsideWriteAppearsAfterReload(t *testing.T) {
	RunTest(t, TestCase{
		Setup: func(shell *Shell) {
			shell.Add("one", "two")
			shell.Cycle(1) // one -> doing
		},
		Run: func(d *TestDriver) {
			d.Press(d.Keys.Todos.Next) // park the cursor on "two"

			// Another process does a read-modify-write, exactly like the extension.
			other := commands.NewStore(d.Store().Path())
			if err := other.Load(); err != nil {
				t.Fatalf("loading from the other store: %v", err)
			}
			other.Add("from the agent", "written outside the TUI")
			if err := other.Save(); err != nil {
				t.Fatalf("saving from the other store: %v", err)
			}

			d.Reload()

			// The new todo is there and the cursor stayed on "two".
			d.Todos().Lines(
				Contains("one"),
				Contains("two").IsSelected(),
				Contains("from the agent"),
			)

			// Its description comes through too.
			d.Press(d.Keys.Todos.Bottom)
			d.Detail().Lines(Equals("written outside the TUI"))
		},
	})
}

// TestInstallAgentKeyWritesTheExtension covers the keystroke that copies the
// embedded integration into the agent's extension folder.
func TestInstallAgentKeyWritesTheExtension(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	RunTest(t, TestCase{
		Setup: func(shell *Shell) { shell.Add("one") },
		Run: func(d *TestDriver) {
			d.Press(d.Keys.Universal.InstallAgent)

			path := filepath.Join(home, ".pi", "agent", "extensions", "lazytodo.ts")
			if _, err := os.Stat(path); err != nil {
				t.Fatalf("expected the extension at %s: %v", path, err)
			}
			// The status bar clips the long temp path, so match the verb.
			d.Status().Lines(Contains("Installed"))

			// A second press reports that nothing changed.
			d.Press(d.Keys.Universal.InstallAgent)
			d.Status().Lines(Contains("up to date"))
		},
	})
}

func TestFilterTabsSitOnTheTitleRow(t *testing.T) {
	RunTest(t, TestCase{
		Setup: func(shell *Shell) { shell.Add("one") },
		Run: func(d *TestDriver) {
			border := d.Todos().TopBorder()
			for _, want := range []string{"Todos", "All", "Doing", "Done"} {
				if !strings.Contains(border, want) {
					t.Errorf("top border %q is missing %q", border, want)
				}
			}
		},
	})
}

func TestClickingATabNarrowsTheList(t *testing.T) {
	RunTest(t, TestCase{
		Setup: func(shell *Shell) {
			shell.Add("one", "two", "three")
			shell.Cycle(2) // two -> doing
		},
		Run: func(d *TestDriver) {
			d.Click(d.Todos().TabCenter("Doing"))

			d.Todos().Lines(Contains("two").IsSelected())
			d.Status().Lines(Equals("0/3 done · 1 doing · only Doing"))

			d.Click(d.Todos().TabCenter("Done"))
			d.Todos().Lines()
			d.Status().Lines(Equals("0/3 done · 1 doing · only Done"))

			d.Click(d.Todos().TabCenter("All"))
			d.Todos().Lines(Contains("one"), Contains("two"), Contains("three"))
			d.Status().Lines(Equals("0/3 done · 1 doing"))
		},
	})
}

func TestTabCyclesTheStatusFilter(t *testing.T) {
	RunTest(t, TestCase{
		Setup: func(shell *Shell) {
			shell.Add("one", "two", "three")
			shell.Cycle(2) // two -> doing
		},
		Run: func(d *TestDriver) {
			d.Press(d.Keys.Todos.NextStatusFilter) // All -> Doing
			d.Todos().Lines(Contains("two").IsSelected())
			d.Status().Lines(Equals("0/3 done · 1 doing · only Doing"))

			d.Press(d.Keys.Todos.NextStatusFilter) // Doing -> Done
			d.Todos().Lines()

			d.Press(d.Keys.Todos.NextStatusFilter) // Done -> All, wraps
			d.Todos().Lines(Contains("one"), Contains("two"), Contains("three"))
			d.Status().Lines(Equals("0/3 done · 1 doing"))

			d.Press(d.Keys.Todos.PrevStatusFilter) // back to Done
			d.Status().Lines(Equals("0/3 done · 1 doing · only Done"))
		},
	})
}

func TestClickingARowSelectsIt(t *testing.T) {
	RunTest(t, TestCase{
		Setup: func(shell *Shell) { shell.Add("one", "two", "three") },
		Run: func(d *TestDriver) {
			d.Click(d.Todos().ContentPoint(2))
			if got := d.Todos().Cursor(); got != 2 {
				t.Fatalf("cursor after clicking row 3 = %d, want 2", got)
			}

			d.Click(d.Todos().ContentPoint(0))
			if got := d.Todos().Cursor(); got != 0 {
				t.Fatalf("cursor after clicking row 1 = %d, want 0", got)
			}

			// Clicking past the last todo is a miss, not a crash.
			d.Click(d.Todos().ContentPoint(8))
			if got := d.Todos().Cursor(); got != 0 {
				t.Errorf("cursor after clicking empty space = %d, want 0", got)
			}
		},
	})
}

func TestEscClearsBothFilters(t *testing.T) {
	RunTest(t, TestCase{
		Setup: func(shell *Shell) { shell.Add("one", "two", "three") },
		Run: func(d *TestDriver) {
			d.Press(d.Keys.Todos.Filter).Type("two").Press(tui.KeyEnter)
			d.Click(d.Todos().TabCenter("Doing")) // "two" is a todo, so nothing matches
			d.Todos().Lines()

			d.Press(d.Keys.Todos.ClearFilter)

			d.Todos().Lines(Contains("one"), Contains("two"), Contains("three"))
			d.Status().Lines(Equals("0/3 done"))
		},
	})
}

// TestChineseTitlesSurviveTheForm also guards the key pipeline for multi-byte
// characters, and the width maths that keeps the row from overflowing.
func TestChineseTitlesSurviveTheForm(t *testing.T) {
	RunTest(t, TestCase{
		Run: func(d *TestDriver) {
			d.Press(d.Keys.Todos.Add)
			d.Type("写存储层")
			d.Press(tui.KeyEnter)

			d.Todos().Lines(Contains("写存储层").IsSelected())
			if got := d.Store().Todos[0].Title; got != "写存储层" {
				t.Errorf("title = %q, want %q", got, "写存储层")
			}
		},
	})
}

func TestLongChineseTitleIsTruncatedToThePane(t *testing.T) {
	RunTest(t, TestCase{
		Setup: func(shell *Shell) {
			shell.Add("这是一个非常长的中文待办标题用来确认它会按格子宽度截断")
		},
		Size: [2]int{46, 10},
		Run: func(d *TestDriver) {
			lines := d.Todos().Content()
			if len(lines) != 1 {
				t.Fatalf("got %d lines, want 1", len(lines))
			}
			if !strings.HasSuffix(lines[0], "…") {
				t.Errorf("line %q was not truncated", lines[0])
			}

			x0, _, x1, _ := d.Todos().Region()
			if got, limit := tui.DisplayWidth(lines[0]), x1-x0+1; got > limit {
				t.Errorf("line is %d cells wide, pane holds %d", got, limit)
			}
		},
	})
}

func TestClicksAreIgnoredWhileTheFormIsOpen(t *testing.T) {
	RunTest(t, TestCase{
		Setup: func(shell *Shell) { shell.Add("one") },
		Run: func(d *TestDriver) {
			d.Press(d.Keys.Todos.Add)

			d.Click(d.Todos().TabCenter("Done"))

			// Still the form on screen, and escaping shows an unfiltered list.
			d.Popup().Includes("▸ Title")
			d.Press(tui.KeyEsc)
			d.Todos().Lines(Contains("one"))
		},
	})
}

func TestNarrowTerminalKeepsOnlyTheList(t *testing.T) {
	RunTest(t, TestCase{
		Setup: func(shell *Shell) { shell.Add("one") },
		Size:  [2]int{30, 10},
		Run: func(d *TestDriver) {
			d.Click(d.Todos().TabCenter("Done"))
			d.Todos().Lines()

			d.Click(d.Todos().TabCenter("All"))
			d.Todos().Lines(Contains("one"))
		},
	})
}

// ---------------------------------------------------------------------------
// the add/edit form
// ---------------------------------------------------------------------------

func TestEnterInTheTitleFieldSubmits(t *testing.T) {
	RunTest(t, TestCase{
		Run: func(d *TestDriver) {
			d.Press(d.Keys.Todos.Add)
			d.Popup().Includes("▸ Title", "Description")

			d.Type("buy milk").Press(tui.KeyEnter)

			d.Todos().Lines(Contains("buy milk").IsSelected())
			d.Status().Lines(Equals("0/1 done"))
			if got := d.Store().Todos[0].Detail; got != "" {
				t.Errorf("detail = %q, want empty", got)
			}
		},
	})
}

func TestDescriptionAcceptsMultipleLines(t *testing.T) {
	RunTest(t, TestCase{
		Run: func(d *TestDriver) {
			d.Press(d.Keys.Todos.Add)
			d.Type("buy milk")

			// Tab moves to the description, where Enter inserts a newline and
			// Ctrl+S submits.
			d.Press(tui.KeyTab)
			d.Popup().Includes("▸ Description")

			d.Type("get oat milk").Press(tui.KeyEnter)
			d.Type("2 litres")
			d.Popup().Includes("  get oat milk", "  2 litres█")

			d.Press(tui.KeyCtrlS)

			d.Todos().Lines(Contains("buy milk").IsSelected())
			d.Detail().Lines(Equals("get oat milk"), Equals("2 litres"))

			if got, want := d.Store().Todos[0].Detail, "get oat milk\n2 litres"; got != want {
				t.Errorf("stored detail = %q, want %q", got, want)
			}
		},
	})
}

func TestAddingWhileFilteredClearsTheFilters(t *testing.T) {
	RunTest(t, TestCase{
		Setup: func(shell *Shell) { shell.Add("write the store") },
		Run: func(d *TestDriver) {
			// Filter to done todos, then add one: it would be hidden, so the
			// filters are dropped rather than leaving an unchanged list.
			d.Click(d.Todos().TabCenter("Done"))
			d.Todos().Lines()

			d.Press(d.Keys.Todos.Add).Type("brand new").Press(tui.KeyEnter)

			d.Todos().Lines(Contains("write the store"), Contains("brand new"))
		},
	})
}

func TestBlankTitleIsRejectedInPlace(t *testing.T) {
	RunTest(t, TestCase{
		Run: func(d *TestDriver) {
			d.Press(d.Keys.Todos.Add)
			d.Press(tui.KeyEnter)

			if got := d.Popup().Footer(); got != "title is required" {
				t.Errorf("footer = %q, want %q", got, "title is required")
			}
			d.Popup().Includes("▸ Title")
			if got := len(d.Store().Todos); got != 0 {
				t.Errorf("store has %d todos, want 0", got)
			}

			d.Type("now it has a title")
			if got := d.Popup().Footer(); got == "title is required" {
				t.Error("footer still shows the stale error")
			}
		},
	})
}

func TestCancellingTheFormChangesNothing(t *testing.T) {
	RunTest(t, TestCase{
		Setup: func(shell *Shell) { shell.Add("one") },
		Run: func(d *TestDriver) {
			d.Press(d.Keys.Todos.Add).Type("nope").Press(tui.KeyEsc)

			d.Todos().Lines(Contains("one"))
			if got := len(d.Store().Todos); got != 1 {
				t.Errorf("store has %d todos, want 1", got)
			}
		},
	})
}

func TestEditingPrefillsTitleAndDescription(t *testing.T) {
	RunTest(t, TestCase{
		Setup: func(shell *Shell) { shell.AddWithDetail("typo here", "keep me") },
		Run: func(d *TestDriver) {
			d.Press(d.Keys.Todos.Edit)
			d.Popup().Includes("▸ Title", "typo here█", "keep me")

			// Enter submits from the title field, and the untouched description
			// survives.
			d.Type(" fixed").Press(tui.KeyEnter)

			d.Todos().Lines(Contains("typo here fixed").IsSelected())
			if got := d.Store().Todos[0].Detail; got != "keep me" {
				t.Errorf("detail = %q, want %q", got, "keep me")
			}
		},
	})
}

// ---------------------------------------------------------------------------
// detail pane
// ---------------------------------------------------------------------------

func TestDetailPaneFollowsTheSelection(t *testing.T) {
	RunTest(t, TestCase{
		Setup: func(shell *Shell) {
			shell.AddWithDetail("write the gui", "paste the skeleton\ninto pkg/gui")
			shell.Add("walk the dog")
		},
		Run: func(d *TestDriver) {
			d.Detail().Lines(Equals("paste the skeleton"), Equals("into pkg/gui"))

			d.Press(d.Keys.Todos.Next)
			d.Detail().Lines(Equals("(no description)"))
		},
	})
}

func TestDetailLineCanBeCopied(t *testing.T) {
	RunTest(t, TestCase{
		Setup: func(shell *Shell) {
			shell.AddWithDetail("write the gui", "first line\nsecond line\nthird line")
		},
		Run: func(d *TestDriver) {
			d.Press(d.Keys.Todos.FocusDetail)
			d.Press(d.Keys.Detail.Next) // first -> second

			d.Press(d.Keys.Detail.Copy)

			if got, want := d.Clipboard(), "second line"; got != want {
				t.Fatalf("clipboard = %q, want %q", got, want)
			}
			d.Status().Lines(Equals(`copied "second line"`))
			d.Options().Lines(Equals("[y] Copy [h] Back"))
		},
	})
}

func TestDetailCopyIsDisabledWithoutADescription(t *testing.T) {
	RunTest(t, TestCase{
		Setup: func(shell *Shell) { shell.Add("no detail") },
		Run: func(d *TestDriver) {
			d.Press(d.Keys.Todos.FocusDetail)
			d.Press(d.Keys.Detail.Copy)

			d.Status().Lines(Equals("Nothing to copy"))
			if got := d.Clipboard(); got != "" {
				t.Errorf("clipboard = %q, want nothing copied", got)
			}
		},
	})
}

func TestDetailCursorClampsToTheDescription(t *testing.T) {
	RunTest(t, TestCase{
		Setup: func(shell *Shell) { shell.AddWithDetail("one", "a\nb") },
		Run: func(d *TestDriver) {
			d.Press(d.Keys.Todos.FocusDetail)
			d.Press(d.Keys.Detail.Next, d.Keys.Detail.Next, d.Keys.Detail.Next)

			d.Press(d.Keys.Detail.Copy)
			if got, want := d.Clipboard(), "b"; got != want {
				t.Fatalf("clipboard = %q, want the last line %q", got, want)
			}
		},
	})
}

func TestDetailGivesFocusBackToTheList(t *testing.T) {
	RunTest(t, TestCase{
		Setup: func(shell *Shell) { shell.Add("one") },
		Run: func(d *TestDriver) {
			d.Press(d.Keys.Todos.FocusDetail)
			d.Press(d.Keys.Detail.FocusList)

			// The list's bindings work again.
			d.Press(d.Keys.Todos.CycleStatus)
			if got := d.Store().Todos[0].Status; got != models.StatusDoing {
				t.Errorf("status = %q, want %q: focus never came back to the list", got, models.StatusDoing)
			}
		},
	})
}

// ---------------------------------------------------------------------------
// the rest
// ---------------------------------------------------------------------------

func TestDeletingAsksFirst(t *testing.T) {
	RunTest(t, TestCase{
		Setup: func(shell *Shell) { shell.Add("one", "two", "three") },
		Run: func(d *TestDriver) {
			d.Press(d.Keys.Todos.Next)
			d.Press(d.Keys.Todos.Delete)

			// The question is on screen and nothing has been destroyed yet.
			d.Popup().Includes(`Delete "two"?`)
			if got, want := d.Popup().Footer(), "[y] yes  [n/esc] cancel"; got != want {
				t.Errorf("footer = %q, want %q", got, want)
			}
			if got := len(d.Store().Todos); got != 3 {
				t.Fatalf("store has %d todos before confirming, want 3", got)
			}

			d.Press(tui.Key("y"))

			d.Todos().Lines(Contains("one"), Contains("three").IsSelected())
			if got := len(d.Store().Todos); got != 2 {
				t.Fatalf("store has %d todos after confirming, want 2", got)
			}
		},
	})
}

func TestCancellingTheDeleteKeepsTheTodo(t *testing.T) {
	for _, cancel := range []tui.Key{tui.Key("n"), tui.KeyEsc} {
		t.Run(cancel.Label(), func(t *testing.T) {
			RunTest(t, TestCase{
				Setup: func(shell *Shell) { shell.Add("one", "two") },
				Run: func(d *TestDriver) {
					d.Press(d.Keys.Todos.Delete)
					d.Press(cancel)

					d.Todos().Lines(Contains("one"), Contains("two"))
					if got := len(d.Store().Todos); got != 2 {
						t.Fatalf("store has %d todos, want 2", got)
					}
				},
			})
		})
	}
}

// TestConfirmIgnoresStrayKeys pins that a destructive question can't be answered
// by accident: Enter, j, arrows, space and Tab all do nothing. Only y deletes.
func TestConfirmIgnoresStrayKeys(t *testing.T) {
	RunTest(t, TestCase{
		Setup: func(shell *Shell) { shell.Add("one", "two") },
		Run: func(d *TestDriver) {
			d.Press(d.Keys.Todos.Delete)

			d.Press(tui.KeyEnter, tui.Key("j"), tui.KeyDown, tui.KeySpace, tui.KeyTab)

			d.Popup().Includes("Delete")
			if got := len(d.Store().Todos); got != 2 {
				t.Fatalf("store has %d todos, want the question to still be open", got)
			}

			d.Press(tui.Key("y"))
			if got := len(d.Store().Todos); got != 1 {
				t.Fatalf("store has %d todos, want 1 after y", got)
			}
		},
	})
}

func TestFilteringNarrowsAndClearingRestores(t *testing.T) {
	RunTest(t, TestCase{
		Setup: func(shell *Shell) { shell.Add("write the store", "write the gui", "walk the dog") },
		Run: func(d *TestDriver) {
			d.Press(d.Keys.Todos.Filter).Type("gui").Press(tui.KeyEnter)

			d.Todos().Lines(Contains("write the gui").IsSelected())
			d.Status().Lines(Equals("0/1 done · filter: \"gui\""))

			d.Press(d.Keys.Todos.ClearFilter)

			d.Todos().Lines(
				Contains("write the store").IsSelected(),
				Contains("write the gui"),
				Contains("walk the dog"),
			)
			d.Status().Lines(Equals("0/3 done"))
		},
	})
}

func TestFilterMatchesTheDescriptionToo(t *testing.T) {
	RunTest(t, TestCase{
		Setup: func(shell *Shell) {
			shell.AddWithDetail("write the store", "atomic rename")
			shell.Add("walk the dog")
		},
		Run: func(d *TestDriver) {
			d.Press(d.Keys.Todos.Filter).Type("atomic").Press(tui.KeyEnter)

			d.Todos().Lines(Contains("write the store").IsSelected())
		},
	})
}

func TestDisabledBindingsExplainThemselves(t *testing.T) {
	RunTest(t, TestCase{
		Run: func(d *TestDriver) {
			// Nothing is selected, so the cycle binding is disabled and the
			// status bar says why instead of doing nothing silently.
			d.Press(d.Keys.Todos.CycleStatus)
			d.Status().Lines(Equals("No todo selected"))

			// The options bar only advertises runnable bindings.
			d.Options().Lines(Equals("[a] Add [/] Filter"))
		},
	})
}

func TestOptionsBarAdvertisesRunnableBindings(t *testing.T) {
	RunTest(t, TestCase{
		Setup: func(shell *Shell) { shell.Add("one") },
		Run: func(d *TestDriver) {
			d.Options().Lines(Equals("[␣] Cycle [a] Add [e] Edit [d] Delete [/] Filter"))
		},
	})
}

func TestSmallTerminalDegradesInsteadOfPanicking(t *testing.T) {
	RunTest(t, TestCase{
		Setup: func(shell *Shell) { shell.Add("only one") },
		Size:  [2]int{8, 2},
		Run: func(d *TestDriver) {
			d.Press(d.Keys.Todos.CycleStatus)
			if got := d.Store().Todos[0].Status; got != models.StatusDoing {
				t.Errorf("status = %q, want %q", got, models.StatusDoing)
			}
		},
	})
}

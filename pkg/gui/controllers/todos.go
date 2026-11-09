package controllers

import (
	"errors"
	"fmt"
	"strings"

	"github.com/shen/lazytodo/pkg/gui/presentation"
	"github.com/shen/lazytodo/pkg/gui/types"
	"github.com/shen/lazytodo/pkg/models"
	"github.com/shen/lazytodo/pkg/tui"
)

// Field positions in the todo form. Kept as constants because the popup hands
// values back positionally.
const (
	titleField = iota
	detailField
)

// errTitleRequired is returned by the form handlers when the title is blank. The
// GUI shows it in the popup footer and keeps the popup open.
var errTitleRequired = errors.New("title is required")

// TodosController owns the todo pane: moving the cursor, the filter tabs on its
// top border, and the actions that change the list. Every mutation goes
// store -> Save -> Refresh, so the screen is always a pure function of the file
// on disk.
type TodosController struct {
	c *Common
}

var (
	_ types.IController = &TodosController{}
	_ types.IRenderHook = &TodosController{}
	_ types.IClickHook  = &TodosController{}
)

// NewTodosController builds the todo pane controller.
func NewTodosController(c *Common) *TodosController {
	return &TodosController{c: c}
}

// GetKeybindings maps the configured keys to the actions. The
// GetDisabledReason fields are what keep the handlers simple: when a todo isn't
// selected the GUI shows the reason instead of calling the handler at all.
func (self *TodosController) GetKeybindings() []*types.Binding {
	keys := self.c.Config.Keybinding.Todos

	return []*types.Binding{
		{
			Keys:        []tui.Key{keys.Next, tui.KeyDown},
			Handler:     func() error { return self.move(1) },
			Description: "Next",
		},
		{
			Keys:        []tui.Key{keys.Prev, tui.KeyUp},
			Handler:     func() error { return self.move(-1) },
			Description: "Prev",
		},
		{
			Keys:        []tui.Key{keys.Top},
			Handler:     self.selectFirst,
			Description: "Top",
		},
		{
			Keys:        []tui.Key{keys.Bottom},
			Handler:     self.selectLast,
			Description: "Bottom",
		},
		{
			Keys:              []tui.Key{keys.CycleStatus},
			Handler:           self.cycleStatus,
			Description:       self.c.Tr.Cycle,
			Tooltip:           "todo -> doing -> done -> todo",
			GetDisabledReason: self.requireItem,
			DisplayOnScreen:   true,
		},
		{
			Keys:            []tui.Key{keys.Add},
			Handler:         self.add,
			Description:     self.c.Tr.Add,
			Tooltip:         "Add a todo with a title and a description",
			DisplayOnScreen: true,
		},
		{
			Keys:              []tui.Key{keys.Edit},
			Handler:           self.edit,
			Description:       self.c.Tr.Edit,
			Tooltip:           "Edit the title and description",
			GetDisabledReason: self.requireItem,
			DisplayOnScreen:   true,
		},
		{
			Keys:              []tui.Key{keys.Delete},
			Handler:           self.delete,
			Description:       self.c.Tr.Delete,
			GetDisabledReason: self.requireItem,
			DisplayOnScreen:   true,
		},
		{
			Keys:              []tui.Key{keys.Filter},
			Handler:           self.filter,
			Description:       self.c.Tr.Filter,
			Tooltip:           "Show only matching todos",
			DisplayOnScreen:   true,
			GetDisabledReason: nil,
		},
		{
			Keys:              []tui.Key{keys.ClearFilter},
			Handler:           self.clearFilter,
			Description:       self.c.Tr.ClearFilter,
			GetDisabledReason: self.requireFilter,
		},
		{
			Keys:        []tui.Key{keys.FocusDetail},
			Handler:     self.focusDetail,
			Description: "Detail",
			Tooltip:     "Move the cursor into the detail pane",
		},
		{
			Keys:        []tui.Key{keys.NextStatusFilter},
			Handler:     func() error { return self.cycleStatusFilter(1) },
			Description: self.c.Tr.NextFilter,
			Tooltip:     "Cycle the status filter: All -> Doing -> Done",
		},
		{
			Keys:        []tui.Key{keys.PrevStatusFilter},
			Handler:     func() error { return self.cycleStatusFilter(-1) },
			Description: self.c.Tr.PrevFilter,
			Tooltip:     "Cycle the status filter backwards",
		},
	}
}

// GetOnRender is the pull half of refresh: it rebuilds the pane from
// g.State().Todos, puts the filter tabs on the top border, and tells the view
// which row is selected.
func (self *TodosController) GetOnRender() func() {
	return func() {
		state := self.c.Gui.State()
		view := self.c.Gui.Views().Todos

		lines, styles := presentation.TodoLines(state.Todos, view.InnerWidth())
		view.SetContent(lines)
		view.SetLineStyles(styles)
		view.SetSelected(state.SelectedIdx)
		view.Tabs = presentation.StatusTabs(state.StatusFilter, self.c.Tr)
	}
}

// GetOnClick handles a click inside the list: it selects the row that was hit.
// Clicks on the filter tabs never get here -- the GUI claims those first,
// because the tabs are part of the view's border, not its content.
func (self *TodosController) GetOnClick() func(x, y int) error {
	return func(_, y int) error {
		row, ok := self.c.Gui.Views().Todos.RowAt(y)
		if !ok {
			return nil
		}

		self.c.Gui.State().SelectedIdx = row
		self.c.Gui.Refresh()
		return nil
	}
}

func (self *TodosController) move(delta int) error {
	self.c.Gui.State().SelectedIdx += delta
	self.c.Gui.Refresh()
	return nil
}

// selectLast goes to the end of the list; Refresh clamps -1 to 0 when empty.
func (self *TodosController) selectLast() error {
	self.c.Gui.State().SelectedIdx = len(self.c.Gui.State().Todos) - 1
	self.c.Gui.Refresh()
	return nil
}

func (self *TodosController) focusDetail() error {
	self.c.Gui.Focus(types.DETAIL_CONTEXT_KEY)
	return nil
}

func (self *TodosController) selectFirst() error {
	self.c.Gui.State().SelectedIdx = 0
	self.c.Gui.Refresh()
	return nil
}

func (self *TodosController) cycleStatus() error {
	todo, ok := self.selected()
	if !ok {
		return nil
	}
	if err := self.c.Gui.Store().CycleStatus(todo.ID); err != nil {
		return err
	}
	return self.persist()
}

// add opens the two-field form. Enter in the title field submits, which is what
// makes "type a title, hit Enter" the fast path; the description keeps newlines
// and is submitted with Ctrl+S.
func (self *TodosController) add() error {
	self.c.Gui.Popup(self.c.Tr.NewTodoTitle, []types.Field{
		{Label: self.c.Tr.FieldTitle},
		{Label: self.c.Tr.FieldDetail, Multiline: true},
	}, func(values []string) error {
		title := cleanTitle(values[titleField])
		if title == "" {
			return errTitleRequired
		}

		store := self.c.Gui.Store()
		todo := store.Add(title, cleanDetail(values[detailField]))
		if err := store.Save(); err != nil {
			return err
		}

		// Make sure what we just added is visible: if the active filters would
		// hide it, drop them rather than leaving the user staring at an
		// unchanged list.
		state := self.c.Gui.State()
		if !todo.Matches(state.Filter) {
			state.Filter = ""
		}
		if state.StatusFilter != models.StatusAll && todo.Status != state.StatusFilter {
			state.StatusFilter = models.StatusAll
		}
		state.SelectedIdx = len(store.Filter(state.Filter, state.StatusFilter)) - 1
		self.c.Gui.Refresh()
		return nil
	})
	return nil
}

// edit reuses the add form, prefilled with the selected todo.
func (self *TodosController) edit() error {
	todo, ok := self.selected()
	if !ok {
		return nil
	}

	self.c.Gui.Popup(self.c.Tr.EditTodoTitle, []types.Field{
		{Label: self.c.Tr.FieldTitle, Text: todo.Title},
		{Label: self.c.Tr.FieldDetail, Text: todo.Detail, Multiline: true},
	}, func(values []string) error {
		title := cleanTitle(values[titleField])
		if title == "" {
			return errTitleRequired
		}

		store := self.c.Gui.Store()
		if err := store.SetTitle(todo.ID, title); err != nil {
			return err
		}
		if err := store.SetDetail(todo.ID, cleanDetail(values[detailField])); err != nil {
			return err
		}
		return self.persist()
	})
	return nil
}

// delete asks first. Deleting is the one action here that destroys data with no
// way back, so it goes through a yes/no modal instead of acting on the keypress.
func (self *TodosController) delete() error {
	todo, ok := self.selected()
	if !ok {
		return nil
	}

	self.c.Gui.Confirm(self.c.Tr.ConfirmTitle, fmt.Sprintf(self.c.Tr.ConfirmDelete, todo.Title), func() error {
		if err := self.c.Gui.Store().Delete(todo.ID); err != nil {
			return err
		}
		return self.persist()
	})
	return nil
}

func (self *TodosController) filter() error {
	self.c.Gui.Popup(self.c.Tr.FilterTitle, []types.Field{
		{Label: self.c.Tr.FieldMatch, Text: self.c.Gui.State().Filter},
	}, func(values []string) error {
		state := self.c.Gui.State()
		state.Filter = cleanTitle(values[0])
		state.SelectedIdx = 0
		self.c.Gui.Refresh()
		return nil
	})
	return nil
}

// cycleStatusFilter walks the All / Doing / Done tabs from the keyboard, so the
// filter isn't mouse-only. It wraps around at both ends.
func (self *TodosController) cycleStatusFilter(delta int) error {
	tabCount := presentation.StatusTabCount()
	if tabCount == 0 {
		return nil
	}

	current := presentation.StatusTabIndex(self.c.Gui.State().StatusFilter)
	next := (current + delta + tabCount) % tabCount

	self.c.Gui.SetStatusFilter(presentation.StatusAtTab(next))
	return nil
}

func (self *TodosController) clearFilter() error {
	self.c.Gui.ClearFilters()
	return nil
}

// persist writes the store and refreshes the screen. This is the tail end of
// every mutation; keeping it in one place is why the handlers above are three
// lines each.
func (self *TodosController) persist() error {
	if err := self.c.Gui.Store().Save(); err != nil {
		return err
	}
	self.c.Gui.Refresh()
	return nil
}

func (self *TodosController) selected() (*models.Todo, bool) {
	state := self.c.Gui.State()
	if state.SelectedIdx < 0 || state.SelectedIdx >= len(state.Todos) {
		return nil, false
	}
	return state.Todos[state.SelectedIdx], true
}

func (self *TodosController) requireItem() *types.DisabledReason {
	if _, ok := self.selected(); !ok {
		return &types.DisabledReason{Text: self.c.Tr.NoTodoSelected}
	}
	return nil
}

func (self *TodosController) requireFilter() *types.DisabledReason {
	state := self.c.Gui.State()
	if state.Filter == "" && state.StatusFilter == models.StatusAll {
		return &types.DisabledReason{Text: self.c.Tr.NoFilter}
	}
	return nil
}

// cleanTitle trims a one-line value on both ends.
func cleanTitle(value string) string {
	return strings.TrimSpace(value)
}

// cleanDetail keeps a description's leading indentation but drops the trailing
// blank lines that pressing Enter before submitting tends to leave behind.
func cleanDetail(value string) string {
	return strings.TrimRight(value, " \t\n")
}

// ---------------------------------------------------------------------------
// detail pane
// ---------------------------------------------------------------------------

// DetailController owns the detail pane. It is read-only: the pane exists so a
// line of the description can be picked out and copied, so nothing here mutates
// the store.
type DetailController struct {
	c *Common
}

var (
	_ types.IController = &DetailController{}
	_ types.IRenderHook = &DetailController{}
	_ types.IClickHook  = &DetailController{}
)

// NewDetailController builds the detail pane controller.
func NewDetailController(c *Common) *DetailController {
	return &DetailController{c: c}
}

// GetKeybindings maps the configured keys (plus the arrows) to moving the
// read-only cursor and copying the line under it.
func (self *DetailController) GetKeybindings() []*types.Binding {
	keys := self.c.Config.Keybinding.Detail

	return []*types.Binding{
		{
			Keys:        []tui.Key{keys.Next, tui.KeyDown},
			Handler:     func() error { return self.move(1) },
			Description: "Next",
		},
		{
			Keys:        []tui.Key{keys.Prev, tui.KeyUp},
			Handler:     func() error { return self.move(-1) },
			Description: "Prev",
		},
		{
			Keys:        []tui.Key{keys.Top},
			Handler:     self.selectFirst,
			Description: "Top",
		},
		{
			Keys:        []tui.Key{keys.Bottom},
			Handler:     self.selectLast,
			Description: "Bottom",
		},
		{
			Keys:              []tui.Key{keys.Copy},
			Handler:           self.copy,
			Description:       self.c.Tr.Copy,
			Tooltip:           "Copy the selected line to the clipboard",
			GetDisabledReason: self.requireCopyable,
			DisplayOnScreen:   true,
		},
		{
			Keys:            []tui.Key{keys.FocusList},
			Handler:         self.focusList,
			Description:     self.c.Tr.Back,
			Tooltip:         "Move the cursor back to the list",
			DisplayOnScreen: true,
		},
	}
}

// GetOnRender draws the description and parks the cursor on the right row. The
// view clamps the cursor, so the clamped value is written back to the model.
func (self *DetailController) GetOnRender() func() {
	return func() {
		state := self.c.Gui.State()
		view := self.c.Gui.Views().Detail

		if !view.Visible {
			view.SetContent(nil)
			return
		}

		view.SetContent(presentation.DetailLines(self.todo(), view.InnerWidth(), self.c.Tr))
		view.SetSelected(state.DetailLine)
		state.DetailLine = view.SelectedLine
	}
}

// GetOnClick selects the clicked row.
func (self *DetailController) GetOnClick() func(x, y int) error {
	return func(_, y int) error {
		row, ok := self.c.Gui.Views().Detail.RowAt(y)
		if !ok {
			return nil
		}

		self.c.Gui.State().DetailLine = row
		self.c.Gui.Refresh()
		return nil
	}
}

func (self *DetailController) todo() *models.Todo {
	state := self.c.Gui.State()
	if state.SelectedIdx < 0 || state.SelectedIdx >= len(state.Todos) {
		return nil
	}
	return state.Todos[state.SelectedIdx]
}

func (self *DetailController) move(delta int) error {
	self.c.Gui.State().DetailLine += delta
	self.c.Gui.Refresh()
	return nil
}

func (self *DetailController) selectFirst() error {
	self.c.Gui.State().DetailLine = 0
	self.c.Gui.Refresh()
	return nil
}

func (self *DetailController) selectLast() error {
	self.c.Gui.State().DetailLine = len(presentation.DetailSourceLines(self.todo())) - 1
	self.c.Gui.Refresh()
	return nil
}

func (self *DetailController) focusList() error {
	self.c.Gui.Focus(types.TODOS_CONTEXT_KEY)
	return nil
}

// copy puts the whole selected line on the clipboard -- the untruncated one, so
// a narrow pane doesn't copy the ellipsis it had to add.
func (self *DetailController) copy() error {
	line, ok := self.selectedLine()
	if !ok {
		return nil
	}

	if err := self.c.Gui.Copy(line); err != nil {
		return err
	}

	self.c.Gui.ReportInfo(fmt.Sprintf(self.c.Tr.Copied, line))
	return nil
}

func (self *DetailController) selectedLine() (string, bool) {
	source := presentation.DetailSourceLines(self.todo())

	line := self.c.Gui.State().DetailLine
	if line < 0 || line >= len(source) {
		return "", false
	}
	return source[line], true
}

func (self *DetailController) requireCopyable() *types.DisabledReason {
	if _, ok := self.selectedLine(); !ok {
		return &types.DisabledReason{Text: self.c.Tr.NothingToCopy}
	}
	return nil
}

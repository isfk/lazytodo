// Package types holds the vocabulary shared by the GUI, its contexts and its
// controllers. It exists so pkg/gui/controllers can talk to the GUI without
// importing it -- the same job lazygit's own pkg/gui/types does.
package types

import (
	"github.com/isfk/lazytodo/pkg/commands"
	"github.com/isfk/lazytodo/pkg/config"
	"github.com/isfk/lazytodo/pkg/i18n"
	"github.com/isfk/lazytodo/pkg/models"
	"github.com/isfk/lazytodo/pkg/tui"
)

// ContextKey identifies one pane. String constants keep bindings and logs
// readable.
type ContextKey string

const (
	GLOBAL_CONTEXT_KEY  ContextKey = "global"
	STATUS_CONTEXT_KEY  ContextKey = "status"
	TODOS_CONTEXT_KEY   ContextKey = "todos"
	DETAIL_CONTEXT_KEY  ContextKey = "detail"
	OPTIONS_CONTEXT_KEY ContextKey = "options"
	POPUP_CONTEXT_KEY   ContextKey = "popup"
)

// DisabledReason is the answer to "why can't this run right now?". Returning a
// non-nil reason from a Binding disables it and shows the text in the status
// bar, which keeps every handler free of guard clauses.
type DisabledReason struct {
	Text string
}

// Binding is one key (or set of keys) mapped to an action.
type Binding struct {
	Keys        []tui.Key
	Handler     func() error
	Description string
	Tooltip     string

	// GetDisabledReason is optional. When nil the binding is always enabled.
	GetDisabledReason func() *DisabledReason

	// DisplayOnScreen asks the options bar to advertise this binding.
	DisplayOnScreen bool
}

// Match reports whether the binding accepts the given key.
func (b *Binding) Match(k tui.Key) bool {
	for _, key := range b.Keys {
		if key == k {
			return true
		}
	}
	return false
}

// Disabled reports why the binding can't run right now, or nil if it can. The
// GUI consults this before calling the handler, which is what lets handlers
// assume their preconditions hold.
func (b *Binding) Disabled() *DisabledReason {
	if b.GetDisabledReason == nil {
		return nil
	}
	return b.GetDisabledReason()
}

// Context is one pane: a view plus the behaviour around it. The GUI asks a
// context for its keybindings and its render hook; the context never drives the
// screen itself.
type Context interface {
	GetKey() ContextKey
	GetView() *tui.View
	IsFocusable() bool

	GetKeybindings() []*Binding
	HandleRender()
	HandleFocus()
	HandleFocusLost()

	// HandleClick is offered a click that landed inside this context's view.
	HandleClick(x, y int) error
}

// Controllers are attached through these optional interfaces rather than a fat
// base struct: a controller implements only the hooks it cares about, and the
// attachment step type-asserts for the rest. Adding a hook means adding an
// interface here, never touching existing controllers.
type IController interface {
	GetKeybindings() []*Binding
}

type IFocusHook interface {
	GetOnFocus() func()
}

type IFocusLostHook interface {
	GetOnFocusLost() func()
}

type IRenderHook interface {
	GetOnRender() func()
}

// IClickHook lets a controller react to a click inside its pane. The GUI decides
// which pane was hit (and claims the filter tabs for itself); the controller
// decides what a click on that pane means for the model.
type IClickHook interface {
	GetOnClick() func(x, y int) error
}

// Views is every pane the GUI owns. Keeping them in one struct means layout,
// rendering and clearing all iterate the same list.
type Views struct {
	Status  *tui.View
	Todos   *tui.View
	Detail  *tui.View
	Options *tui.View
	Popup   *tui.View
}

// All returns the views in paint order. Later views win on overlap, which is
// what makes the popup float above the panes.
func (v Views) All() []*tui.View {
	return []*tui.View{v.Status, v.Todos, v.Detail, v.Options, v.Popup}
}

// Mode is the GUI's input mode. In ModePopup every key goes to the popup before
// any binding is considered, so "a" can mean both "add" and the letter a.
type Mode int

const (
	ModeNormal Mode = iota
	ModePopup
)

// Field is one input inside a popup.
type Field struct {
	Label string
	Text  string
	// Multiline fields keep the newlines Enter produces and are submitted with
	// Ctrl+S instead; single-line fields submit on Enter.
	Multiline bool
}

// PopupState is the open modal panel, if any. The "add todo" form, the "edit"
// form, the filter box and the delete confirmation are all the same mechanism
// with different contents.
type PopupState struct {
	Title  string
	Fields []Field
	Focus  int
	Error  string

	// OnSubmit is the form handler: it gets one value per field and can reject
	// the input by returning an error.
	OnSubmit func(values []string) error

	// Prompt, when set, is body text shown instead of fields. Together with
	// OnConfirm it turns the popup into a yes/no modal.
	Prompt    string
	OnConfirm func() error
}

// IsConfirm reports whether this is a yes/no modal rather than a form.
func (p *PopupState) IsConfirm() bool { return p.OnConfirm != nil }

// Current returns the focused field, or nil when there is none.
func (p *PopupState) Current() *Field {
	if p.Focus < 0 || p.Focus >= len(p.Fields) {
		return nil
	}
	return &p.Fields[p.Focus]
}

// State is the GUI's model: what the panes are showing right now. It is derived
// from the store on every Refresh, never mutated behind the store's back.
type State struct {
	// Todos is the projection of the store after both filters.
	Todos       []*models.Todo
	SelectedIdx int

	// DetailLine is the cursor row in the detail pane.
	DetailLine int

	// Filter is the text filter, StatusFilter the status one. StatusAll means
	// "no status narrowing".
	Filter       string
	StatusFilter models.Status

	// StatusCounts is the tally of the text-filtered list, i.e. the numbers the
	// status pane shows next to each option.
	StatusCounts models.ByStatus

	Mode  Mode
	Popup PopupState

	// Info is a one-shot message shown in the status bar.
	Info string
}

// IGuiCommon is the entire GUI surface controllers are allowed to touch. It is
// deliberately small: if a controller needs something that isn't here, that's a
// signal the code belongs in a helper instead.
type IGuiCommon interface {
	Store() *commands.Store
	State() *State
	Views() Views
	Config() *config.UserConfig

	// Tr is the interface language, resolved once at startup.
	Tr() *i18n.TranslationSet

	// Refresh re-derives State from the store and re-renders every context.
	Refresh()

	// Popup opens a modal with the given fields. Esc cancels; the handler gets
	// one string per field and can reject the input by returning an error, which
	// is shown in the popup's footer while it stays open.
	Popup(title string, fields []Field, onSubmit func(values []string) error)

	// Confirm opens a yes/no modal. Only y (do it) and n/esc (forget it) are
	// accepted, so a destructive action can't be confirmed by muscle memory.
	Confirm(title, prompt string, onConfirm func() error)

	// Focus moves the cursor to another pane.
	Focus(key ContextKey)

	// Copy puts text on the system clipboard through the screen driver.
	Copy(text string) error

	// SetStatusFilter narrows the list to one status (StatusAll clears it).
	// Clicks on the filter tabs and the Tab key both go through here.
	SetStatusFilter(status models.Status)

	// ReportInfo shows a one-shot message; ReportError is the same slot for a
	// failed action.
	ReportInfo(msg string)
	ReportError(err error)

	// ClearFilters drops both the text and the status filter. It lives on the
	// GUI because two panes offer it.
	ClearFilters()

	Quit()
}

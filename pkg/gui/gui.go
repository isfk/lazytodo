// Package gui is the wiring layer: it owns the views, the contexts, the refresh
// cycle and the input loop. Controllers do the work; this package decides who
// gets the keys and when things get re-rendered.
package gui

import (
	"github.com/isfk/lazytodo/pkg/commands"
	"github.com/isfk/lazytodo/pkg/config"
	"github.com/isfk/lazytodo/pkg/gui/context"
	"github.com/isfk/lazytodo/pkg/gui/controllers"
	"github.com/isfk/lazytodo/pkg/gui/presentation"
	"github.com/isfk/lazytodo/pkg/gui/types"
	"github.com/isfk/lazytodo/pkg/i18n"
	"github.com/isfk/lazytodo/pkg/models"
	"github.com/isfk/lazytodo/pkg/tui"
)

const (
	// The list/detail split only happens once there is room for both.
	minTwoPaneWidth = 40
	minListWidth    = 20

	// The popup is a floating box, centred, with a sane maximum width.
	popupMaxWidth = 64
	popupMarginX  = 4
	popupMarginY  = 4
	// Frame rows: one above the content, one below.
	popupFrameRows = 2
)

// Gui is the whole application state that isn't on disk.
type Gui struct {
	screen tui.Screen
	store  *commands.Store
	cfg    *config.UserConfig

	tr *i18n.TranslationSet

	views types.Views
	state types.State

	contexts map[types.ContextKey]types.Context
	focus    types.ContextKey

	quit bool
}

var _ types.IGuiCommon = &Gui{}

// New wires a screen to a store. The caller decides which screen to use, which
// is what makes the headless tests possible.
func New(screen tui.Screen, store *commands.Store, cfg *config.UserConfig) *Gui {
	tr := i18n.For(cfg.Language)
	gui := &Gui{
		screen: screen,
		store:  store,
		cfg:    cfg,
		tr:     tr,
		views: types.Views{
			Status: &tui.View{
				Name:    "status",
				Visible: true,
				Fg:      tui.ColorCyan,
			},
			Todos: &tui.View{
				Name:       "todos",
				Title:      tr.TodosTitle,
				Visible:    true,
				Frame:      true,
				Selectable: true,
				Fg:         tui.ColorDefault,
				Bg:         tui.ColorDefault,
				SelFg:      tui.ColorBlack,
				SelBg:      tui.ColorCyan,
				TitleFg:    tui.ColorGreen,
			},
			Detail: &tui.View{
				Name:       "detail",
				Title:      tr.DetailTitle,
				Visible:    true,
				Frame:      true,
				Selectable: true,
				Fg:         tui.ColorDefault,
				Bg:         tui.ColorDefault,
				SelFg:      tui.ColorBlack,
				SelBg:      tui.ColorCyan,
				TitleFg:    tui.ColorGreen,
			},
			Options: &tui.View{
				Name:    "options",
				Visible: true,
				Fg:      tui.ColorYellow,
			},
			Popup: &tui.View{
				Name:       "popup",
				Visible:    false,
				Frame:      true,
				Selectable: false,
				Fg:         tui.ColorDefault,
				Bg:         tui.ColorDefault,
				SelFg:      tui.ColorDefault,
				SelBg:      tui.ColorDefault,
				TitleFg:    tui.ColorYellow,
				FooterFg:   tui.ColorDefault,
			},
		},
		focus: types.TODOS_CONTEXT_KEY,
	}

	gui.buildContexts()
	return gui
}

// buildContexts creates one context per pane and attaches its controllers. The
// panes without controllers still get a render hook, because their content is
// pure presentation.
func (g *Gui) buildContexts() {
	common := controllers.NewCommon(g, g.cfg)

	global := context.New(context.Opts{Key: types.GLOBAL_CONTEXT_KEY})
	todos := context.New(context.Opts{
		Key:       types.TODOS_CONTEXT_KEY,
		View:      g.views.Todos,
		Focusable: true,
	})
	status := context.New(context.Opts{Key: types.STATUS_CONTEXT_KEY, View: g.views.Status})
	detail := context.New(context.Opts{
		Key:       types.DETAIL_CONTEXT_KEY,
		View:      g.views.Detail,
		Focusable: true,
	})
	options := context.New(context.Opts{Key: types.OPTIONS_CONTEXT_KEY, View: g.views.Options})
	popup := context.New(context.Opts{Key: types.POPUP_CONTEXT_KEY, View: g.views.Popup})

	context.AttachControllers(global, controllers.NewGlobalController(common))
	context.AttachControllers(todos, controllers.NewTodosController(common))
	context.AttachControllers(detail, controllers.NewDetailController(common))

	context.AttachRenderFn(status, g.renderStatus)
	context.AttachRenderFn(options, g.renderOptions)
	context.AttachRenderFn(popup, g.renderPopup)

	g.contexts = map[types.ContextKey]types.Context{
		types.GLOBAL_CONTEXT_KEY:  global,
		types.TODOS_CONTEXT_KEY:   todos,
		types.STATUS_CONTEXT_KEY:  status,
		types.DETAIL_CONTEXT_KEY:  detail,
		types.OPTIONS_CONTEXT_KEY: options,
		types.POPUP_CONTEXT_KEY:   popup,
	}
}

// Run is the input loop: poll, dispatch, redraw. Everything that changes the
// screen does so by calling Refresh, so there is exactly one render path.
func (g *Gui) Run() error {
	if err := g.screen.Init(); err != nil {
		return err
	}
	defer g.screen.Close()

	g.Refresh()
	if err := g.Render(); err != nil {
		return err
	}

	for !g.quit {
		event, err := g.screen.PollEvent()
		if err != nil {
			return err
		}

		switch event.Type {
		case tui.EventResize:
			// Refresh re-runs layout, so a resize needs no special case beyond
			// rebuilding the panes at the new size.
			g.Refresh()

		case tui.EventKey:
			if err := g.HandleKey(event.Key); err != nil {
				g.ReportError(err)
			}

		case tui.EventClick:
			if err := g.HandleClick(event.X, event.Y); err != nil {
				g.ReportError(err)
			}
		}

		if err := g.Render(); err != nil {
			return err
		}
	}

	return nil
}

// Layout places the panes for the current screen size. It runs at the start of
// every Refresh, so content is always built against up-to-date bounds.
func (g *Gui) Layout() {
	width, height := g.screen.Size()
	if width < 2 || height < 2 {
		return
	}

	lastRow := height - 1
	statusRow := lastRow
	if height >= 3 {
		statusRow = height - 2
	}

	contentBottom := statusRow - 1
	if contentBottom < 0 {
		contentBottom = 0
	}

	if width >= minTwoPaneWidth {
		listWidth := width * 2 / 5
		if listWidth < minListWidth {
			listWidth = minListWidth
		}
		g.views.Todos.SetBounds(0, 0, listWidth-1, contentBottom)
		g.views.Detail.SetBounds(listWidth, 0, width-1, contentBottom)
		g.views.Detail.Visible = true
	} else {
		g.views.Todos.SetBounds(0, 0, width-1, contentBottom)
		g.views.Detail.Visible = false
	}

	g.views.Status.SetBounds(0, statusRow, width-1, statusRow)
	g.views.Options.SetBounds(0, lastRow, width-1, lastRow)

	g.layoutPopup(width, height)
}

// layoutPopup centres the modal over everything else, giving it just enough
// height for its content and capping it to the screen.
func (g *Gui) layoutPopup(width, height int) {
	popupWidth := width - popupMarginX*2
	if popupWidth > popupMaxWidth {
		popupWidth = popupMaxWidth
	}
	if popupWidth < 12 {
		popupWidth = width - 2
	}
	if popupWidth < 1 {
		popupWidth = 1
	}

	lines, _ := presentation.PopupLines(&g.state.Popup, popupWidth-2)

	popupHeight := len(lines) + popupFrameRows
	if maxHeight := height - popupMarginY; popupHeight > maxHeight {
		popupHeight = maxHeight
	}
	if popupHeight < 3 {
		popupHeight = 3
	}
	if popupHeight > height {
		popupHeight = height
	}

	x0 := (width - popupWidth) / 2
	y0 := (height - popupHeight) / 2
	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}

	g.views.Popup.SetBounds(x0, y0, x0+popupWidth-1, y0+popupHeight-1)
}

// Render clears the screen and paints every view. Views later in Views.All()
// win on overlap, which is why the popup floats above the panes.
func (g *Gui) Render() error {
	width, height := g.screen.Size()
	blank := tui.Style{}
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			g.screen.SetCell(x, y, ' ', blank)
		}
	}

	for _, view := range g.views.All() {
		view.Render(g.screen)
	}

	return g.screen.Show()
}

// Refresh re-derives the model from the store and re-renders every pane. It is
// cheap and idempotent: actions call it instead of patching views by hand.
//
// The filters compose: the text filter narrows first, and its tally is what the
// status bar reports, so the numbers stay honest while both are active.
func (g *Gui) Refresh() {
	g.Layout()

	if !g.paneVisible(g.focus) {
		g.focus = types.TODOS_CONTEXT_KEY
	}

	base := g.store.Filter(g.state.Filter, models.StatusAll)
	g.state.StatusCounts = models.Tally(base)
	g.state.Todos = g.store.Filter(g.state.Filter, g.state.StatusFilter)

	g.clampSelection()
	g.syncFocus()

	for _, ctx := range g.renderOrder() {
		ctx.HandleRender()
	}
}

// renderOrder is the order panes get rebuilt in. The options bar reads the
// focused pane's bindings, so it has to come after the list is up to date.
func (g *Gui) renderOrder() []types.Context {
	return []types.Context{
		g.contexts[types.TODOS_CONTEXT_KEY],
		g.contexts[types.DETAIL_CONTEXT_KEY],
		g.contexts[types.STATUS_CONTEXT_KEY],
		g.contexts[types.OPTIONS_CONTEXT_KEY],
		g.contexts[types.POPUP_CONTEXT_KEY],
	}
}

func (g *Gui) clampSelection() {
	n := len(g.state.Todos)
	switch {
	case n == 0:
		g.state.SelectedIdx = 0
	case g.state.SelectedIdx < 0:
		g.state.SelectedIdx = 0
	case g.state.SelectedIdx > n-1:
		g.state.SelectedIdx = n - 1
	}
}

// syncFocus tells the views whether they hold the cursor, so an unfocused pane
// dims its selection instead of looking active.
func (g *Gui) syncFocus() {
	for key, ctx := range g.contexts {
		if view := ctx.GetView(); view != nil {
			view.Focused = key == g.focus
		}
	}
}

// HandleKey dispatches one key press. An open popup swallows everything, then
// the focused pane gets first refusal, then the global bindings.
func (g *Gui) HandleKey(key tui.Key) error {
	// Info is one-shot: any key press clears last time's message.
	g.state.Info = ""

	if g.state.Mode == types.ModePopup {
		return g.handlePopupKey(key)
	}

	for _, binding := range g.bindings() {
		if !binding.Match(key) {
			continue
		}
		if reason := binding.Disabled(); reason != nil {
			g.ReportInfo(reason.Text)
			return nil
		}
		return binding.Handler()
	}

	return nil
}

// HandleClick dispatches one left click. An open popup swallows clicks too, so a
// modal really is modal.
//
// The filter tabs are checked before any pane: they live on the list pane's top
// border, so a naive "is the click inside this view" test would also match them.
func (g *Gui) HandleClick(x, y int) error {
	g.state.Info = ""

	if g.state.Mode == types.ModePopup {
		return nil
	}

	if index, ok := g.views.Todos.TabAt(x, y); ok {
		g.SetStatusFilter(presentation.StatusAtTab(index))
		return nil
	}

	for _, key := range []types.ContextKey{types.TODOS_CONTEXT_KEY, types.DETAIL_CONTEXT_KEY} {
		ctx, ok := g.contexts[key]
		if !ok {
			continue
		}
		view := ctx.GetView()
		if view == nil || !view.Visible || !view.Contains(x, y) {
			continue
		}

		g.Focus(key)
		return ctx.HandleClick(x, y)
	}

	return nil
}

// SetStatusFilter narrows the list to one status (StatusAll clears it). The
// filter tabs and the Tab key both call this; it is the single place the filter
// is written, so the cursor reset can't be forgotten.
func (g *Gui) SetStatusFilter(status models.Status) {
	g.state.StatusFilter = status
	g.state.SelectedIdx = 0
	g.Refresh()
}

func (g *Gui) bindings() []*types.Binding {
	var bindings []*types.Binding

	if focused, ok := g.contexts[g.focus]; ok {
		bindings = append(bindings, focused.GetKeybindings()...)
	}
	if global, ok := g.contexts[types.GLOBAL_CONTEXT_KEY]; ok {
		bindings = append(bindings, global.GetKeybindings()...)
	}

	return bindings
}

// handlePopupKey is the modal editor: Tab cycles fields, Enter submits from a
// one-line field (or adds a newline in a multi-line one), Ctrl+S submits from
// anywhere, Esc cancels.
func (g *Gui) handlePopupKey(key tui.Key) error {
	if g.state.Popup.IsConfirm() {
		return g.handleConfirmKey(key)
	}

	switch key {
	case tui.KeyEsc:
		g.closePopup()
		return nil

	case tui.KeyCtrlS:
		return g.submitPopup()

	case tui.KeyTab:
		g.cyclePopupField(1)
		return nil

	case tui.KeyBacktab:
		g.cyclePopupField(-1)
		return nil

	case tui.KeyEnter:
		if field := g.state.Popup.Current(); field != nil && field.Multiline {
			field.Text += "\n"
			g.state.Popup.Error = ""
			g.Refresh()
			return nil
		}
		return g.submitPopup()

	case tui.KeyBackspace:
		if field := g.state.Popup.Current(); field != nil {
			if runes := []rune(field.Text); len(runes) > 0 {
				field.Text = string(runes[:len(runes)-1])
			}
			g.state.Popup.Error = ""
			g.Refresh()
		}
		return nil
	}

	if r, ok := tui.RuneOfKey(key); ok {
		if field := g.state.Popup.Current(); field != nil {
			field.Text += string(r)
			g.state.Popup.Error = ""
		}
		g.Refresh()
	}

	return nil
}

// handleConfirmKey accepts only y (do it) and n/escape (forget it). Everything
// else is swallowed, so a stray keystroke leaves the question on screen.
func (g *Gui) handleConfirmKey(key tui.Key) error {
	switch key {
	case tui.Key("y"), tui.Key("Y"):
		onConfirm := g.state.Popup.OnConfirm
		g.closePopup()
		if onConfirm == nil {
			return nil
		}
		return onConfirm()

	case tui.Key("n"), tui.Key("N"), tui.KeyEsc:
		g.closePopup()
	}

	return nil
}

func (g *Gui) cyclePopupField(delta int) {
	popup := &g.state.Popup
	if len(popup.Fields) == 0 {
		return
	}

	popup.Focus = (popup.Focus + delta + len(popup.Fields)) % len(popup.Fields)
	popup.Error = ""
	g.Refresh()
}

func (g *Gui) submitPopup() error {
	popup := &g.state.Popup
	if popup.OnSubmit == nil {
		g.closePopup()
		return nil
	}

	values := make([]string, len(popup.Fields))
	for i, field := range popup.Fields {
		values[i] = field.Text
	}

	// A handler can reject the input (an empty title, say). Keeping the popup
	// open and saying why beats closing it and losing what was typed.
	if err := popup.OnSubmit(values); err != nil {
		popup.Error = err.Error()
		g.Refresh()
		return nil
	}

	g.closePopup()
	return nil
}

// renderStatus shows either the one-shot info message or the counters.
func (g *Gui) renderStatus() {
	if g.state.Info != "" {
		g.views.Status.SetContent([]string{g.state.Info})
		return
	}

	g.views.Status.SetContent([]string{
		presentation.StatusLine(g.state.StatusCounts, g.state.Filter, g.state.StatusFilter, g.tr),
	})
}

// renderOptions builds the hint bar from the bindings that asked to be shown and
// are currently runnable, so it never advertises a key that would do nothing.
func (g *Gui) renderOptions() {
	if g.state.Mode == types.ModePopup {
		// The popup carries its own hints in its footer.
		g.views.Options.SetContent(nil)
		return
	}

	var hints []string
	for _, binding := range g.bindings() {
		if !binding.DisplayOnScreen || len(binding.Keys) == 0 || binding.Disabled() != nil {
			continue
		}
		hints = append(hints, "["+binding.Keys[0].Label()+"] "+binding.Description)
	}

	g.views.Options.SetContent([]string{presentation.OptionsLine(hints)})
}

func (g *Gui) renderPopup() {
	view := g.views.Popup
	view.Visible = g.state.Mode == types.ModePopup

	if !view.Visible {
		view.SetContent(nil)
		view.Footer = ""
		return
	}

	view.Title = g.state.Popup.Title
	lines, caretLine := presentation.PopupLines(&g.state.Popup, view.InnerWidth())
	view.SetContent(lines)
	view.ScrollToLine(caretLine)

	if message := g.state.Popup.Error; message != "" {
		view.Footer = message
		view.FooterFg = tui.ColorRed
		return
	}

	view.Footer = g.popupFooter()
	view.FooterFg = tui.ColorDefault
}

// popupFooter advertises the keys that apply to the focused field.
func (g *Gui) popupFooter() string {
	if g.state.Popup.IsConfirm() {
		return g.tr.ConfirmFooter
	}

	if field := g.state.Popup.Current(); field != nil && field.Multiline {
		return g.tr.FormFooterMulti
	}
	return g.tr.FormFooter
}

// Store is the command layer, for controllers.
func (g *Gui) Store() *commands.Store { return g.store }

// State is the live model, for controllers.
func (g *Gui) State() *types.State { return &g.state }

// Views exposes the panes, for controllers.
func (g *Gui) Views() types.Views { return g.views }

// Config exposes the user configuration, for controllers.
func (g *Gui) Config() *config.UserConfig { return g.cfg }

// Tr is the interface language, resolved once at startup.
func (g *Gui) Tr() *i18n.TranslationSet { return g.tr }

// Popup opens the modal editor.
func (g *Gui) Popup(title string, fields []types.Field, onSubmit func(values []string) error) {
	g.state.Mode = types.ModePopup
	g.state.Popup = types.PopupState{
		Title:    title,
		Fields:   fields,
		OnSubmit: onSubmit,
	}
	g.views.Popup.Visible = true
	g.Refresh()
}

// Confirm opens a yes/no modal. Enter is deliberately not bound: a destructive
// action shouldn't be one muscle-memory keystroke away from the form-submit key.
func (g *Gui) Confirm(title, prompt string, onConfirm func() error) {
	g.state.Mode = types.ModePopup
	g.state.Popup = types.PopupState{
		Title:     title,
		Prompt:    prompt,
		OnConfirm: onConfirm,
	}
	g.views.Popup.Visible = true
	g.Refresh()
}

func (g *Gui) closePopup() {
	g.state.Mode = types.ModeNormal
	g.state.Popup = types.PopupState{}
	g.views.Popup.Visible = false
	g.Refresh()
}

// Focus moves the cursor to a focusable pane.
func (g *Gui) Focus(key types.ContextKey) {
	target, ok := g.contexts[key]
	if !ok || !target.IsFocusable() || !g.paneVisible(key) {
		return
	}

	if current, ok := g.contexts[g.focus]; ok {
		current.HandleFocusLost()
	}
	g.focus = key
	target.HandleFocus()
	g.Refresh()
}

func (g *Gui) paneVisible(key types.ContextKey) bool {
	ctx, ok := g.contexts[key]
	if !ok {
		return false
	}
	view := ctx.GetView()
	return view != nil && view.Visible
}

// Copy puts text on the system clipboard through the screen driver, so the same
// call works on a real terminal and on the fake one used by tests.
func (g *Gui) Copy(text string) error {
	return g.screen.SetClipboard(text)
}

// ClearFilters drops both filters and puts the cursor back at the top of the
// list.
func (g *Gui) ClearFilters() {
	g.state.Filter = ""
	g.state.StatusFilter = models.StatusAll
	g.state.SelectedIdx = 0
	g.Refresh()
}

// ReportInfo shows a message in the status bar until the next key press.
func (g *Gui) ReportInfo(msg string) {
	g.state.Info = msg
	g.Refresh()
}

// ReportError shows an error in the same slot as ReportInfo. Errors are not
// fatal: the user can keep working, and the file on disk is untouched because
// the failing step came before Save.
func (g *Gui) ReportError(err error) {
	g.ReportInfo(g.tr.ErrorPrefix + err.Error())
}

// Quit ends the input loop. The store is already saved by the action that
// changed it, so there is nothing to flush here.
func (g *Gui) Quit() { g.quit = true }

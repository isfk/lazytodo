// Package context holds the pane state: which view a context drives, whether it
// can hold the cursor, and the hooks controllers attach to it.
package context

import (
	"github.com/isfk/lazytodo/pkg/gui/types"
	"github.com/isfk/lazytodo/pkg/tui"
)

// BaseContext is the one context implementation. A pane that needs extra
// behaviour gets it by attaching controllers (keybindings, render, focus hooks)
// rather than by defining a new type, which is why this file is small.
type BaseContext struct {
	key       types.ContextKey
	view      *tui.View
	focusable bool

	keybindingFns []func() []*types.Binding
	renderFns     []func()
	focusFns      []func()
	focusLostFns  []func()
	clickFns      []func(x, y int) error
}

var _ types.Context = &BaseContext{}

// Opts configures a new context.
type Opts struct {
	Key       types.ContextKey
	View      *tui.View
	Focusable bool
}

// New builds a context around a view.
func New(opts Opts) *BaseContext {
	return &BaseContext{
		key:       opts.Key,
		view:      opts.View,
		focusable: opts.Focusable,
	}
}

func (c *BaseContext) GetKey() types.ContextKey { return c.key }
func (c *BaseContext) GetView() *tui.View       { return c.view }
func (c *BaseContext) IsFocusable() bool        { return c.focusable }

// GetKeybindings concatenates every attached controller's bindings. The most
// recently attached controller wins on conflicts, exactly like lazygit, so a
// feature-specific controller can override a general one.
func (c *BaseContext) GetKeybindings() []*types.Binding {
	var bindings []*types.Binding
	for i := len(c.keybindingFns) - 1; i >= 0; i-- {
		bindings = append(bindings, c.keybindingFns[i]()...)
	}
	return bindings
}

func (c *BaseContext) AddKeybindingsFn(fn func() []*types.Binding) {
	c.keybindingFns = append(c.keybindingFns, fn)
}

func (c *BaseContext) AddOnRenderFn(fn func()) {
	if fn != nil {
		c.renderFns = append(c.renderFns, fn)
	}
}

func (c *BaseContext) AddOnFocusFn(fn func()) {
	if fn != nil {
		c.focusFns = append(c.focusFns, fn)
	}
}

func (c *BaseContext) AddOnFocusLostFn(fn func()) {
	if fn != nil {
		c.focusLostFns = append(c.focusLostFns, fn)
	}
}

func (c *BaseContext) AddOnClickFn(fn func(x, y int) error) {
	if fn != nil {
		c.clickFns = append(c.clickFns, fn)
	}
}

// HandleClick offers the click to the attached controllers in order; the first
// one that is wired up handles it.
func (c *BaseContext) HandleClick(x, y int) error {
	for _, fn := range c.clickFns {
		return fn(x, y)
	}
	return nil
}

// HandleRender rebuilds the pane's text from the current model. This is the
// pull half of the refresh story: nothing pushes content into a view except
// this.
func (c *BaseContext) HandleRender() {
	for _, fn := range c.renderFns {
		fn()
	}
}

func (c *BaseContext) HandleFocus() {
	for _, fn := range c.focusFns {
		fn()
	}
}

func (c *BaseContext) HandleFocusLost() {
	for _, fn := range c.focusLostFns {
		fn()
	}
}

// AttachControllers wires controllers into a context. Controllers only have to
// implement the hooks they care about, so this is a series of optional type
// assertions rather than a required interface.
func AttachControllers(ctx *BaseContext, controllers ...types.IController) {
	for _, controller := range controllers {
		ctx.AddKeybindingsFn(controller.GetKeybindings)

		if c, ok := controller.(types.IRenderHook); ok {
			ctx.AddOnRenderFn(c.GetOnRender())
		}
		if c, ok := controller.(types.IFocusHook); ok {
			ctx.AddOnFocusFn(c.GetOnFocus())
		}
		if c, ok := controller.(types.IFocusLostHook); ok {
			ctx.AddOnFocusLostFn(c.GetOnFocusLost())
		}
		if c, ok := controller.(types.IClickHook); ok {
			ctx.AddOnClickFn(c.GetOnClick())
		}
	}
}

// AttachRenderFn is a convenience for the panes whose content is pure
// presentation and needs no controller at all.
func AttachRenderFn(ctx *BaseContext, fn func()) {
	ctx.AddOnRenderFn(fn)
}

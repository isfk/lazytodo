package controllers

import (
	"github.com/shen/lazytodo/pkg/gui/types"
	"github.com/shen/lazytodo/pkg/tui"
)

// GlobalController holds the bindings that work in every pane. The GUI consults
// the focused pane's bindings first, then these, so a pane can always override a
// global key.
type GlobalController struct {
	c *Common
}

var _ types.IController = &GlobalController{}

// NewGlobalController builds the global controller.
func NewGlobalController(c *Common) *GlobalController {
	return &GlobalController{c: c}
}

// GetKeybindings returns the always-available bindings.
func (self *GlobalController) GetKeybindings() []*types.Binding {
	keys := self.c.Config.Keybinding.Universal

	return []*types.Binding{
		{
			Keys:        []tui.Key{keys.Quit},
			Handler:     func() error { self.c.Gui.Quit(); return nil },
			Description: self.c.Tr.Quit,
			Tooltip:     "Quit the app; everything is already saved",
		},
		{
			Keys:        []tui.Key{keys.Refresh},
			Handler:     func() error { self.c.Gui.Refresh(); return nil },
			Description: self.c.Tr.Reload,
			Tooltip:     "Re-read the todo file from disk",
		},
	}
}

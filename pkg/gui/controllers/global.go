package controllers

import (
	"fmt"

	"github.com/isfk/lazytodo/pkg/agents"
	"github.com/isfk/lazytodo/pkg/gui/types"
	"github.com/isfk/lazytodo/pkg/tui"
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
		{
			Keys:        []tui.Key{keys.InstallAgent},
			Handler:     self.installAgent,
			Description: self.c.Tr.InstallAgent,
			Tooltip:     "Write the agent integration into ~/.pi/agent/extensions",
		},
	}
}

// installAgent drops the embedded agent integration into the agent's extension
// folder, so a fresh machine needs only `lazytodo` plus one keystroke. The file is
// ours and versioned here, so overwriting it is the whole point of the key.
func (self *GlobalController) installAgent() error {
	result, err := agents.Install(agents.Pi)
	if err != nil {
		return err
	}

	switch {
	case !result.Changed:
		self.c.Gui.ReportInfo(fmt.Sprintf(self.c.Tr.ExtUpToDate, result.Path))
	case result.Backup != "":
		self.c.Gui.ReportInfo(fmt.Sprintf(self.c.Tr.ExtInstalledBackup, result.Path, result.Backup))
	default:
		self.c.Gui.ReportInfo(fmt.Sprintf(self.c.Tr.ExtInstalled, result.Path))
	}
	return nil
}

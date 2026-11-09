// Package controllers holds the key handlers. A controller never draws: it
// mutates the store or the GUI state and calls Refresh, which then pulls new
// content into the views.
package controllers

import (
	"github.com/isfk/lazytodo/pkg/config"
	"github.com/isfk/lazytodo/pkg/gui/types"
	"github.com/isfk/lazytodo/pkg/i18n"
)

// Common is everything a controller is allowed to reach. Passing one struct
// instead of a dozen arguments keeps constructors readable and makes it obvious
// when a controller starts reaching for something it shouldn't.
type Common struct {
	Gui    types.IGuiCommon
	Config *config.UserConfig
	Tr     *i18n.TranslationSet
}

// NewCommon builds the controller-side view of the GUI.
func NewCommon(gui types.IGuiCommon, cfg *config.UserConfig) *Common {
	return &Common{Gui: gui, Config: cfg, Tr: gui.Tr()}
}

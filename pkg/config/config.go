// Package config holds the user-facing knobs. Everything is plain data with
// defaults built in, so the GUI can read it without any I/O; loading a file on
// top of these defaults is a later, isolated change.
package config

import (
	"github.com/isfk/lazytodo/pkg/i18n"
	"github.com/isfk/lazytodo/pkg/tui"
)

// UserConfig is the whole configuration tree.
type UserConfig struct {
	// Language is a language code such as "en" or "zh". Default() fills it in
	// from the locale the shell exports.
	Language   string
	Keybinding Keybindings
}

// Keybindings groups bindings by the pane they apply to, mirroring how the
// controllers are attached. Tests reference these fields directly
// (keys.Todos.CycleStatus) so that changing a default key never breaks a test.
type Keybindings struct {
	Universal UniversalKeybindings
	Todos     TodosKeybindings
	Detail    DetailKeybindings
}

type UniversalKeybindings struct {
	Quit    tui.Key
	Refresh tui.Key
}

type TodosKeybindings struct {
	Next        tui.Key
	Prev        tui.Key
	Top         tui.Key
	Bottom      tui.Key
	CycleStatus tui.Key
	Add         tui.Key
	Edit        tui.Key
	Delete      tui.Key
	Filter      tui.Key
	ClearFilter tui.Key
	// NextStatusFilter / PrevStatusFilter walk the All / Doing / Done tabs that
	// sit on the list pane's title row.
	NextStatusFilter tui.Key
	PrevStatusFilter tui.Key
	// FocusDetail moves the cursor into the detail pane.
	FocusDetail tui.Key
}

// StatusesKeybindings is gone: the status filter is now a row of clickable tabs
// on the list pane's top border, so there is no pane to drive with keys.

// DetailKeybindings drives the read-only detail pane.
type DetailKeybindings struct {
	Next      tui.Key
	Prev      tui.Key
	Top       tui.Key
	Bottom    tui.Key
	Copy      tui.Key
	FocusList tui.Key
}

// Default returns the built-in configuration.
func Default() *UserConfig {
	return &UserConfig{
		Language: i18n.Detect(),
		Keybinding: Keybindings{
			Universal: UniversalKeybindings{
				Quit:    "q",
				Refresh: tui.KeyCtrlD,
			},
			Todos: TodosKeybindings{
				Next:             "j",
				Prev:             "k",
				Top:              "g",
				Bottom:           "G",
				CycleStatus:      tui.KeySpace,
				Add:              "a",
				Edit:             "e",
				Delete:           "d",
				Filter:           "/",
				ClearFilter:      tui.KeyEsc,
				NextStatusFilter: tui.KeyTab,
				PrevStatusFilter: tui.KeyBacktab,
				FocusDetail:      "l",
			},
			Detail: DetailKeybindings{
				Next:      "j",
				Prev:      "k",
				Top:       "g",
				Bottom:    "G",
				Copy:      "y",
				FocusList: "h",
			},
		},
	}
}

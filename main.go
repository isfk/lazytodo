// Command lazytodo is a small TUI todo list, structured the way lazygit is:
// a command layer that owns the data, a set of panes with controllers attached,
// and a screen abstraction that the tests can fake.
package main

import (
	"fmt"
	"os"

	"github.com/shen/lazytodo/pkg/commands"
	"github.com/shen/lazytodo/pkg/config"
	"github.com/shen/lazytodo/pkg/gui"
	"github.com/shen/lazytodo/pkg/tui/tcelldriver"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "lazytodo:", err)
		os.Exit(1)
	}
}

func run() error {
	store := commands.NewStore(commands.DefaultPath())
	if err := store.Load(); err != nil {
		return fmt.Errorf("loading %s: %w", store.Path(), err)
	}

	return gui.New(tcelldriver.New(), store, config.Default()).Run()
}

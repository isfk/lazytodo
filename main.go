// Command lazytodo is a small TUI todo list, structured the way lazygit is:
// a command layer that owns the data, a set of panes with controllers attached,
// and a screen abstraction that the tests can fake.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/isfk/lazytodo/pkg/agents"
	"github.com/isfk/lazytodo/pkg/commands"
	"github.com/isfk/lazytodo/pkg/config"
	"github.com/isfk/lazytodo/pkg/gui"
	"github.com/isfk/lazytodo/pkg/tui/tcelldriver"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "lazytodo:", err)
		os.Exit(1)
	}
}

func run() error {
	installFor := flag.String("install-extension", "",
		"install the integration for a coding agent and exit (agents: "+strings.Join(agentNames(), ", ")+")")
	flag.Parse()

	if *installFor != "" {
		return installExtension(*installFor)
	}

	store := commands.NewStore(commands.DefaultPath())
	if err := store.Load(); err != nil {
		return fmt.Errorf("loading %s: %w", store.Path(), err)
	}
	if err := store.EnsureFile(); err != nil {
		return fmt.Errorf("preparing %s: %w", store.Path(), err)
	}

	return gui.New(tcelldriver.New(), store, config.Default()).Run()
}

// installExtension is the scriptable twin of the TUI's install key: same code
// path, so an agent can bootstrap its own integration in one command.
func installExtension(name string) error {
	agent, ok := agents.For(name)
	if !ok {
		return fmt.Errorf("unknown agent %q, known: %s", name, strings.Join(agentNames(), ", "))
	}

	result, err := agents.Install(agent)
	if err != nil {
		return err
	}

	fmt.Println(result.Path)
	switch {
	case !result.Changed:
		fmt.Println("already up to date")
	case result.Backup != "":
		fmt.Printf("previous file kept as %s\n", result.Backup)
	}
	return nil
}

func agentNames() []string {
	names := make([]string, 0, len(agents.All()))
	for _, agent := range agents.All() {
		names = append(names, agent.Name)
	}
	return names
}

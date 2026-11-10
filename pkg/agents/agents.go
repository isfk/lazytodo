// Package agents ships the integrations that let a CLI coding agent write into
// the todo list.
//
// Each integration is a source file embedded in the binary, so `lazytodo` can
// drop it into the agent's extension folder without downloading anything. Adding
// a new agent (codex, claude, ...) means adding one Agent value here and the
// matching source file next to it -- nothing else in the app changes.
package agents

import (
	"embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

//go:embed pi/lazytodo.ts
var sources embed.FS

// Agent describes one CLI agent whose extension folder we know how to fill.
type Agent struct {
	// Name is what the user types on the command line.
	Name string
	// Folder is the path below $HOME that holds the agent's extensions.
	Folder []string
	// File is the name of the file we write there.
	File string
	// Source is the path inside this package the file is embedded from.
	Source string
}

// Pi is the pi coding-agent integration.
var Pi = Agent{
	Name:   "pi",
	Folder: []string{".pi", "agent", "extensions"},
	File:   "lazytodo.ts",
	Source: "pi/lazytodo.ts",
}

// All is every agent we know how to install into.
func All() []Agent { return []Agent{Pi} }

// For looks an agent up by name.
func For(name string) (Agent, bool) {
	for _, agent := range All() {
		if agent.Name == name {
			return agent, true
		}
	}
	return Agent{}, false
}

// Result reports what an install actually did, so the caller can tell the user
// something more useful than "ok".
type Result struct {
	Path    string
	Changed bool   // false when the file was already up to date
	Backup  string // set when an existing, different file was kept aside
}

// Target is where the agent's integration lives on this machine.
func (a Agent) Target() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locate home directory: %w", err)
	}

	path := append([]string{home}, a.Folder...)
	return filepath.Join(append(path, a.File)...), nil
}

// Install writes the agent's integration, creating folders as needed.
//
// An existing file that differs is copied aside to <file>.bak.<timestamp> first:
// the file is ours and overwriting is the point, but a local edit someone made by
// hand is never silently thrown away.
func Install(agent Agent) (Result, error) {
	source, err := sources.ReadFile(agent.Source)
	if err != nil {
		return Result{}, fmt.Errorf("read embedded %s: %w", agent.Source, err)
	}

	path, err := agent.Target()
	if err != nil {
		return Result{}, err
	}

	result := Result{Path: path}

	existing, readErr := os.ReadFile(path)
	switch {
	case readErr == nil && string(existing) == string(source):
		return result, nil

	case readErr == nil:
		backup := fmt.Sprintf("%s.bak.%s", path, time.Now().Format("20060102-150405"))
		if err := os.WriteFile(backup, existing, 0o644); err != nil {
			return Result{}, fmt.Errorf("back up %s: %w", path, err)
		}
		result.Backup = backup

	case !errors.Is(readErr, os.ErrNotExist):
		return Result{}, fmt.Errorf("read %s: %w", path, readErr)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return Result{}, fmt.Errorf("create %s: %w", filepath.Dir(path), err)
	}

	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, source, 0o644); err != nil {
		return Result{}, fmt.Errorf("write %s: %w", temporary, err)
	}
	if err := os.Rename(temporary, path); err != nil {
		_ = os.Remove(temporary)
		return Result{}, fmt.Errorf("move into place: %w", err)
	}

	result.Changed = true
	return result, nil
}

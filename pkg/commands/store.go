// Package commands is the app's only I/O layer: it owns the todo list and its
// file on disk. It knows nothing about the GUI, so it can be tested headlessly
// and swapped out (e.g. for a remote backend) without touching pkg/gui.
package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/isfk/lazytodo/pkg/models"
)

// ErrNotFound is returned by the mutating helpers when the id is gone. Callers
// in the GUI never see it in practice (they always act on the loaded list), so
// it exists mostly to keep the layer honest.
var ErrNotFound = errors.New("todo not found")

type storeFile struct {
	NextID int            `json:"nextId"`
	Todos  []*models.Todo `json:"todos"`
}

// Store is the todo command layer: load, save, and the handful of mutations the
// GUI needs. It holds the full list; filtering is a read-only projection.
type Store struct {
	path   string
	Todos  []*models.Todo
	nextID int
}

// NewStore builds a store around the given file path. The file is created on
// the first Save.
func NewStore(path string) *Store {
	return &Store{path: path, nextID: 1}
}

// DefaultPath is the todo file in the current directory.
//
// The list is per directory on purpose: run the tool in another project and you
// see that project's todos, with nothing shared behind your back. Resolved to an
// absolute path so error messages say exactly which file is being read.
func DefaultPath() string {
	const name = ".lazytodo.json"

	if wd, err := os.Getwd(); err == nil {
		return filepath.Join(wd, name)
	}
	return name
}

func (s *Store) Path() string { return s.path }

// Signature changes whenever the file on disk changes (or appears/disappears),
// without reading it. A watcher compares two signatures to decide whether to
// reload -- this is how a todo written by another process reaches the screen.
func (s *Store) Signature() string {
	info, err := os.Stat(s.path)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%d/%d", info.ModTime().UnixNano(), info.Size())
}

// Load reads the file into memory. A missing file is not an error: it just means
// an empty list.
func (s *Store) Load() error {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		s.Todos = nil
		s.nextID = 1
		return nil
	}
	if err != nil {
		return fmt.Errorf("read %s: %w", s.path, err)
	}

	var f storeFile
	if err := json.Unmarshal(data, &f); err != nil {
		return fmt.Errorf("parse %s: %w", s.path, err)
	}

	s.Todos = f.Todos
	s.nextID = f.NextID
	if s.nextID < 1 {
		s.nextID = s.maxID() + 1
	}
	return nil
}

// Save writes the list atomically (temp file + rename) so a crash mid-write
// can't truncate the user's todos.
func (s *Store) Save() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	data, err := json.MarshalIndent(storeFile{NextID: s.nextID, Todos: s.Todos}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode: %w", err)
	}
	data = append(data, '\n')

	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename into place: %w", err)
	}
	return nil
}

// Add appends a todo in the "todo" state and returns it.
func (s *Store) Add(title, detail string) *models.Todo {
	todo := &models.Todo{
		ID:     s.nextID,
		Title:  title,
		Detail: detail,
		Status: models.StatusTodo,
	}
	s.nextID++
	s.Todos = append(s.Todos, todo)
	return todo
}

// CycleStatus advances every given todo one step along todo -> doing -> done.
func (s *Store) CycleStatus(ids ...int) error {
	return s.each(ids, func(t *models.Todo) { t.Status = t.Status.Next() })
}

// SetTitle renames one todo.
func (s *Store) SetTitle(id int, title string) error {
	return s.each([]int{id}, func(t *models.Todo) { t.Title = title })
}

// SetDetail replaces one todo's description.
func (s *Store) SetDetail(id int, detail string) error {
	return s.each([]int{id}, func(t *models.Todo) { t.Detail = detail })
}

// Delete removes every given id.
func (s *Store) Delete(ids ...int) error {
	if len(ids) == 0 {
		return nil
	}

	drop := make(map[int]bool, len(ids))
	for _, id := range ids {
		drop[id] = true
	}

	kept := make([]*models.Todo, 0, len(s.Todos))
	for _, t := range s.Todos {
		if !drop[t.ID] {
			kept = append(kept, t)
		}
	}
	if len(kept) == len(s.Todos) {
		return ErrNotFound
	}
	s.Todos = kept
	return nil
}

// Filter returns the projection of Todos matching both the text filter and the
// status filter, preserving order. Pass models.StatusAll to skip the status
// narrowing; pass "" as text to match everything.
func (s *Store) Filter(text string, status models.Status) []*models.Todo {
	out := make([]*models.Todo, 0, len(s.Todos))
	for _, t := range s.Todos {
		if !t.Matches(text) {
			continue
		}
		if status != models.StatusAll && t.Status != status {
			continue
		}
		out = append(out, t)
	}
	return out
}

func (s *Store) each(ids []int, fn func(*models.Todo)) error {
	wanted := make(map[int]bool, len(ids))
	for _, id := range ids {
		wanted[id] = true
	}

	found := 0
	for _, t := range s.Todos {
		if wanted[t.ID] {
			fn(t)
			found++
		}
	}
	if found != len(wanted) {
		return ErrNotFound
	}
	return nil
}

func (s *Store) maxID() int {
	max := 0
	for _, t := range s.Todos {
		if t.ID > max {
			max = t.ID
		}
	}
	return max
}

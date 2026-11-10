package commands

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/isfk/lazytodo/pkg/models"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	return NewStore(filepath.Join(t.TempDir(), "todos.json"))
}

func TestDefaultPathIsInTheWorkingDirectory(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}

	want := filepath.Join(wd, ".lazytodo.json")
	if got := DefaultPath(); got != want {
		t.Errorf("DefaultPath() = %q, want %q", got, want)
	}
}

// TestSignatureTracksTheFile is what lets the TUI notice an outside write.
func TestSignatureTracksTheFile(t *testing.T) {
	s := newTestStore(t)

	if got := s.Signature(); got != "" {
		t.Errorf("Signature() on a missing file = %q, want empty", got)
	}

	s.Add("x", "")
	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	first := s.Signature()
	if first == "" {
		t.Fatal("Signature() is empty after save")
	}

	s.Add("y", "")
	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if got := s.Signature(); got == first {
		t.Error("Signature() did not change after the file was rewritten")
	}
}

func TestLoadMissingFileIsEmpty(t *testing.T) {
	s := newTestStore(t)

	if err := s.Load(); err != nil {
		t.Fatalf("Load() on missing file: %v", err)
	}
	if len(s.Todos) != 0 {
		t.Fatalf("expected empty list, got %d items", len(s.Todos))
	}
}

func TestSaveThenLoadRoundTrips(t *testing.T) {
	s := newTestStore(t)
	s.Add("write the store", "with an atomic write")
	s.Add("write the gui", "")
	if err := s.CycleStatus(2); err != nil {
		t.Fatalf("CycleStatus: %v", err)
	}
	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	reloaded := NewStore(s.Path())
	if err := reloaded.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}

	if got, want := len(reloaded.Todos), 2; got != want {
		t.Fatalf("len = %d, want %d", got, want)
	}
	if got := reloaded.Todos[0].Detail; got != "with an atomic write" {
		t.Fatalf("detail = %q, want the description to survive", got)
	}
	if got := reloaded.Todos[1].Status; got != models.StatusDoing {
		t.Fatalf("status = %q, want %q", got, models.StatusDoing)
	}

	// ids must keep counting up, not restart at 1
	if got := reloaded.Add("third", "").ID; got != 3 {
		t.Fatalf("next id = %d, want 3", got)
	}
}

func TestAddAssignsIncreasingIDs(t *testing.T) {
	s := newTestStore(t)

	for i, want := range []int{1, 2, 3} {
		if got := s.Add("x", "").ID; got != want {
			t.Fatalf("add #%d: id = %d, want %d", i, got, want)
		}
	}
}

func TestAddStartsInTheTodoState(t *testing.T) {
	s := newTestStore(t)

	if got := s.Add("x", "").Status; got != models.StatusTodo {
		t.Fatalf("status = %q, want %q", got, models.StatusTodo)
	}
}

func TestCycleStatusWalksTheThreeStates(t *testing.T) {
	s := newTestStore(t)
	s.Add("x", "")

	want := []models.Status{models.StatusDoing, models.StatusDone, models.StatusTodo}
	for i, expected := range want {
		if err := s.CycleStatus(1); err != nil {
			t.Fatalf("CycleStatus #%d: %v", i, err)
		}
		if got := s.Todos[0].Status; got != expected {
			t.Fatalf("after cycle #%d: status = %q, want %q", i+1, got, expected)
		}
	}
}

func TestDeleteRemovesOnlyRequested(t *testing.T) {
	s := newTestStore(t)
	s.Add("a", "")
	s.Add("b", "")
	s.Add("c", "")

	if err := s.Delete(1, 3); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if len(s.Todos) != 1 || s.Todos[0].ID != 2 {
		t.Fatalf("unexpected list after delete: %+v", s.Todos)
	}
}

func TestMutationsReportMissingIDs(t *testing.T) {
	s := newTestStore(t)
	s.Add("a", "")

	for name, err := range map[string]error{
		"cycle":  s.CycleStatus(99),
		"delete": s.Delete(99),
		"rename": s.SetTitle(99, "x"),
		"detail": s.SetDetail(99, "x"),
	} {
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("%s of a missing id: err = %v, want ErrNotFound", name, err)
		}
	}
}

func TestFilterIsCaseInsensitiveAndOrdered(t *testing.T) {
	s := newTestStore(t)
	s.Add("Buy MILK", "")
	s.Add("walk dog", "")
	s.Add("milk the cat", "")

	got := s.Filter("milk", models.StatusAll)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].Title != "Buy MILK" || got[1].Title != "milk the cat" {
		t.Fatalf("filter changed the order: %+v", []*models.Todo{got[0], got[1]})
	}

	if n := len(s.Filter("", models.StatusAll)); n != 3 {
		t.Fatalf("empty filter returned %d, want all 3", n)
	}
}

func TestFilterMatchesTheDescription(t *testing.T) {
	s := newTestStore(t)
	s.Add("write the store", "atomic rename")
	s.Add("walk the dog", "")

	got := s.Filter("atomic", models.StatusAll)
	if len(got) != 1 || got[0].Title != "write the store" {
		t.Fatalf("got %d matches, want only the described todo", len(got))
	}
}

// TestFilterComposesTextAndStatus covers the two filters that the GUI stacks.
func TestFilterComposesTextAndStatus(t *testing.T) {
	s := newTestStore(t)
	s.Add("write the store", "")
	s.Add("write the gui", "")
	s.Add("walk the dog", "")
	if err := s.CycleStatus(1, 2); err != nil {
		t.Fatalf("CycleStatus: %v", err)
	}

	if got := len(s.Filter("", models.StatusDoing)); got != 2 {
		t.Fatalf("status filter returned %d, want 2", got)
	}

	got := s.Filter("gui", models.StatusDoing)
	if len(got) != 1 || got[0].Title != "write the gui" {
		t.Fatalf("composed filter returned %d matches, want just the gui one", len(got))
	}

	if n := len(s.Filter("gui", models.StatusTodo)); n != 0 {
		t.Fatalf("wrong status returned %d matches, want 0", n)
	}
}

func TestTally(t *testing.T) {
	todos := []*models.Todo{
		{ID: 1, Status: models.StatusDone},
		{ID: 2, Status: models.StatusDoing},
		{ID: 3, Status: models.StatusDoing},
		{ID: 4, Status: models.StatusTodo},
	}

	counts := models.Tally(todos)
	if counts.Done != 1 || counts.Doing != 2 || counts.Todo != 1 || counts.Total() != 4 {
		t.Fatalf("Tally = %+v, want 1 todo, 2 doing, 1 done", counts)
	}

	if got := counts.Count(models.StatusAll); got != 4 {
		t.Fatalf("Count(All) = %d, want 4", got)
	}
	if got := counts.Count(models.StatusDoing); got != 2 {
		t.Fatalf("Count(Doing) = %d, want 2", got)
	}
}

// TestLoadsLegacyDoneFlag covers files written before todos had three states:
// those only carried a "done" bool, and must still load.
func TestLoadsLegacyDoneFlag(t *testing.T) {
	s := newTestStore(t)

	legacy := `{
	  "nextId": 3,
	  "todos": [
	    {"id": 1, "title": "finished", "done": true},
	    {"id": 2, "title": "pending", "done": false}
	  ]
	}`
	if err := os.WriteFile(s.Path(), []byte(legacy), 0o644); err != nil {
		t.Fatalf("writing legacy file: %v", err)
	}

	if err := s.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}

	if got := s.Todos[0].Status; got != models.StatusDone {
		t.Errorf("done:true loaded as %q, want %q", got, models.StatusDone)
	}
	if got := s.Todos[1].Status; got != models.StatusTodo {
		t.Errorf("done:false loaded as %q, want %q", got, models.StatusTodo)
	}
}

func TestSaveWritesTheCurrentShape(t *testing.T) {
	s := newTestStore(t)
	s.Add("x", "long form")
	if err := s.CycleStatus(1); err != nil {
		t.Fatalf("CycleStatus: %v", err)
	}
	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	raw, err := os.ReadFile(s.Path())
	if err != nil {
		t.Fatalf("reading back: %v", err)
	}

	var decoded struct {
		Todos []map[string]any `json:"todos"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("decoding: %v", err)
	}

	todo := decoded.Todos[0]
	if todo["status"] != string(models.StatusDoing) {
		t.Errorf("status in file = %v, want %q", todo["status"], models.StatusDoing)
	}
	if todo["detail"] != "long form" {
		t.Errorf("detail in file = %v, want %q", todo["detail"], "long form")
	}
	if _, hasDone := todo["done"]; hasDone {
		t.Errorf("file still carries the legacy done flag: %v", todo)
	}
	if !strings.Contains(string(raw), "\n") {
		t.Error("file is not pretty-printed")
	}
}

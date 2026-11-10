package agents

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func targetPath(t *testing.T, home string) string {
	t.Helper()
	return filepath.Join(home, ".pi", "agent", "extensions", "lazytodo.ts")
}

func TestInstallWritesTheExtension(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	result, err := Install(Pi)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}

	want := targetPath(t, home)
	if result.Path != want {
		t.Errorf("Path = %q, want %q", result.Path, want)
	}
	if !result.Changed {
		t.Error("the first install should report a change")
	}

	written, err := os.ReadFile(want)
	if err != nil {
		t.Fatalf("reading the installed file: %v", err)
	}
	if !strings.Contains(string(written), "registerTool") {
		t.Errorf("installed file does not look like the extension: %q", string(written[:40]))
	}

	// A second install is a no-op.
	again, err := Install(Pi)
	if err != nil {
		t.Fatalf("second Install: %v", err)
	}
	if again.Changed {
		t.Error("a second install should report no change")
	}
	if again.Backup != "" {
		t.Errorf("a second install should not back anything up, got %q", again.Backup)
	}
}

// TestInstallKeepsALocalEdit is the promise that a hand-edited extension is never
// silently thrown away.
func TestInstallKeepsALocalEdit(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	path := targetPath(t, home)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte("// my hand edit\n"), 0o644); err != nil {
		t.Fatalf("seeding: %v", err)
	}

	result, err := Install(Pi)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}

	if result.Backup == "" {
		t.Fatal("the previous file should have been backed up")
	}

	backup, err := os.ReadFile(result.Backup)
	if err != nil {
		t.Fatalf("reading the backup: %v", err)
	}
	if string(backup) != "// my hand edit\n" {
		t.Errorf("backup = %q, want the previous content", string(backup))
	}
}

func TestFor(t *testing.T) {
	if _, ok := For("pi"); !ok {
		t.Error("pi should be a known agent")
	}
	if _, ok := For("nope"); ok {
		t.Error("unknown agents must not resolve")
	}
	if len(All()) == 0 {
		t.Error("All() is empty")
	}
}

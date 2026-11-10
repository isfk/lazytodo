# lazytodo

A minimal TUI todo list built the way lazygit is: list on the left, detail on the right, status filter on the title row. ~3400 lines.

[中文](README.md) | **English**

## Install

```bash
go install github.com/isfk/lazytodo@latest   # use @main if there is no tag yet
```

Requires Go 1.26+. Or `git clone` and `go run .`.

## Usage

Todos live in `.lazytodo.json` **in the current directory** — another directory means another list. A missing file is an empty list; the file appears on the first change.

```
╭─Todos─All - Doing - Done───╮╭─Detail────────────────────────╮
│ ✓ write the store          ││atomic rename                   │
│ ◐ write the gui            ││                                │
│ ○ walk the dog             ││                                │
╰────────────────────────────╯╰────────────────────────────────╯
1/3 done · 1 doing
[␣] Cycle [a] Add [e] Edit [d] Delete [/] Filter
```

| Key / action | What it does |
|---|---|
| `↓` `↑` / `j` `k` | Move |
| `g` `G` | Jump to first / last |
| `space` | Cycle todo → doing → done |
| `a` / `e` | Add / edit (Enter in the title submits, the description is multi-line, `^s` submits) |
| `d` | Delete (asks first; only `y` goes through) |
| `/` | Text filter (matches title and description) |
| `⇥` `⇧⇥` / click a tab | Status filter: All → Doing → Done |
| `l` `h` | Enter / leave the detail pane (`y` there copies the selected line) |
| `esc` | Clear filters |
| `ctrl+d` / `q` | Reload / quit |

The interface language follows `LC_ALL` → `LC_MESSAGES` → `LANGUAGE` → `LANG` (Chinese or English). Mixed CJK/Latin text is aligned and truncated by display width, so a Chinese character takes two cells.

## Layout

| Package | Role |
|---|---|
| `pkg/commands` | Data layer: JSON store + atomic write, no UI dependencies |
| `pkg/gui/types` `context` `controllers` | Pane state, key and click handlers |
| `pkg/gui/presentation` | Pure functions from model to display rows (alignment, truncation, styles) |
| `pkg/tui` + `tcelldriver` | Terminal abstraction (View / Screen / FakeScreen); only the driver imports tcell |
| `pkg/gui` | Wiring: layout, refresh, key dispatch |
| `pkg/i18n` | Interface strings (English / Chinese) |
| `pkg/integration` | Headless integration test DSL |

Data flows one way: a key or a click mutates the store, `Refresh()` rebuilds every pane, and the views are drawn from that. Nothing pushes content into a view from the side.

## Tests

```bash
go test ./...   # 97 tests: commands / presentation / tui / driver / headless integration
```

The only dependency is `github.com/gdamore/tcell/v3`, plus the width table it already uses (`displaywidth`).

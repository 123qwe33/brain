# Brain — Agent Guide

## Orientation

`brain` opens `brain.db` from the **current directory** — always `cd ~/code/brain` first.

```bash
brain query                      # all IN nodes (your starting point)
brain query <label>              # content of a specific node
brain relations <label>          # outgoing edges
brain relations <label> --in     # incoming edges
brain graph <label> --depth 2    # BFS neighborhood as JSON
```

## Mental model

Brain implements a **Reason Maintenance System**. Nodes are either `IN` (true) or `OUT` (false). A node can be *justified* by other nodes — if a justifier goes `OUT`, dependents cascade `OUT` too. This means stale facts automatically invalidate downstream conclusions.

Use `brain query` to understand what is currently known before making assumptions about the project.

## Codebase layout

| Path | Role |
|---|---|
| `cmd/brain/main.go` | All cobra commands, wired to RMS methods |
| `cmd/brain/serve.go` | `brain serve` — embedded D3.js web viewer |
| `cmd/brain/viewer.html` | Graph UI (embedded via `//go:embed`) |
| `internal/rms/rms.go` | Engine: Assert, Justify, Invalidate, Query, Link, Relations, Graph |
| `internal/db/db.go` | Schema, WAL init, migrations, event log helpers |
| `internal/memoryagent/` | Experimental LLM integration (brain-ask/brain-remember) |

To add a new command: add the `*cobra.Command` in `main.go`, register it in `init()`, implement the method on `*rms.RMS` in `internal/rms/rms.go`, and append an event via `r.appendEvent(...)` for audit log consistency.

## Label conventions

| Prefix | Meaning |
|---|---|
| `proj_` | The project itself |
| `comp_` | A component |
| `iface_` | Interface between components |
| `dec_` | A design decision |
| `fact_` | A verified fact |
| `hyp_` | A hypothesis |
| `task_` | A task (invalidate when complete) |
| `sys_` | Infrastructure or runtime |

## Updating the brain DB

After architectural changes, reflect them in the graph:

```bash
brain assert <label> <type> "<content>"   # new or updated fact
brain invalidate <old_label>              # stale fact
```

Checkpoint WAL before committing `brain.db`:

```bash
python3 -c "import sqlite3; c=sqlite3.connect('brain.db'); c.execute('PRAGMA wal_checkpoint(TRUNCATE)'); c.commit(); c.close()"
```

## Testing

`test_brain.sh` runs a destructive end-to-end smoke test — it deletes `brain.db` in the working directory. Run it from a temp directory with a copy of the binary, not from the repo root.

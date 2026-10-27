# Contributing to Brain

## Prerequisites

- Go 1.24+
- GCC (required for cgo — `go-sqlite3` uses cgo)

## Build

```bash
git clone https://github.com/123qwe33/brain
cd brain
go build -o brain ./cmd/brain
```

## Test

`test_brain.sh` is a destructive smoke test — it deletes `brain.db` in the working directory. Run it from a temp directory:

```bash
mkdir /tmp/brain-test
cp brain test_brain.sh /tmp/brain-test/
cd /tmp/brain-test && ./test_brain.sh
```

## Code layout

| Path | Purpose |
|---|---|
| `cmd/brain/main.go` | All CLI commands (cobra) |
| `cmd/brain/serve.go` | Web viewer HTTP server |
| `cmd/brain/viewer.html` | Embedded D3.js graph UI |
| `internal/rms/rms.go` | RMS engine logic |
| `internal/db/db.go` | SQLite schema, init, WAL, migrations |
| `internal/memoryagent/` | Experimental LLM integration |

## Adding a command

1. Add the `*cobra.Command` in `cmd/brain/main.go`
2. Register it in `init()` with `rootCmd.AddCommand(...)`
3. Implement the method on `*rms.RMS` in `internal/rms/rms.go`
4. Append an event via `r.appendEvent(...)` for audit log consistency

## Commit style

Conventional commits: `feat:`, `fix:`, `chore:`, `docs:`. Subject line under 72 characters. If you add a command, include a brief usage example in the PR description.

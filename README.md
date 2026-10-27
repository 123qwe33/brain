# Brain

A Reason Maintenance System (RMS) for project knowledge. Store facts as a truth-tracked graph — nodes go `IN` when asserted, `OUT` when invalidated, with truth cascading through dependency chains.

## Concept

Three primitives:

- **Nodes** — facts, decisions, tasks, components
- **Edges** — directed relationships (`link`/`unlink`)
- **Justifications** — truth dependencies: a node is `IN` only while its justifiers are also `IN`

When a foundational fact is invalidated, anything that depends on it cascades `OUT`. This keeps a project's working knowledge consistent as understanding evolves.

## Installation

Requires GCC (for cgo — `go-sqlite3` uses cgo).

```bash
go install github.com/123qwe33/brain/cmd/brain@latest
```

Or build from source:

```bash
git clone https://github.com/123qwe33/brain
cd brain
go build -o brain ./cmd/brain
```

## Usage

`brain` opens `brain.db` in the **current directory** — always run from your project root (or `cd` there first).

### Commands

| Command | Description |
|---|---|
| `brain assert <label> <type> "<content>"` | Create or update a node |
| `brain query` | List all IN nodes |
| `brain query <label>` | Get a node's content |
| `brain query --type <type>` | Filter IN nodes by type |
| `brain link <from> <to> <relation>` | Add a directed edge |
| `brain unlink <from> <to> <relation>` | Remove a directed edge |
| `brain relations <label>` | Show outgoing edges |
| `brain relations <label> --in` | Show incoming edges |
| `brain justify <node> <dep> [dep...]` | Add a justification group |
| `brain invalidate <label>` | Mark node OUT; cascades to dependents |
| `brain graph <label>` | BFS neighborhood as JSON |
| `brain log` | Recent event history |
| `brain serve` | Launch local D3.js graph viewer |

### Quick example

```bash
# Build a project map
brain assert proj_myapp project "A Go web API"
brain assert comp_db component "PostgreSQL layer"
brain assert dec_postgres decision "Use PostgreSQL for relational storage"

brain link comp_db proj_myapp "part-of"
brain justify dec_postgres proj_myapp

# Query what's known
brain query
brain query proj_myapp

# A decision changes — cascade it out
brain invalidate dec_postgres
brain query  # dec_postgres is now OUT
```

### Justification logic

`brain justify` creates an AND-group: the node stays `IN` only while **all** justifiers in the group are `IN`. Multiple `justify` calls create OR-groups — the node stays `IN` if **any** group is fully satisfied.

```bash
brain assert A fact "A"
brain assert B fact "B"
brain assert C fact "C"

brain justify A B C   # A is IN only while both B and C are IN
brain invalidate B    # A cascades OUT
```

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

## Committing brain.db

`brain.db` is designed to be committed alongside code — the graph serves as living documentation. Before committing, checkpoint the WAL to ensure the file is self-contained:

```bash
python3 -c "import sqlite3; c=sqlite3.connect('brain.db'); c.execute('PRAGMA wal_checkpoint(TRUNCATE)'); c.commit(); c.close()"
```

Keep `.db-wal` and `.db-shm` in `.gitignore`.

## LLM integration (experimental)

`brain-ask` and `brain-remember` are companion CLIs that let an LLM query and write to the graph from plain English. They support Anthropic and OpenAI-compatible endpoints. Both are work in progress — direct use of the `brain` CLI is more reliable in practice.

## License

[MIT](LICENSE)

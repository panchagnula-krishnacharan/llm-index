# llm-index

A CLI tool that indexes and resumes LLM coding sessions across multiple tools — OpenCode, Zed, and Codex CLI — from a single terminal interface.

Sessions are imported into a local PostgreSQL database with full-text search. A minimalist TUI lets you browse, preview, and resume any session with one keypress.

## Features

- **Unified session index** — Aggregates sessions from OpenCode (SQLite), Zed (Markdown), and Codex CLI (JSON) into one searchable store
- **Full-text search** — PostgreSQL `tsvector` indexing across session titles, models, and directories
- **Table TUI** — Browse sessions in a sortable table with columns: Time, Type, Title, Model, Messages
- **Session preview** — View conversation messages inline before resuming
- **One-key resume** — Press Enter to launch the native tool with the selected session
- **CLI search** — Search sessions without opening the TUI
- **Homebrew distribution** — Install via `brew tap` after release

## Requirements

- Go 1.21+
- PostgreSQL (local or Docker)
- One or more supported LLM tools installed

## Setup

### 1. Start PostgreSQL

```bash
docker run -d --name llm-index-pg \
  -e POSTGRES_DB=llm_index \
  -e POSTGRES_PASSWORD=postgres \
  -e POSTGRES_HOST_AUTH_METHOD=trust \
  -p 5432:5432 \
  postgres:16-alpine
```

### 2. Build

```bash
git clone https://github.com/panchagnula-krishnacharan/llm-index.git
cd llm-index
go build -o llm-index ./cmd/main.go
```

### 3. Configure

```bash
export LLM_INDEX_DSN="postgres://postgres@localhost:5432/llm_index?sslmode=disable"
```

### 4. Run migrations

```bash
./llm-index migrate
```

### 5. Import sessions

```bash
./llm-index sync
```

## Usage

### Interactive TUI (default)

```bash
./llm-index
```

| Key     | Action                                      |
|---------|---------------------------------------------|
| `↑`/`↓` | Navigate sessions                          |
| `Enter` | Resume selected session in its native tool  |
| `p`     | Preview conversation messages               |
| `Esc`   | Close preview                               |
| `q`     | Quit                                        |

### CLI search

```bash
./llm-index search "terraform"
```

### Sync sessions

```bash
./llm-index sync
```

### Check version

```bash
./llm-index version
```

## Supported tools

| Tool       | Session location                              | Format   | Resume command         |
|------------|-----------------------------------------------|----------|------------------------|
| OpenCode   | `~/.local/share/opencode/opencode.db`         | SQLite   | `opencode -s <id>`     |
| Zed        | `~/.local/share/zed/conversations/*.md`       | Markdown | `zed <file>`           |
| Codex CLI  | `~/.codex/**/*.json`                          | JSON     | `codex --resume <path>`|

## Project structure

```
llm-index/
├── cmd/
│   └── main.go                    # CLI entrypoint — sync, search, migrate, version, TUI
├── internal/
│   ├── db/
│   │   ├── db.go                  # PostgreSQL operations (connect, upsert, search, list)
│   │   └── migrations.sql         # Embedded schema (sessions, messages, FTS trigger)
│   ├── importer/
│   │   ├── sync.go                # Orchestrator — runs all importers
│   │   ├── opencode.go            # Reads OpenCode SQLite DB (sessions, messages, parts)
│   │   ├── zed.go                 # Parses Zed markdown conversations with YAML frontmatter
│   │   └── codex.go               # Parses Codex CLI JSON session files
│   └── tui/
│       └── tui.go                 # Bubbletea table TUI with preview and resume
├── migrations/
│   └── 001_init.sql               # Reference SQL (also embedded in db package)
├── .github/
│   └── workflows/
│       └── release.yaml           # GitHub Actions — GoReleaser on tag push
├── .goreleaser.yaml               # Build matrix, archives, Homebrew tap formula
├── go.mod
└── go.sum
```

## Database schema

```
sessions
├── id (UUID)
├── source (opencode | zed | codex)
├── source_id
├── title
├── model
├── provider
├── directory
├── started_at / updated_at
├── message_count
├── resume_cmd
└── search_vector (tsvector, auto-updated via trigger)

messages
├── id (UUID)
├── session_id → sessions.id
├── role (user | assistant)
├── content
├── seq
└── created_at
```

## Releasing

Releases are automated via GoReleaser. Tag and push:

```bash
git tag v0.1.0
git push origin v0.1.0
```

This builds binaries for linux/darwin (amd64 + arm64), creates GitHub releases, and publishes a Homebrew formula.

### Homebrew install (after first release)

```bash
brew tap panchagnula-krishnacharan/tap
brew install llm-index
```

## Contributing

1. Fork the repository
2. Create a feature branch: `git checkout -b feat/my-feature`
3. Make your changes — keep diffs minimal and focused
4. Test locally:
   ```bash
   go build -o llm-index ./cmd/main.go
   ./llm-index migrate
   ./llm-index sync
   ./llm-index
   ```
5. Commit with a clear message: `git commit -m "add: description of change"`
6. Push and open a pull request

### Adding a new importer

1. Create `internal/importer/<tool>.go` with a `Sync<Tool>(pool *pgxpool.Pool) (int, error)` function
2. Call it from `SyncAll` in `internal/importer/sync.go`
3. Set the `resume_cmd` field to the command that opens the native tool with that session

### Guidelines

- Keep changes surgical — only modify what's needed
- No CGO — use pure-Go dependencies (e.g. `modernc.org/sqlite` instead of `mattn/go-sqlite3`)
- Match existing code style
- Test the full flow: `migrate` → `sync` → TUI browse → resume

## License

MIT

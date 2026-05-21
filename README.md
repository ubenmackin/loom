# Loom

An agent-first JIT Kanban board. Loom provides a real-time Kanban board designed around how AI coding agents actually work — managing stories, tasks, dependencies, and agent sessions with WebSocket-first communication.

## Tech Stack

- **Language:** Go 1.25.8
- **HTTP Router:** [chi](https://github.com/go-chi/chi)
- **Database:** SQLite via [go-sqlite3](https://github.com/mattn/go-sqlite3) (CGO required)
- **WebSocket:** [gorilla/websocket](https://github.com/gorilla/websocket)
- **ID Generation:** [google/uuid](https://github.com/google/uuid)

## Quick Start

```bash
make build && make run
```

The server starts on `http://localhost:8080` by default.

## Configuration

| Flag       | Default     | Description                                    |
|------------|-------------|------------------------------------------------|
| `--db-path`| `loom.db`   | Path to SQLite database file                   |
| `--port`   | `8080`      | HTTP server port                               |
| `--web-dir`| `web/dist`  | Path to frontend static files                  |
| `--mcp`    | `false`     | Run as MCP server on stdio instead of HTTP     |

## Make Targets

| Target     | Description                              |
|------------|------------------------------------------|
| `build`    | Compile server binary to `dist/`         |
| `run`      | Run the server directly                  |
| `migrate`  | Placeholder (migrations run on startup)  |
| `test`     | Run all Go tests                         |
| `lint`     | Run `go vet`                             |
| `clean`    | Remove build artifacts                   |

## License

MIT

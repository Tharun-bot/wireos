# WireOS — Personal Data Federation for AI Agents

WireOS gives AI agents authenticated, structured access to your entire digital life via a single intent.

## Architecture

- **Layer 1 — Intent Router (Go):** Maps natural language or structured intents to Wire action IDs
- **Layer 2 — Parallel Wire Executor (Go):** Fans out actions concurrently via goroutines with graceful partial failure handling
- **Layer 3 — Result Normalizer:** Maps heterogeneous Wire responses to canonical schemas

## Stack

| Component | Tech |
|-----------|------|
| Backend   | Go 1.22, stdlib only |
| MCP       | TypeScript / Node |
| Frontend  | HTML/CSS/JS (no framework) |
| Container | Docker + Compose |

## Quickstart

```bash
cp .env.example .env
# fill in your ANAKIN_API_KEY

cd backend && go run .
# or
docker-compose up
```

## Endpoints

| Endpoint | Description |
|----------|-------------|
| `GET /health` | Health check |
| `GET /metrics` | Prometheus metrics |
| `POST /intent` | _(coming in Task 2)_ Query across all your accounts |
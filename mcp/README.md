# WireOS — Personal Data Federation for AI Agents

WireOS is a unified intent layer built on the Wire API (by Anakin). It exposes
a clean Go backend that maps natural-language intents — purchases, professional
activity, GitHub commits, portfolio, job applications, web research — to Wire's
agentic action executor. Responses from heterogeneous platforms (Amazon,
LinkedIn, GitHub, Robinhood) are normalised into consistent typed schemas by a
per-output-type normalizer, so any consumer gets the same shape regardless of
source.

The project is self-hostable in one command. Without an API key it runs in demo
mode and returns realistic mock data for every intent, giving the full UI
experience out of the box. Set `ANAKIN_API_KEY` to switch to live mode and fan
real Wire actions out in parallel across your connected accounts.

```
┌──────────────────┐   POST /intent    ┌─────────────────────────────────────────┐
│                  │ ────────────────▶ │           Go Backend (:8081)            │
│  Frontend        │                   │                                         │
│  (nginx :80)     │   JSON response   │  Intent Router → Executor (goroutines)  │
│                  │ ◀──────────────── │         │              │                │
└──────────────────┘                   │    Normalizer     Wire API Client        │
                                       └───────────────────────┬─────────────────┘
        GET /catalog  ──────────────────────────────────────── │
        GET /health   ──────────────────────────────────────── │
        GET /metrics  ──────────────────────────────────────── │
                                                               ▼
                                              https://api.anakin.io/v1/wire
                                                               │
                       ┌───────────────┬───────────────┬───────────────┬──────────────┐
                       │               │               │               │              │
                    Amazon         LinkedIn         GitHub        Robinhood        Anakin
                  (purchases)    (activity)       (commits)     (portfolio)       (search)
```

## Quick Start

```bash
git clone https://github.com/Tharun-bot/wireos.git && cd wireos
cp .env.example .env          # add ANAKIN_API_KEY for live mode (optional)
docker-compose up --build
```

Frontend → http://localhost:3000 · Backend → http://localhost:8081

## Supported Intents

| Intent ID               | Label                | Description                                            | Output Type  |
|-------------------------|----------------------|--------------------------------------------------------|--------------|
| `recent_purchases`      | Recent Purchases     | Fetch recent Amazon orders and spend                   | transaction  |
| `professional_activity` | Professional Activity| LinkedIn posts, connections, and profile summary       | activity     |
| `github_activity`       | GitHub Activity      | Recent commits, pushes, and open pull requests         | activity     |
| `portfolio_snapshot`    | Portfolio Snapshot   | Current holdings, positions, and day P&L               | generic      |
| `job_applications`      | Job Applications     | Applied jobs and their status; saved jobs list         | activity     |
| `web_research`          | Web Research         | Agentic web search on any topic (requires user query)  | generic      |

## Environment Variables

| Variable         | Required | Description                                  |
|------------------|----------|----------------------------------------------|
| `ANAKIN_API_KEY` | No       | Wire API key — omit to run in demo mode      |
| `PORT`           | No       | Backend listen port (default `8081`)         |

## Endpoints

| Endpoint        | Method | Description                             |
|-----------------|--------|-----------------------------------------|
| `/health`       | GET    | Version + liveness check                |
| `/metrics`      | GET    | Prometheus-format request counter       |
| `/catalog`      | GET    | Intents list + Wire sites + mode        |
| `/intent`       | POST   | Execute an intent, returns results JSON |

## Deployment

**Backend → Fly.io (free tier, Singapore)**
```bash
fly auth login
fly launch --no-deploy
fly secrets set ANAKIN_API_KEY=your_key
fly deploy
```

**Frontend → GitHub Pages**

In repo Settings → Pages → source: `main` branch → `/frontend` folder.
Update `API_BASE` in `app.js` to your Fly.io URL before pushing.

## Live Demo

🔗 _https://wireos-backend.fly.dev_ (backend)
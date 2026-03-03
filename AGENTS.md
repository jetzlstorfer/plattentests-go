# Agent Instructions

This file provides guidance for AI coding agents (e.g., GitHub Copilot, Claude, Gemini) working in this repository.

## What this project does

This Go application crawls weekly music album highlight reviews from [Plattentests.de](https://www.plattentests.de) and builds a Spotify playlist from the highlighted tracks. A Gin-based web UI lets users browse the current highlights and trigger playlist creation.

## Repository layout

```
plattentests-go/
├── cmd/
│   ├── crawler/        # Fetches album reviews & highlight tracks from Plattentests.de
│   ├── creator/        # Searches Spotify and adds tracks to a playlist
│   └── token/          # Generates & stores Spotify OAuth2 token in Azure Blob Storage
├── internal/
│   └── auth/           # Spotify OAuth2 + Azure Blob Storage token management
├── webui/              # Gin web server (port 8081), HTML templates, static assets
├── .devcontainer/      # Dev container configuration for VS Code / Codespaces
├── .github/
│   └── workflows/      # GitHub Actions: build/deploy to Azure Container Apps, CodeQL
├── env                 # Template for .env file — copy and fill in credentials
├── Makefile            # Build/run targets
├── go.mod / go.sum     # Go module definition
└── README.md           # Project overview and usage
```

## How to build and test

```bash
# Run all tests
go test ./...

# Build the web UI
cd webui && go build ./...

# Run the web UI locally (requires a populated .env file)
make web

# Generate a Spotify OAuth2 token and upload it to Azure
make token
```

## Environment variables

Copy `env` to `.env` and populate the following variables before running:

| Variable         | Description                                      |
|------------------|--------------------------------------------------|
| `SPOTIFY_ID`     | Spotify application client ID                    |
| `SPOTIFY_SECRET` | Spotify application client secret                |
| `PLAYLIST_ID`    | Spotify playlist ID (target / dev playlist)      |
| `PLAYLIST_ID_PROD` | Spotify playlist ID (production playlist)      |
| `AZ_ACCOUNT`     | Azure Storage account name                       |
| `AZ_KEY`         | Azure Storage account key                        |
| `AZ_CONTAINER`   | Azure Blob container name                        |
| `TOKEN_FILE`     | Blob filename for the OAuth2 token (default: `token.txt`) |

## Coding conventions

- Standard Go style; follow `gofmt` formatting
- Table-driven tests using the `testing` package (see `cmd/creator/sanitize_test.go` for examples)
- ISO-8859-1 → UTF-8 decoding is required when handling text from Plattentests.de
- Levenshtein distance (threshold 0.8) is used for fuzzy artist/track name matching
- Configuration is injected via environment variables using `github.com/kelseyhightower/envconfig`

## Important notes for agents

- `cmd/crawler` and `cmd/creator` are **not** `main` packages; they export functions consumed by `webui/main.go`
- Do **not** commit secrets or credentials; use environment variables only
- New tests should follow the table-driven pattern already in use
- The Docker image is built from `webui/Dockerfile` and deployed to Azure Container Apps via CI/CD

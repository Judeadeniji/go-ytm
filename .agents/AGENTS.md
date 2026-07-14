# AGENTS.md — YouTube Music Terminal Client

> Working name: `go-ytm`. Rename freely — nothing below depends on the name.

This file orients any coding agent (or future you) working on this repo. Read it before making structural changes.

## What this is

A terminal-based YouTube Music client written in Go. Target feature set: streaming playback, background playback, download/offline caching, silence-skip, sleep timer, audio normalization, tempo/pitch control, EQ, synced lyrics, personalized quick picks, full search (songs/albums/artists/videos/playlists), local + synced library management, playlist import/reorder, and YouTube Music account login.

## Architecture decision (read this first)

The system is split into two processes on purpose:

1. **Go core (this repo, majority of the code)** — TUI, playback control, search/extraction, local library, caching, lyrics.
2. **Python sidecar (`sidecar/`, small, isolated)** — wraps `ytmusicapi` to handle account login, library sync, and personalized quick picks, exposed as a tiny localhost JSON API.

**Why the split exists — do not "simplify" it away:**
YouTube Music has no public API. Account login, library sync, and quick picks only work by replaying browser auth headers against Google's internal InnerTube endpoints. The Python library `ytmusicapi` (sigma67) is the mature, community-hardened implementation of this. The Go equivalent is a small, thinly-maintained port. Reimplementing this in Go means owning YouTube's internal API reverse-engineering ourselves — high maintenance cost, no upside. The sidecar isolates that fragility behind an HTTP boundary so it can be patched or replaced without touching the Go core.

If an agent is asked to "port the sidecar to Go for consistency," push back and link this section unless the Go ytmusicapi port has since become genuinely mature — check its commit activity first.

## Tech stack

| Concern | Choice | Notes |
|---|---|---|
| TUI | `charmbracelet/bubbletea` + `lipgloss` | Elm-style update loop; keep views pure functions of state |
| Playback | `mpv --idle --input-ipc-server=<socket>` | Controlled via JSON IPC over a unix socket, not a Go audio library |
| Search / extraction | `github.com/kkdai/youtube` | Primary. Fallback: shell out to `yt-dlp` if extraction breaks on a format change |
| Local DB | `modernc.org/sqlite` | Pure Go, no cgo — keeps cross-compilation simple |
| Lyrics | LRCLIB REST API | Plain HTTP, no auth |
| Account/library/quick picks | Python sidecar wrapping `ytmusicapi` | Localhost-only HTTP, never exposed externally |

## Repo layout

```
/cmd/ytm-tui/          entrypoint
/internal/player/      mpv IPC client (connect, load, seek, filters, sleep timer)
/internal/search/      kkdai/youtube wrapper + yt-dlp fallback
/internal/library/     sqlite-backed local playlists, queue, download cache index
/internal/lyrics/      LRCLIB client
/internal/ytmapi/         thin HTTP client for the Python ytm-api
/internal/tui/         bubbletea models/views (now-playing, queue, search, library)
/ytm-api/              Python process, ytmusicapi-based, FastAPI or plain http.server
/downloads/            cached audio files (gitignored)
```

## Conventions

- **No cgo** outside the mpv subprocess boundary. Keep builds a single static binary.
- **mpv is the only audio engine.** Don't add a second playback backend "for flexibility" — it doubles the surface area for EQ/normalize/tempo work that mpv already does via filters (`af=loudnorm`, `rubberband`).
- **kkdai/youtube first, yt-dlp fallback second.** Don't invert this — subprocess calls are slower and add a runtime dependency.
- **All ytm-api calls go through `internal/ytmapi`.** No direct HTTP calls to the ytm-api scattered through the codebase — one client, one place to add retry/backoff when Google inevitably changes something.
- **Bubbletea models stay pure.** Side effects (mpv commands, HTTP calls) happen in `tea.Cmd`s, not inline in `Update`.
- **Errors from the sidecar are expected, not exceptional.** Auth/library calls will periodically break upstream. Surface these as a visible but non-fatal TUI state ("library sync unavailable"), never a crash.
- **Use `make` for all tooling tasks.** Do not run raw `go build`, `go test`, `go fmt`, or `go mod` commands manually. Always use the provided `Makefile` targets (`make build`, `make test`, `make lint`, `make tidy`, `make run`) to ensure required environment variables like `CGO_ENABLED=0` and proper build flags are applied.

## Build order (for new work / new agents picking this up)

1. mpv IPC wrapper + basic playback
2. Search (kkdai/youtube) + bubbletea shell
3. Local playlists/queue/download cache — no account needed
4. mpv filter config: EQ, normalization, tempo/pitch, silence-skip
5. Lyrics via LRCLIB
6. Python ytm-api: login, library sync, quick picks

Later steps depend on earlier ones being stable. Don't start the sidecar work before steps 1–3 are solid — it's the highest-risk, most-likely-to-break piece and easiest to build/debug last.

## Known fragility (don't be surprised by this)

- Sidecar auth headers expire/rotate; expect periodic re-auth flow breakage.
- `kkdai/youtube` extraction can break when YouTube changes player signatures — keep the yt-dlp fallback path actually tested, not just present.
- Legal/ToS note: this uses unofficial extraction and auth-replay against YouTube Music, which violates YouTube's ToS. Personal-use tool, not a distributable product.

## Testing

- `internal/player`, `internal/library`, `internal/lyrics` should have unit tests with mpv/HTTP mocked.
- `internal/search` and `sidecar/` need integration tests but expect them to be the flakiest in CI (they hit real or near-real external behavior).

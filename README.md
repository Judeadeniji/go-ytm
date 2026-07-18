# ytm

Terminal YouTube Music client — Bubble Tea TUI, mpv playback, local Python API (`ytmusicapi`).

## Install (recommended)

```bash
curl -fsSL https://raw.githubusercontent.com/Judeadeniji/go-ytm/main/scripts/install.sh | bash
```

Then:

```bash
ytm doctor   # verify mpv, python3, API files
ytm          # launch (first run creates the API venv)
```

Pin a release:

```bash
VERSION=v0.1.0 bash -c 'curl -fsSL https://raw.githubusercontent.com/Judeadeniji/go-ytm/main/scripts/install.sh | bash'
```

Ensure `~/.local/bin` is on your `PATH`.

### Requirements

| Dependency | Required | Notes |
|---|---|---|
| **mpv** | yes | Audio engine (IPC) |
| **python3** | yes | Runs the local `ytm-api` |
| **curl** | yes | Installer / health checks |
| **yt-dlp** | recommended | Fallback for some streams |

```bash
# Arch
pacman -S mpv python yt-dlp

# Debian/Ubuntu
sudo apt install mpv python3 yt-dlp

# Fedora
sudo dnf install mpv python3 yt-dlp
```

Kitty (or another terminal with Kitty graphics) improves cover art; plain terminals still work.

### First-run auth

1. Start `ytm`
2. Open **Settings**
3. Paste request headers or complete OAuth

Auth is stored at `~/.local/state/go-ytm/headers_auth.json`.

In-app shortcuts: press **`?`**.

## From source

```bash
git clone https://github.com/Judeadeniji/go-ytm.git
cd go-ytm
make install          # → ~/.local/bin/ytm + share/go-ytm/ytm-api
ytm doctor && ytm
```

Uninstall:

```bash
make uninstall
```

### Development

```bash
make run              # builds bin/ytm, sets YTM_DEV=1, uses ./ytm-api
make test
make build
```

## How it works

```
ytm (Go)
  ├─ starts/reuses ytm-api over a Unix socket
  ├─ talks to YouTube Music via ytmusicapi
  └─ plays audio through mpv IPC
```

| Path | Purpose |
|---|---|
| `~/.local/bin/ytm` | CLI |
| `~/.local/share/go-ytm/ytm-api` | Python API + venv |
| `~/.local/state/go-ytm/` | socket, token, logs, library DB, auth |

### Environment

| Variable | Meaning |
|---|---|
| `YTM_API_HOME` | Override API directory |
| `YTM_API_SOCK` | Override Unix socket path |
| `YTM_API_TOKEN` | Override API auth token |
| `YTM_DEV=1` | Dev mode (repo `./ytm-api`, logs in `tmp/`) |

## Releases

Tagged versions (`v*`) publish Linux `amd64` / `arm64` archives via GoReleaser (binary + `ytm-api/`).

```bash
git tag v0.1.0
git push origin v0.1.0
```

## License

See repository for license terms.

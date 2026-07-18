# ytm

YouTube Music in the terminal. Go TUI (Bubble Tea), mpv for audio, and a small local Python API (`ytmusicapi`) that `ytm` starts for you.

Linux only for now (`amd64` / `arm64`). Still prerelease — expect rough edges.

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/Judeadeniji/go-ytm/main/scripts/install.sh | bash
```

Puts the binary in `~/.local/bin/ytm` and the API tree under `~/.local/share/go-ytm/ytm-api`. Make sure `~/.local/bin` is on your `PATH`.

Then:

```bash
ytm doctor   # mpv, python, API files, socket health
ytm          # first launch creates the API venv (can take a minute)
```

### Dependencies


|             |                               |
| ----------- | ----------------------------- |
| **mpv**     | required — playback           |
| **python3** | required — local `ytm-api`    |
| **yt-dlp**  | recommended — stream fallback |


```bash
# Arch
pacman -S mpv python yt-dlp

# Debian / Ubuntu
sudo apt install mpv python3 yt-dlp

# Fedora
sudo dnf install mpv python3 yt-dlp
```

Cover art looks best in Kitty (or anything that speaks the Kitty graphics protocol). Other terminals just skip the images.

### From source

Needs Go 1.26+.

```bash
git clone https://github.com/Judeadeniji/go-ytm.git
cd go-ytm
make install
ytm doctor && ytm
```

```bash
make uninstall   # removes ~/.local/bin/ytm and the shared API tree
```

## Sign in

YouTube Music needs cookies / OAuth. In the TUI:

1. Open **Settings**
2. **Sign in (Browser Headers)** — paste request headers from a logged-in browser session, or
3. **Sign in (OAuth)** — point it at a Google `client_secret.json`

Credentials land in `~/.local/state/go-ytm/headers_auth.json`. If search/library stay empty, you’re probably not authed — run through Settings again.

## Using it

```
ytm              # start
ytm doctor       # diagnose
ytm --version
ytm --help
```

In-app, press `?` for shortcuts. A few worth knowing:


|               |                        |
| ------------- | ---------------------- |
| `/`           | search                 |
| `tab`         | cycle panes            |
| `p` / `space` | play / pause           |
| `n` / `b`     | next / previous        |
| `\`           | toggle queue rail      |
| `d`           | download focused track |
| `f`           | now-playing stage      |
| `q`           | quit                   |


Home, Explore, Library, Downloads, and Settings live in the sidebar. Downloads and library metadata stay under the state directory.

## Layout on disk

```
~/.local/bin/ytm
~/.local/share/go-ytm/ytm-api/     # Python app + venv
~/.local/state/go-ytm/
  ytm-api.sock
  ytm-api.log
  api.token
  headers_auth.json
  library.db
  downloads/
  tui.log
```

`ytm` opens a Unix socket to the API, waits for `/health`, then draws the UI. If something blows up on start, read `ytm-api.log` or run `ytm doctor`.


| Variable        |                                                                      |
| --------------- | -------------------------------------------------------------------- |
| `YTM_API_HOME`  | API directory (default: share path, or `./ytm-api` with `YTM_DEV=1`) |
| `YTM_API_SOCK`  | socket path                                                          |
| `YTM_API_TOKEN` | bearer token for the local API                                       |
| `YTM_DEV=1`     | use the repo’s `ytm-api`, log under `tmp/`                           |


## Development

```bash
make run     # build + YTM_DEV=1
make test
make lint
make build
```

`make run` builds `bin/ytm` and points the supervisor at `./ytm-api`. Don’t run the AppImage-hijacked `python3` from some IDEs when creating venvs — `ytm doctor` / the runner prefer `/usr/bin/python3`.

Releases: push a `v*` tag; GitHub Actions + GoReleaser publish Linux archives (binary + `ytm-api/` with `routers/` intact).

## Troubleshooting

**`ytm-api exited: exit status 1`**  
Check `~/.local/state/go-ytm/ytm-api.log`. Common causes: missing `routers/` (reinstall from a current release), broken venv (delete `~/.local/share/go-ytm/ytm-api/venv` and start again), or no usable system Python.

**Install 404 / no release**  
There may only be prereleases. Pin `VERSION=…` or wait for a stable tag — the installer falls back to the newest release including prereleases when `/releases/latest` is empty.

**No audio**  
`mpv` must be on `PATH`. `ytm doctor` will say if it isn’t.

**Empty library after “login”**  
Headers expired or incomplete. Re-auth from Settings.

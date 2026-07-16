"""
Shared dependencies for the ytm-api routers.

All routers import `ytmusic`, `_ytm_error`, and `_CATCH` from here.
`reload_ytmusic()` is called by the auth router after login to refresh the
authenticated instance without restarting the server.
"""
import os

from fastapi import HTTPException
from requests import RequestException
from ytmusicapi import YTMusic
from ytmusicapi.exceptions import YTMusicError, YTMusicUserError

AUTH_FILE = os.path.expanduser("~/.local/state/go-ytm/headers_auth.json")

# Global, mutable — replaced in-place by reload_ytmusic() after auth flows.
ytmusic: YTMusic = None  # type: ignore[assignment]  # initialised below


def _load_ytmusic() -> YTMusic:
    if os.path.exists(AUTH_FILE):
        try:
            return YTMusic(AUTH_FILE)
        except Exception:
            return YTMusic()
    return YTMusic()


def reload_ytmusic() -> None:
    """Replace the global ytmusic instance (called after successful auth)."""
    global ytmusic
    ytmusic = _load_ytmusic()


# Initialise at import time.
ytmusic = _load_ytmusic()


# ---------------------------------------------------------------------------
# Error helpers
# ---------------------------------------------------------------------------

_CATCH = (YTMusicError, YTMusicUserError, RequestException, ValueError, TypeError)


def _ytm_error(exc: Exception) -> HTTPException:
    """Map ytmusicapi / network failures to non-fatal HTTP errors for the TUI."""
    if isinstance(exc, YTMusicUserError):
        return HTTPException(status_code=400, detail=str(exc))
    if isinstance(exc, RequestException):
        return HTTPException(status_code=502, detail=f"YouTube Music upstream error: {exc}")
    return HTTPException(status_code=502, detail=str(exc))


# ---------------------------------------------------------------------------
# Misc helpers
# ---------------------------------------------------------------------------

def _normalize_playlist_id(playlist_id: str) -> str:
    """browseId for playlists is often VL + playlistId — strip the prefix."""
    if playlist_id.startswith("VL"):
        return playlist_id[2:]
    return playlist_id


def _format_secs(secs: int) -> str:
    if secs <= 0:
        return ""
    m, s = divmod(secs, 60)
    if m >= 60:
        h, m = divmod(m, 60)
        return f"{h}:{m:02d}:{s:02d}"
    return f"{m}:{s:02d}"

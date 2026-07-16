from fastapi import FastAPI, HTTPException, Query
from requests import RequestException
from ytmusicapi import YTMusic
from ytmusicapi.exceptions import YTMusicError, YTMusicUserError
from ytmusicapi.setup import setup
from pydantic import BaseModel
import os

app = FastAPI()
AUTH_FILE = os.path.expanduser("~/.local/state/go-ytm/headers_auth.json")

def load_ytmusic():
    if os.path.exists(AUTH_FILE):
        try:
            return YTMusic(AUTH_FILE)
        except Exception:
            return YTMusic()
    return YTMusic()

ytmusic = load_ytmusic()


def _ytm_error(exc: Exception) -> HTTPException:
    """Map ytmusicapi / network failures to non-fatal HTTP errors for the TUI."""
    if isinstance(exc, YTMusicUserError):
        return HTTPException(status_code=400, detail=str(exc))
    if isinstance(exc, RequestException):
        return HTTPException(status_code=502, detail=f"YouTube Music upstream error: {exc}")
    return HTTPException(status_code=502, detail=str(exc))


_CATCH = (YTMusicError, YTMusicUserError, RequestException, ValueError, TypeError)


def _normalize_playlist_id(playlist_id: str) -> str:
    # browseId for playlists is often VL + playlistId
    if playlist_id.startswith("VL"):
        return playlist_id[2:]
    return playlist_id


@app.get("/health")
def health():
    """Liveness check used by the make run bootstrap."""
    return {"ok": True, "authenticated": os.path.exists(AUTH_FILE)}

class AuthRequest(BaseModel):
    headers_raw: str

@app.post("/auth/setup")
def auth_setup(req: AuthRequest):
    try:
        os.makedirs(os.path.dirname(AUTH_FILE), exist_ok=True)
        setup(filepath=AUTH_FILE, headers_raw=req.headers_raw)
        global ytmusic
        ytmusic = load_ytmusic()
        return {"status": "ok"}
    except Exception as exc:
        raise HTTPException(status_code=400, detail=str(exc))

from ytmusicapi.auth.oauth import OAuthCredentials, RefreshingToken

class OAuthCodeRequest(BaseModel):
    client_id: str
    client_secret: str

@app.post("/auth/oauth/code")
def oauth_code(req: OAuthCodeRequest):
    cred = OAuthCredentials(req.client_id, req.client_secret)
    try:
        code = cred.get_code()
        return code
    except Exception as exc:
        raise HTTPException(status_code=400, detail=str(exc))

class OAuthTokenRequest(BaseModel):
    client_id: str
    client_secret: str
    device_code: str

@app.post("/auth/oauth/token")
def oauth_token(req: OAuthTokenRequest):
    cred = OAuthCredentials(req.client_id, req.client_secret)
    try:
        raw_token = cred.token_from_code(req.device_code)
    except Exception as exc:
        raise HTTPException(status_code=400, detail=str(exc))
    
    if "error" in raw_token:
        err = raw_token.get("error")
        if err == "authorization_pending":
            return {"status": "pending"}
        raise HTTPException(status_code=400, detail=err)

    refresh_token_expires_in = raw_token.get("refresh_token_expires_in", raw_token.get("expires_in", 0))
    ref_token = RefreshingToken(
        credentials=cred,
        access_token=raw_token.get("access_token", ""),
        refresh_token=raw_token.get("refresh_token", ""),
        scope=raw_token.get("scope", ""),
        token_type=raw_token.get("token_type", ""),
        expires_in=refresh_token_expires_in,
    )
    ref_token.update(raw_token)
    
    # Ensure directory exists before saving
    os.makedirs(os.path.dirname(AUTH_FILE), exist_ok=True)
    # _local_cache is a pathlib.Path or str ? 
    from pathlib import Path
    ref_token.local_cache = Path(AUTH_FILE)
    
    global ytmusic
    ytmusic = load_ytmusic()
    return {"status": "ok"}

@app.get("/library/playlists")
def library_playlists(limit: int = Query(50)):
    try:
        return {"playlists": ytmusic.get_library_playlists(limit=limit)}
    except _CATCH as exc:
        raise _ytm_error(exc) from exc

@app.get("/library/songs")
def library_songs(limit: int = Query(100)):
    try:
        return {"songs": ytmusic.get_library_songs(limit=limit)}
    except _CATCH as exc:
        raise _ytm_error(exc) from exc

@app.get("/library/albums")
def library_albums(limit: int = Query(100)):
    try:
        return {"albums": ytmusic.get_library_albums(limit=limit)}
    except _CATCH as exc:
        raise _ytm_error(exc) from exc

@app.get("/library/artists")
def library_artists(limit: int = Query(100)):
    try:
        return {"artists": ytmusic.get_library_artists(limit=limit)}
    except _CATCH as exc:
        raise _ytm_error(exc) from exc


from typing import Literal

_SearchFilterType = Literal[
    "songs",
    "videos",
    "albums",
    "artists",
    "playlists",
    "community_playlists",
    "featured_playlists",
    "profiles",
    "podcasts",
    "episodes",
]

@app.get("/search")
def search(
    q: str = Query(..., description="The search query"),
    filter: _SearchFilterType | None = Query(
        None,
        description="songs|videos|albums|artists|playlists|community_playlists|featured_playlists|profiles|podcasts|episodes",
    ),
    limit: int = Query(20, ge=1, le=100),
):
    """
    Search YouTube Music.
    Wraps ytmusicapi.search — resultType discriminates song/video/album/artist/playlist/…
    Album subtypes (Album/Single/EP) appear in the ``type`` field.
    """
    try:
        results = ytmusic.search(q, filter=filter, limit=limit)
    except _CATCH as exc:
        raise _ytm_error(exc) from exc
    return {"results": results}


@app.get("/suggestions")
def suggestions(q: str = Query(..., description="The search query")):
    """Get search suggestions with detailed runs (bold typed match + history flags)."""
    try:
        results = ytmusic.get_search_suggestions(q, detailed_runs=True)
    except _CATCH as exc:
        raise _ytm_error(exc) from exc
    # Normalize plain strings / drop nulls so clients can decode reliably.
    out = []
    for item in results or []:
        if isinstance(item, str):
            out.append({"text": item, "runs": [{"text": item}], "fromHistory": False})
            continue
        if not isinstance(item, dict):
            continue
        cleaned = {k: v for k, v in item.items() if v is not None}
        if "text" not in cleaned and "runs" in cleaned:
            cleaned["text"] = "".join(r.get("text", "") for r in cleaned.get("runs") or [])
        out.append(cleaned)
    return {"suggestions": out}


@app.get("/home")
def home(limit: int = Query(3, ge=1, le=20)):
    """Get the home page with dynamic carousels (includes Quick picks)."""
    try:
        results = ytmusic.get_home(limit=limit)
    except _CATCH as exc:
        raise _ytm_error(exc) from exc
    return {"carousels": results}


@app.get("/explore")
def explore():
    """Get explore page data."""
    try:
        result = ytmusic.get_explore()
    except _CATCH as exc:
        raise _ytm_error(exc) from exc
    return result


@app.get("/explore/moods")
def explore_moods():
    """Get mood categories."""
    try:
        result = ytmusic.get_mood_categories()
    except _CATCH as exc:
        raise _ytm_error(exc) from exc
    return {"moodCategories": result}


@app.get("/explore/moods/playlists")
def explore_mood_playlists(params: str = Query(...)):
    """Get playlists for a mood category."""
    try:
        result = ytmusic.get_mood_playlists(params)
    except _CATCH as exc:
        raise _ytm_error(exc) from exc
    return {"playlists": result}


@app.get("/explore/charts")
def explore_charts(country: str = Query("ZZ")):
    """Get charts data."""
    try:
        result = ytmusic.get_charts(country=country)
    except _CATCH as exc:
        raise _ytm_error(exc) from exc
    return result



@app.get("/artist/{channel_id}")
def artist(channel_id: str):
    """
    Artist page via get_artist(channelId).
    Sections (songs/albums/singles/videos/…) include results plus optional browseId/params.
    """
    try:
        result = ytmusic.get_artist(channel_id)
    except _CATCH as exc:
        raise _ytm_error(exc) from exc
    return result


@app.get("/artist/{channel_id}/albums")
def artist_albums(
    channel_id: str,
    params: str = Query(..., description="params from get_artist albums/singles/shows section"),
    limit: int = Query(100, ge=1, le=500),
):
    """Full album/single/show list via get_artist_albums(channelId, params)."""
    try:
        results = ytmusic.get_artist_albums(channel_id, params, limit=limit)
    except _CATCH as exc:
        raise _ytm_error(exc) from exc
    return {"albums": results}


@app.get("/album/browse-id")
def album_browse_id(
    audioPlaylistId: str = Query(..., description="audio playlist id starting with OLAK5uy_"),
):
    """Resolve OLAK5uy_* audioPlaylistId to MPREb_* album browseId."""
    try:
        browse_id = ytmusic.get_album_browse_id(audioPlaylistId)
    except _CATCH as exc:
        raise _ytm_error(exc) from exc
    if not browse_id:
        raise HTTPException(status_code=404, detail="album browseId not found")
    return {"browseId": browse_id}


@app.get("/album/{browse_id}")
def album(browse_id: str):
    """Album/Single/EP page via get_album(browseId). type field distinguishes Album/Single/EP."""
    try:
        result = ytmusic.get_album(browse_id)
    except _CATCH as exc:
        raise _ytm_error(exc) from exc
    return result


@app.get("/playlist/{playlist_id}")
def playlist(
    playlist_id: str,
    limit: int = Query(100, ge=1, le=500),
):
    """Playlist page via get_playlist. Strips VL prefix when present."""
    try:
        result = ytmusic.get_playlist(_normalize_playlist_id(playlist_id), limit=limit)
    except _CATCH as exc:
        raise _ytm_error(exc) from exc
    return result


@app.get("/watch")
def watch(
    videoId: str | None = Query(None),
    playlistId: str | None = Query(None),
    radio: bool = Query(False),
    limit: int = Query(25, ge=1, le=100),
):
    """
    Watch / radio queue via get_watch_playlist.
    At least one of videoId or playlistId is required.
    """
    if not videoId and not playlistId:
        raise HTTPException(status_code=400, detail="videoId or playlistId required")
    try:
        result = ytmusic.get_watch_playlist(
            videoId=videoId,
            playlistId=playlistId,
            limit=limit,
            radio=radio,
        )
    except _CATCH as exc:
        raise _ytm_error(exc) from exc
    return result


@app.get("/song/{video_id}")
def song(video_id: str):
    """
    Catalog-style song metadata for the Details rail.

    Composes ytmusicapi sources (not raw get_song videoDetails):
    - get_watch_playlist → title, artists[{name,id}], album[{name,id}], year, length
    - get_album (when album id known) → type, track #, explicit, creditsBrowseId
    - get_song_credits → performed/written/produced by
    - get_song only as a title/duration/author-name fallback
    """
    out: dict = {
        "videoId": video_id,
        "title": "",
        "artists": [],
        "album": None,
        "year": "",
        "duration": "",
        "durationSeconds": 0,
        "isExplicit": False,
        "trackNumber": None,
        "albumType": "",
        "albumTrackCount": 0,
        "likeStatus": "",
        "videoType": "",
        "thumbnails": [],
        "credits": None,
    }

    def _format_secs(secs: int) -> str:
        if secs <= 0:
            return ""
        m, s = divmod(secs, 60)
        if m >= 60:
            h, m = divmod(m, 60)
            return f"{h}:{m:02d}:{s:02d}"
        return f"{m}:{s:02d}"

    # --- watch playlist: best source for artist/album names ---
    try:
        watch = ytmusic.get_watch_playlist(videoId=video_id, limit=5)
        tracks = watch.get("tracks") or []
        tr: dict | None = None
        for candidate in tracks:
            if isinstance(candidate, dict) and candidate.get("videoId") == video_id:
                tr = candidate
                break
        if tr:
            out["title"] = tr.get("title") or ""
            out["artists"] = [
                {"name": a.get("name") or "", "id": a.get("id") or ""}
                for a in (tr.get("artists") or [])
                if isinstance(a, dict) and (a.get("name") or "")
            ]
            album = tr.get("album")
            if isinstance(album, dict):
                out["album"] = {
                    "name": album.get("name") or "",
                    "id": album.get("id") or "",
                }
            elif isinstance(album, str) and album:
                out["album"] = {"name": album, "id": ""}
            out["year"] = tr.get("year") or ""
            out["duration"] = tr.get("length") or tr.get("duration") or ""
            out["likeStatus"] = tr.get("likeStatus") or ""
            out["videoType"] = tr.get("videoType") or ""
            out["isExplicit"] = bool(tr.get("isExplicit"))
            thumbs = tr.get("thumbnail") or tr.get("thumbnails") or []
            if isinstance(thumbs, list):
                out["thumbnails"] = thumbs
    except _CATCH:
        pass

    # --- album page: track number, release type, credits browse id ---
    album_id = ""
    if isinstance(out.get("album"), dict):
        album_id = out["album"].get("id") or ""
    credits_id = None
    if album_id:
        try:
            album = ytmusic.get_album(album_id)
            out["albumType"] = album.get("type") or ""
            out["albumTrackCount"] = int(album.get("trackCount") or 0)
            if not out["year"]:
                out["year"] = album.get("year") or ""
            if not out["artists"]:
                out["artists"] = [
                    {"name": a.get("name") or "", "id": a.get("id") or ""}
                    for a in (album.get("artists") or [])
                    if isinstance(a, dict) and (a.get("name") or "")
                ]
            if isinstance(out.get("album"), dict) and not out["album"].get("name"):
                out["album"]["name"] = album.get("title") or ""
            for i, atr in enumerate(album.get("tracks") or []):
                if atr.get("videoId") != video_id:
                    continue
                tn = atr.get("trackNumber")
                if isinstance(tn, int) and tn > 0:
                    out["trackNumber"] = tn
                else:
                    out["trackNumber"] = i + 1
                out["isExplicit"] = bool(atr.get("isExplicit"))
                if atr.get("duration"):
                    out["duration"] = atr["duration"]
                ds = atr.get("duration_seconds")
                if isinstance(ds, (int, float)) and ds > 0:
                    out["durationSeconds"] = int(ds)
                credits_id = atr.get("creditsBrowseId") or credits_id
                if atr.get("artists"):
                    out["artists"] = [
                        {"name": a.get("name") or "", "id": a.get("id") or ""}
                        for a in atr["artists"]
                        if isinstance(a, dict) and (a.get("name") or "")
                    ]
                break
        except _CATCH:
            pass

    # --- credits: performers, writers, producers (names, not browse ids) ---
    if credits_id:
        try:
            out["credits"] = ytmusic.get_song_credits(credits_id)
        except _CATCH:
            out["credits"] = None

    # --- fallback: player playerDetails for missing title / length / author ---
    if not out["title"] or not out["duration"] or not out["artists"]:
        try:
            song = ytmusic.get_song(video_id)
            vd = song.get("videoDetails") or {}
            if not out["title"]:
                out["title"] = vd.get("title") or ""
            if not out["artists"] and vd.get("author"):
                # Author name only — never surface raw channelId as the label.
                out["artists"] = [{"name": vd["author"], "id": vd.get("channelId") or ""}]
            if not out["duration"]:
                try:
                    secs = int(vd.get("lengthSeconds") or 0)
                except (TypeError, ValueError):
                    secs = 0
                if secs > 0:
                    out["durationSeconds"] = secs
                    out["duration"] = _format_secs(secs)
            if not out["thumbnails"]:
                thumbs = (vd.get("thumbnail") or {}).get("thumbnails") or []
                if thumbs:
                    out["thumbnails"] = thumbs
        except _CATCH:
            pass

    if not out["duration"] and out["durationSeconds"]:
        out["duration"] = _format_secs(int(out["durationSeconds"]))

    if not (out.get("title") or "").strip():
        raise HTTPException(status_code=404, detail="song metadata unavailable")

    return out


if __name__ == "__main__":
    import uvicorn

    uvicorn.run(app, host="127.0.0.1", port=8000)

from fastapi import FastAPI, HTTPException, Query
from requests import RequestException
from ytmusicapi import YTMusic
from ytmusicapi.exceptions import YTMusicError, YTMusicUserError

app = FastAPI()
ytmusic = YTMusic()


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
    return {"ok": True}


@app.get("/search")
def search(
    q: str = Query(..., description="The search query"),
    filter: str | None = Query(
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
    Song / video metadata via get_song(videoId).
    Returns a flattened subset of videoDetails + microformat (not streaming URLs).
    """
    try:
        result = ytmusic.get_song(video_id)
    except _CATCH as exc:
        raise _ytm_error(exc) from exc

    vd = result.get("videoDetails") or {}
    mf = (result.get("microformat") or {}).get("microformatDataRenderer") or {}
    thumbs = (vd.get("thumbnail") or {}).get("thumbnails") or []
    if not thumbs:
        thumbs = (mf.get("thumbnail") or {}).get("thumbnails") or []

    return {
        "videoId": vd.get("videoId") or video_id,
        "title": vd.get("title") or "",
        "author": vd.get("author") or "",
        "channelId": vd.get("channelId") or "",
        "lengthSeconds": vd.get("lengthSeconds") or "",
        "viewCount": vd.get("viewCount") or "",
        "musicVideoType": vd.get("musicVideoType") or "",
        "isLiveContent": bool(vd.get("isLiveContent")),
        "thumbnails": thumbs,
        "description": mf.get("description") or "",
        "publishDate": mf.get("publishDate") or mf.get("uploadDate") or "",
        "category": mf.get("category") or "",
        "urlCanonical": mf.get("urlCanonical") or "",
    }


if __name__ == "__main__":
    import uvicorn

    uvicorn.run(app, host="127.0.0.1", port=8000)

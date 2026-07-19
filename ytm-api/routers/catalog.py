"""
Catalog routes — artists, albums, playlists, watch queue, and song metadata.

The /song/{video_id} endpoint is the most complex: it fans out across
get_watch_playlist, get_album, get_song_credits, and get_song to build a
rich metadata payload used by the TUI's Details rail.
"""
from fastapi import APIRouter, HTTPException, Query
import deps

router = APIRouter(tags=["catalog"])


# ---------------------------------------------------------------------------
# Artist
# ---------------------------------------------------------------------------

@router.get("/artist/{channel_id}")
def artist(channel_id: str):
    """
    Artist page via get_artist(channelId).
    Sections (songs/albums/singles/videos/…) include results plus optional browseId/params.
    """
    try:
        return deps.ytmusic.get_artist(channel_id)
    except deps._CATCH as exc:
        raise deps._ytm_error(exc) from exc


@router.get("/artist/{channel_id}/albums")
def artist_albums(
    channel_id: str,
    params: str = Query(..., description="params from get_artist albums/singles/shows section"),
    limit: int = Query(100, ge=1, le=500),
):
    """Full album/single/show list via get_artist_albums(channelId, params)."""
    try:
        results = deps.ytmusic.get_artist_albums(channel_id, params, limit=limit)
    except deps._CATCH as exc:
        raise deps._ytm_error(exc) from exc
    return {"albums": results}


# ---------------------------------------------------------------------------
# Album
# ---------------------------------------------------------------------------

@router.get("/album/browse-id")
def album_browse_id(
    audioPlaylistId: str = Query(..., description="audio playlist id starting with OLAK5uy_"),
):
    """Resolve OLAK5uy_* audioPlaylistId → MPREb_* album browseId."""
    try:
        browse_id = deps.ytmusic.get_album_browse_id(audioPlaylistId)
    except deps._CATCH as exc:
        raise deps._ytm_error(exc) from exc
    if not browse_id:
        raise HTTPException(status_code=404, detail="album browseId not found")
    return {"browseId": browse_id}


@router.get("/album/{browse_id}")
def album(browse_id: str):
    """Album/Single/EP page via get_album(browseId). type field distinguishes Album/Single/EP."""
    try:
        return deps.ytmusic.get_album(browse_id)
    except deps._CATCH as exc:
        raise deps._ytm_error(exc) from exc


# ---------------------------------------------------------------------------
# Playlist
# ---------------------------------------------------------------------------

@router.get("/playlist/{playlist_id}")
def playlist(
    playlist_id: str,
    limit: int = Query(100, ge=1, le=500),
    title: str | None = Query(None),
    author: str | None = Query(None),
):
    """Playlist page via get_playlist. Strips VL prefix when present."""
    try:
        pid = deps._normalize_playlist_id(playlist_id)
        if pid.startswith("RD"):
            # RDAMVM are auto-generated mixes/radios, get_playlist fails on them
            res = deps.ytmusic.get_watch_playlist(playlistId=pid, limit=limit)
            tracks = res.get("tracks") or []
            return {
                "id": res.get("playlistId", pid),
                "title": title or "Mix",
                "description": "YouTube Music Mix",
                "author": {"name": author or "YouTube Music", "id": ""},
                "trackCount": len(tracks),
                "tracks": tracks
            }
        return deps.ytmusic.get_playlist(pid, limit=limit)
    except deps._CATCH as exc:
        raise deps._ytm_error(exc) from exc


# ---------------------------------------------------------------------------
# Watch / radio queue
# ---------------------------------------------------------------------------

@router.get("/watch")
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
        return deps.ytmusic.get_watch_playlist(
            videoId=videoId,
            playlistId=playlistId,
            limit=limit,
            radio=radio,
        )
    except deps._CATCH as exc:
        raise deps._ytm_error(exc) from exc


# ---------------------------------------------------------------------------
# Song metadata (composite — fans out to watch/album/credits/song endpoints)
# ---------------------------------------------------------------------------

@router.get("/song/{video_id}")
def song(video_id: str):
    """
    Rich song metadata for the TUI Details rail.

    Fans out across multiple ytmusicapi sources in priority order:
    1. get_watch_playlist → title, artists, album ref, year, duration
    2. get_album          → release type, track #, explicit flag, creditsBrowseId
    3. get_song_credits   → performed / written / produced by
    4. get_song           → title / duration / author fallback
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
        "relatedBrowseId": "",
    }

    # 1. Watch playlist — best source for artist/album names.
    try:
        watch_data = deps.ytmusic.get_watch_playlist(videoId=video_id, limit=5)
        out["relatedBrowseId"] = watch_data.get("related") or ""
        for candidate in watch_data.get("tracks") or []:
            if not isinstance(candidate, dict) or candidate.get("videoId") != video_id:
                continue
            tr = candidate
            out["title"] = tr.get("title") or ""
            out["artists"] = [
                {"name": a.get("name") or "", "id": a.get("id") or ""}
                for a in (tr.get("artists") or [])
                if isinstance(a, dict) and a.get("name")
            ]
            album = tr.get("album")
            if isinstance(album, dict):
                out["album"] = {"name": album.get("name") or "", "id": album.get("id") or ""}
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
            break
    except deps._CATCH:
        pass

    # 2. Album page — track number, release type, credits browse id.
    album_id = ""
    if isinstance(out.get("album"), dict):
        album_id = out["album"].get("id") or ""

    credits_id = None
    if album_id:
        try:
            alb = deps.ytmusic.get_album(album_id)
            out["albumType"] = alb.get("type") or ""
            out["albumTrackCount"] = int(alb.get("trackCount") or 0)
            if not out["year"]:
                out["year"] = alb.get("year") or ""
            if not out["artists"]:
                out["artists"] = [
                    {"name": a.get("name") or "", "id": a.get("id") or ""}
                    for a in (alb.get("artists") or [])
                    if isinstance(a, dict) and a.get("name")
                ]
            if isinstance(out.get("album"), dict) and not out["album"].get("name"):
                out["album"]["name"] = alb.get("title") or ""
            for i, atr in enumerate(alb.get("tracks") or []):
                if atr.get("videoId") != video_id:
                    continue
                tn = atr.get("trackNumber")
                out["trackNumber"] = tn if isinstance(tn, int) and tn > 0 else i + 1
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
                        if isinstance(a, dict) and a.get("name")
                    ]
                break
        except deps._CATCH:
            pass

    # 3. Credits — performers, writers, producers.
    if credits_id:
        try:
            out["credits"] = deps.ytmusic.get_song_credits(credits_id)
        except deps._CATCH:
            out["credits"] = None

    # 4. Fallback — player videoDetails for missing title / length / author.
    if not out["title"] or not out["duration"] or not out["artists"]:
        try:
            song_data = deps.ytmusic.get_song(video_id)
            vd = song_data.get("videoDetails") or {}
            if not out["title"]:
                out["title"] = vd.get("title") or ""
            if not out["artists"] and vd.get("author"):
                out["artists"] = [{"name": vd["author"], "id": vd.get("channelId") or ""}]
            if not out["duration"]:
                secs = 0
                try:
                    secs = int(vd.get("lengthSeconds") or 0)
                except (TypeError, ValueError):
                    pass
                if secs > 0:
                    out["durationSeconds"] = secs
                    out["duration"] = deps._format_secs(secs)
            if not out["thumbnails"]:
                thumbs = (vd.get("thumbnail") or {}).get("thumbnails") or []
                if thumbs:
                    out["thumbnails"] = thumbs
        except deps._CATCH:
            pass

    if not out["duration"] and out["durationSeconds"]:
        out["duration"] = deps._format_secs(int(out["durationSeconds"]))

    if not (out.get("title") or "").strip():
        raise HTTPException(status_code=404, detail="song metadata unavailable")

    return out

@router.get("/song/related/{browse_id}")
def song_related(browse_id: str):
    try:
        return deps.ytmusic.get_song_related(browse_id)
    except deps._CATCH as exc:
        raise deps._ytm_error(exc) from exc

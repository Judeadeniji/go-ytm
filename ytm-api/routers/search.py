"""Search routes — full-text search and autocomplete suggestions."""
from typing import Literal

from fastapi import APIRouter, Query
import deps

router = APIRouter(tags=["search"])

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


@router.get("/search")
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
        results = deps.ytmusic.search(q, filter=filter, limit=limit)
    except deps._CATCH as exc:
        raise deps._ytm_error(exc) from exc
    return {"results": results}


@router.get("/suggestions")
def suggestions(q: str = Query(..., description="The search query")):
    """Get search suggestions with detailed runs (bold typed match + history flags)."""
    try:
        results = deps.ytmusic.get_search_suggestions(q, detailed_runs=True)
    except deps._CATCH as exc:
        raise deps._ytm_error(exc) from exc

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

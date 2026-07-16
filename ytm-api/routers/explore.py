"""
Explore routes — home feed, explore page, moods/genres, charts.

The mood-playlists endpoint uses a custom parser (_safe_get_mood_playlists)
because YouTube Music occasionally returns musicResponsiveListItemRenderer
items inside mood category sections, which the stock ytmusicapi doesn't handle.
"""
from fastapi import APIRouter, Query
import deps

router = APIRouter(tags=["explore"])


@router.get("/home")
def home(limit: int = Query(3, ge=1, le=20)):
    """Home feed — dynamic carousels including Quick picks."""
    try:
        results = deps.ytmusic.get_home(limit=limit)
    except deps._CATCH as exc:
        raise deps._ytm_error(exc) from exc
    return {"carousels": results}



@router.get("/explore")
def explore():
    """Explore page — new releases, moods/genres, trending, new videos."""
    try:
        result = deps.ytmusic.get_explore()
    except deps._CATCH as exc:
        raise deps._ytm_error(exc) from exc
    return result


@router.get("/explore/moods")
def explore_moods():
    """Mood & genre category list with params tokens."""
    try:
        result = deps.ytmusic.get_mood_categories()
    except deps._CATCH as exc:
        raise deps._ytm_error(exc) from exc
    return {"moodCategories": result}


def _safe_get_mood_playlists(params: str) -> list:
    """
    Custom implementation of get_mood_playlists that tolerates unknown item renderers.

    YouTube Music sometimes returns musicResponsiveListItemRenderer instead of
    musicTwoRowItemRenderer inside mood category sections. The stock ytmusicapi
    hard-codes MTRIR as the only key and raises KeyError on anything else.
    This implementation walks the raw InnerTube response and handles both.
    """
    MTRIR = "musicTwoRowItemRenderer"
    MRLIR = "musicResponsiveListItemRenderer"

    def _get_thumbnail(data: dict) -> list:
        for path in [
            ["thumbnail", "musicThumbnailRenderer", "thumbnail", "thumbnails"],
            ["thumbnailRenderer", "musicThumbnailRenderer", "thumbnail", "thumbnails"],
            ["thumbnail", "thumbnails"],
        ]:
            val: any = data
            for k in path:
                if isinstance(val, dict) and k in val:
                    val = val[k]
                else:
                    val = None
                    break
            if val:
                return val
        return []

    def _parse_mtrir(data: dict) -> dict:
        """Parse a musicTwoRowItemRenderer (standard playlist card)."""
        title = playlistId = None
        try:
            title = data["title"]["runs"][0]["text"]
        except (KeyError, IndexError):
            pass
        try:
            bid = data["title"]["runs"][0]["navigationEndpoint"]["browseEndpoint"]["browseId"]
            playlistId = bid[2:] if bid.startswith("VL") else bid
        except (KeyError, IndexError):
            pass
        if not playlistId:
            try:
                pid = (
                    data["overlay"]["musicItemThumbnailOverlayRenderer"]["content"]
                    ["musicPlayButtonRenderer"]["playNavigationEndpoint"]["watchEndpoint"]["playlistId"]
                )
                playlistId = pid[2:] if pid.startswith("VL") else pid
            except (KeyError, IndexError):
                pass
        return {"title": title or "", "playlistId": playlistId or "", "thumbnails": _get_thumbnail(data)}

    def _parse_mrlir(data: dict) -> dict:
        """Parse a musicResponsiveListItemRenderer (alternative YouTube renderer)."""
        title = playlistId = None
        try:
            title = (
                data["flexColumns"][0]["musicResponsiveListItemFlexColumnRenderer"]
                ["text"]["runs"][0]["text"]
            )
        except (KeyError, IndexError):
            pass
        try:
            bid = data["navigationEndpoint"]["browseEndpoint"]["browseId"]
            playlistId = bid[2:] if bid.startswith("VL") else bid
        except (KeyError, IndexError):
            pass
        if not playlistId:
            try:
                pid = (
                    data["overlay"]["musicItemThumbnailOverlayRenderer"]["content"]
                    ["musicPlayButtonRenderer"]["playNavigationEndpoint"]["watchEndpoint"]["playlistId"]
                )
                playlistId = pid[2:] if pid.startswith("VL") else pid
            except (KeyError, IndexError):
                pass
        return {"title": title or "", "playlistId": playlistId or "", "thumbnails": _get_thumbnail(data)}

    def _parse_item(item: dict) -> dict | None:
        if MTRIR in item:
            return _parse_mtrir(item[MTRIR])
        if MRLIR in item:
            return _parse_mrlir(item[MRLIR])
        return None

    response = deps.ytmusic._send_request(
        "browse", {"browseId": "FEmusic_moods_and_genres_category", "params": params}
    )

    try:
        sections = (
            response["contents"]["singleColumnBrowseResultsRenderer"]
            ["tabs"][0]["tabRenderer"]["content"]["sectionListRenderer"]["contents"]
        )
    except (KeyError, IndexError):
        return []

    playlists: list[dict] = []
    for section in sections:
        if "gridRenderer" in section:
            items = section["gridRenderer"].get("items", [])
        elif "musicCarouselShelfRenderer" in section:
            items = section["musicCarouselShelfRenderer"].get("contents", [])
        elif "musicImmersiveCarouselShelfRenderer" in section:
            items = section["musicImmersiveCarouselShelfRenderer"].get("contents", [])
        else:
            continue

        for item in items:
            parsed = _parse_item(item)
            if parsed:
                playlists.append(parsed)

    return playlists


@router.get("/explore/moods/playlists")
def explore_mood_playlists(params: str = Query(...)):
    """Playlists for a given mood/genre category (params token from /explore/moods)."""
    try:
        result = _safe_get_mood_playlists(params)
    except deps._CATCH as exc:
        raise deps._ytm_error(exc) from exc
    return {"playlists": result}


@router.get("/explore/charts")
def explore_charts(country: str = Query("ZZ")):
    """Global / regional charts — top songs, videos, artists, trending."""
    try:
        result = deps.ytmusic.get_charts(country=country)
    except deps._CATCH as exc:
        raise deps._ytm_error(exc) from exc
    return result

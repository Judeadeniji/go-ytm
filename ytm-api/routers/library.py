"""Library routes — playlists, songs, albums, artists."""
from fastapi import APIRouter, Query
import deps

router = APIRouter(prefix="/library", tags=["library"])


@router.get("/playlists")
def library_playlists(limit: int = Query(50)):
    """User's saved playlists."""
    try:
        return {"playlists": deps.ytmusic.get_library_playlists(limit=limit)}
    except deps._CATCH as exc:
        raise deps._ytm_error(exc) from exc


@router.get("/songs")
def library_songs(limit: int = Query(100)):
    """User's liked / saved songs."""
    try:
        return {"songs": deps.ytmusic.get_library_songs(limit=limit)}
    except deps._CATCH as exc:
        raise deps._ytm_error(exc) from exc


@router.get("/albums")
def library_albums(limit: int = Query(100)):
    """User's saved albums."""
    try:
        return {"albums": deps.ytmusic.get_library_albums(limit=limit)}
    except deps._CATCH as exc:
        raise deps._ytm_error(exc) from exc


@router.get("/artists")
def library_artists(limit: int = Query(100)):
    """User's followed artists."""
    try:
        return {"artists": deps.ytmusic.get_library_artists(limit=limit)}
    except deps._CATCH as exc:
        raise deps._ytm_error(exc) from exc

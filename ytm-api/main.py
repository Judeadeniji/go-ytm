"""
ytm-api — FastAPI server that bridges the Go TUI with ytmusicapi.

Layout
------
deps.py              — shared ytmusic instance, error helpers, utility funcs
routers/auth.py      — /auth/setup, /auth/oauth/*
routers/library.py   — /library/playlists|songs|albums|artists
routers/search.py    — /search, /suggestions
routers/explore.py   — /explore/home, /explore, /explore/moods, /explore/charts
routers/catalog.py   — /artist/*, /album/*, /playlist/*, /watch, /song/*
"""
import os

from fastapi import FastAPI, Request
from fastapi.responses import JSONResponse

import deps
from routers import auth, catalog, explore, library, search

app = FastAPI(title="ytm-api", version="0.1.0")

API_TOKEN = os.environ.get("YTM_API_TOKEN")

@app.middleware("http")
async def verify_token(request: Request, call_next):
    if request.url.path == "/health":
        return await call_next(request)
    if API_TOKEN and request.headers.get("X-API-Token") != API_TOKEN:
        return JSONResponse(status_code=403, content={"detail": "Unauthorized"})
    return await call_next(request)

# Health check lives here so it requires no router prefix.
@app.get("/health", tags=["meta"])
def health():
    """Liveness check used by the make run bootstrap."""
    return {"ok": True, "authenticated": os.path.exists(deps.AUTH_FILE)}


# Wire routers.
app.include_router(auth.router)
app.include_router(library.router)
app.include_router(search.router)
app.include_router(explore.router)
app.include_router(catalog.router)


if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="127.0.0.1", port=8000)

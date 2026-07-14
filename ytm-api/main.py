from fastapi import FastAPI, Query
from ytmusicapi import YTMusic

app = FastAPI()
ytmusic = YTMusic()

@app.get("/search")
def search(q: str = Query(..., description="The search query")):
    """
    Search YouTube Music.
    Returns a list of search results which can be songs, albums, artists, etc.
    """
    # By default, search returns a mix of types.
    results = ytmusic.search(q)
    return {"results": results}

@app.get("/suggestions")
def suggestions(q: str = Query(..., description="The search query")):
    """
    Get search suggestions with detailed runs.
    """
    results = ytmusic.get_search_suggestions(q, detailed_runs=True)
    return {"suggestions": results}

@app.get("/home")
def home(limit: int = 3):
    """
    Get the home page with dynamic carousels.
    """
    results = ytmusic.get_home(limit=limit)
    return {"carousels": results}

if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="127.0.0.1", port=8000)

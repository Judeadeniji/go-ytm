"""Auth routes — header-file setup and OAuth device-flow."""
import os
from pathlib import Path

from fastapi import APIRouter, HTTPException
from pydantic import BaseModel
from ytmusicapi.setup import setup
from ytmusicapi.auth.oauth import OAuthCredentials, RefreshingToken

from deps import AUTH_FILE, reload_ytmusic, ytmusic

router = APIRouter(prefix="/auth", tags=["auth"])


class AuthRequest(BaseModel):
    headers_raw: str


@router.post("/setup")
def auth_setup(req: AuthRequest):
    """Configure authentication from raw browser headers."""
    try:
        import json
        os.makedirs(os.path.dirname(AUTH_FILE), exist_ok=True)
        setup(filepath=AUTH_FILE, headers_raw=req.headers_raw)
        
        # Sanitize headers (remove HTTP request lines from copy/paste)
        with open(AUTH_FILE, 'r') as f:
            headers = json.load(f)
        
        clean_headers = {k: v for k, v in headers.items() if " " not in k and not k.startswith("GET") and not k.startswith("POST")}
        
        with open(AUTH_FILE, 'w') as f:
            json.dump(clean_headers, f, indent=4)
        os.chmod(AUTH_FILE, 0o600)
            
        reload_ytmusic()
        return {"status": "ok"}
    except Exception as exc:
        raise HTTPException(status_code=400, detail=str(exc))


class OAuthCodeRequest(BaseModel):
    client_id: str
    client_secret: str


@router.post("/oauth/code")
def oauth_code(req: OAuthCodeRequest):
    """Initiate OAuth device flow — returns user_code / verification_url."""
    cred = OAuthCredentials(req.client_id, req.client_secret)
    try:
        return cred.get_code()
    except Exception as exc:
        raise HTTPException(status_code=400, detail=str(exc))


class OAuthTokenRequest(BaseModel):
    client_id: str
    client_secret: str
    device_code: str


@router.post("/oauth/token")
def oauth_token(req: OAuthTokenRequest):
    """Poll for OAuth token after the user has authorised the device."""
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

    expires_in = raw_token.get("refresh_token_expires_in", raw_token.get("expires_in", 0))
    ref_token = RefreshingToken(
        credentials=cred,
        access_token=raw_token.get("access_token", ""),
        refresh_token=raw_token.get("refresh_token", ""),
        scope=raw_token.get("scope", ""),
        token_type=raw_token.get("token_type", ""),
        expires_in=expires_in,
    )
    ref_token.update(raw_token)

    os.makedirs(os.path.dirname(AUTH_FILE), exist_ok=True)
    ref_token.local_cache = Path(AUTH_FILE)
    if os.path.exists(AUTH_FILE):
        os.chmod(AUTH_FILE, 0o600)

    reload_ytmusic()
    return {"status": "ok"}


@router.get("/profile")
def get_profile():
    """Get the current authenticated user's profile."""
    if not os.path.exists(AUTH_FILE):
        return {"name": "", "photo": ""}
        
    try:
        res = ytmusic._send_request("account/account_menu", {})
        actions = res.get("actions", [])
        if actions:
            renderer = actions[0].get("openPopupAction", {}).get("popup", {}).get("multiPageMenuRenderer", {}).get("header", {}).get("activeAccountHeaderRenderer", {})
            runs = renderer.get("accountName", {}).get("runs", [])
            name = runs[0].get("text", "") if runs else ""
            
            thumbnails = renderer.get("accountPhoto", {}).get("thumbnails", [])
            photo = thumbnails[0].get("url", "") if thumbnails else ""
            
            return {"name": name, "photo": photo}
    except Exception:
        pass
        
    return {"name": "", "photo": ""}

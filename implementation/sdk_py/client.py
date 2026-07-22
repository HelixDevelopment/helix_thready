"""Typed, stdlib-only Python SDK for the Helix Thready REST ``/v1`` control API.

This module is the Python sibling of ``implementation/sdk_go`` and mirrors the
same ``/v1`` contract (``docs/public/research/mvp/api/openapi.yaml``, realized by
``implementation/rest_gateway``). It is self-contained and depends on **nothing
outside the Python standard library** (``urllib.request``, ``json``, ``uuid``),
so it can be vendored on its own.

A :class:`ThreadyClient` injects auth (a JWT bearer access token OR an
``X-API-Key``), encodes/decodes JSON, maps every non-2xx response to a typed
:class:`ApiError`, retries idempotent GETs on transient 503/429 with capped
exponential backoff, and stamps a fresh ``Idempotency-Key`` onto unsafe POSTs
(``create_channel``, ``reprocess``).

Security: the client refuses to attach a credential over cleartext HTTP to a
non-loopback host (a classic token-leak footgun) unless ``allow_insecure_http``
is explicitly set, raising :class:`InsecureTransportError` instead.
"""

from __future__ import annotations

import json
import time
import urllib.error
import urllib.parse
import urllib.request
import uuid
from dataclasses import dataclass, field
from typing import Any, Dict, List, Optional

__all__ = [
    "ThreadyClient",
    "ThreadyError",
    "ApiError",
    "TransportError",
    "InsecureTransportError",
    "TokenPair",
    "Channel",
    "Thread",
    "Post",
    "Job",
    "SearchHit",
    "SearchResults",
    "Skill",
]

__version__ = "0.1.0"

# Default tuning for a freshly constructed client.
DEFAULT_TIMEOUT = 30.0
DEFAULT_MAX_RETRIES = 3
DEFAULT_BACKOFF_BASE = 0.025  # 25 ms
DEFAULT_BACKOFF_MAX = 2.0

# Hosts for which cleartext HTTP is considered safe (local development).
_LOOPBACK_HOSTS = frozenset({"localhost", "127.0.0.1", "::1"})


# ---------------------------------------------------------------------------
#  Errors
# ---------------------------------------------------------------------------
class ThreadyError(Exception):
    """Base class for every error raised by the SDK."""


class TransportError(ThreadyError):
    """A network/transport failure (DNS, connection refused, timeout, ...)."""


class InsecureTransportError(ThreadyError):
    """Raised when the SDK would attach a credential over an unsafe transport.

    Cleartext HTTP to a non-loopback host would leak the bearer token / API key
    on the wire. Pass ``allow_insecure_http=True`` to opt out of this guard.
    """


class ApiError(ThreadyError):
    """Typed error for every non-2xx response.

    Decoded from the gateway's canonical failure envelope
    ``{"error": {"code", "message", "status", "request_id", "trace_id", ...}}``.
    Callers branch on :attr:`code` / :attr:`status`.
    """

    def __init__(
        self,
        code: str,
        message: str,
        status: int,
        request_id: str = "",
        trace_id: str = "",
        retry_after: Optional[int] = None,
        details: Optional[List[Dict[str, Any]]] = None,
    ) -> None:
        self.code = code
        self.message = message
        self.status = status
        self.request_id = request_id
        self.trace_id = trace_id
        self.retry_after = retry_after
        self.details = details or []
        if request_id:
            text = f"thready: {code} ({status}): {message} [request_id={request_id}]"
        else:
            text = f"thready: {code} ({status}): {message}"
        super().__init__(text)

    def retryable(self) -> bool:
        """Whether this error's code is one the SDK may transparently retry."""
        return self.code in ("rate_limited", "unavailable", "deadline_exceeded")


# Canonical string codes, 1:1 with the gateway / Connect taxonomy.
_STATUS_TO_CODE = {
    400: "invalid_argument",
    401: "unauthenticated",
    403: "permission_denied",
    404: "not_found",
    409: "conflict",
    412: "failed_precondition",
    422: "unprocessable",
    429: "rate_limited",
    500: "internal",
    503: "unavailable",
    504: "deadline_exceeded",
}


def _code_for_status(status: int) -> str:
    return _STATUS_TO_CODE.get(status, "internal")


# ---------------------------------------------------------------------------
#  Typed DTOs (mirror the wire shapes emitted by rest_gateway / sdk_go)
# ---------------------------------------------------------------------------
@dataclass
class TokenPair:
    access_token: str = ""
    refresh_token: str = ""
    token_type: str = ""
    expires_in: int = 0
    refresh_expires_in: int = 0

    @classmethod
    def from_dict(cls, d: Dict[str, Any]) -> "TokenPair":
        d = d or {}
        return cls(
            access_token=d.get("access_token", ""),
            refresh_token=d.get("refresh_token", ""),
            token_type=d.get("token_type", ""),
            expires_in=d.get("expires_in", 0) or 0,
            refresh_expires_in=d.get("refresh_expires_in", 0) or 0,
        )


@dataclass
class Channel:
    id: str = ""
    account_id: str = ""
    name: str = ""
    platform: str = ""
    external_ref: str = ""
    created_at: str = ""

    @classmethod
    def from_dict(cls, d: Dict[str, Any]) -> "Channel":
        d = d or {}
        return cls(
            id=d.get("id", ""),
            account_id=d.get("account_id", ""),
            name=d.get("name", ""),
            platform=d.get("platform", ""),
            external_ref=d.get("external_ref", ""),
            created_at=d.get("created_at", ""),
        )


@dataclass
class Thread:
    id: str = ""
    channel_id: str = ""
    root_post_id: str = ""
    reply_post_ids: List[str] = field(default_factory=list)

    @classmethod
    def from_dict(cls, d: Dict[str, Any]) -> "Thread":
        d = d or {}
        return cls(
            id=d.get("id", ""),
            channel_id=d.get("channel_id", ""),
            root_post_id=d.get("root_post_id", ""),
            reply_post_ids=list(d.get("reply_post_ids") or []),
        )


@dataclass
class Post:
    id: str = ""
    channel_id: str = ""
    account_id: str = ""
    body: str = ""
    hashtags: List[str] = field(default_factory=list)
    categories: List[str] = field(default_factory=list)
    status: str = ""
    created_at: str = ""

    @classmethod
    def from_dict(cls, d: Dict[str, Any]) -> "Post":
        d = d or {}
        return cls(
            id=d.get("id", ""),
            channel_id=d.get("channel_id", ""),
            account_id=d.get("account_id", ""),
            body=d.get("body", ""),
            hashtags=list(d.get("hashtags") or []),
            categories=list(d.get("categories") or []),
            status=d.get("status", ""),
            created_at=d.get("created_at", ""),
        )


@dataclass
class Job:
    job_id: str = ""
    post_id: str = ""
    status: str = ""
    precedence: List[str] = field(default_factory=list)
    queued_at: str = ""

    @classmethod
    def from_dict(cls, d: Dict[str, Any]) -> "Job":
        d = d or {}
        return cls(
            job_id=d.get("job_id", ""),
            post_id=d.get("post_id", ""),
            status=d.get("status", ""),
            precedence=list(d.get("precedence") or []),
            queued_at=d.get("queued_at", ""),
        )


@dataclass
class SearchHit:
    source_id: str = ""
    kind: str = ""
    score: float = 0.0
    span: Optional[str] = None
    snippet: str = ""

    @classmethod
    def from_dict(cls, d: Dict[str, Any]) -> "SearchHit":
        d = d or {}
        return cls(
            source_id=d.get("source_id", ""),
            kind=d.get("kind", ""),
            score=d.get("score", 0.0) or 0.0,
            span=d.get("span"),
            snippet=d.get("snippet", ""),
        )


@dataclass
class SearchResults:
    results: List[SearchHit] = field(default_factory=list)
    took_ms: int = 0
    embedder: str = ""

    @classmethod
    def from_dict(cls, d: Dict[str, Any]) -> "SearchResults":
        d = d or {}
        return cls(
            results=[SearchHit.from_dict(h) for h in (d.get("results") or [])],
            took_ms=d.get("took_ms", 0) or 0,
            embedder=d.get("embedder", ""),
        )


@dataclass
class Skill:
    id: str = ""
    name: str = ""
    kind: str = ""
    sort_order: int = 0

    @classmethod
    def from_dict(cls, d: Dict[str, Any]) -> "Skill":
        d = d or {}
        return cls(
            id=d.get("id", ""),
            name=d.get("name", ""),
            kind=d.get("kind", ""),
            sort_order=d.get("sort_order", 0) or 0,
        )


def _new_idempotency_key() -> str:
    """Mint a UUIDv4 for an unsafe POST."""
    return str(uuid.uuid4())


# ---------------------------------------------------------------------------
#  Client
# ---------------------------------------------------------------------------
class ThreadyClient:
    """A typed, stdlib-only client for the Helix Thready ``/v1`` API.

    Args:
        base_url: Gateway origin, with or without a trailing slash, e.g.
            ``"https://thready.hxd3v.com/v1"`` or ``"http://127.0.0.1:8080"``.
            The ``/v1`` prefix is added per-path, so pass only the origin
            (``"http://127.0.0.1:8080"``), matching the Go SDK.
        access_token: JWT bearer access token (sent as ``Authorization: Bearer``).
        api_key: Scoped API key (sent as ``X-API-Key``) for non-interactive use.
        timeout: Per-request timeout in seconds (default 30).
        allow_insecure_http: Permit attaching credentials over cleartext HTTP to
            a non-loopback host. Off by default; leaving it off raises
            :class:`InsecureTransportError` for that case.
    """

    def __init__(
        self,
        base_url: str,
        access_token: Optional[str] = None,
        api_key: Optional[str] = None,
        timeout: float = DEFAULT_TIMEOUT,
        allow_insecure_http: bool = False,
    ) -> None:
        base = (base_url or "").strip().rstrip("/")
        if not base:
            raise ValueError("thready: base_url is required")
        self._base_url = base
        self._access_token = access_token or ""
        self._api_key = api_key or ""
        self._timeout = timeout if timeout and timeout > 0 else DEFAULT_TIMEOUT
        self._allow_insecure_http = bool(allow_insecure_http)

        self.max_retries = DEFAULT_MAX_RETRIES
        self.backoff_base = DEFAULT_BACKOFF_BASE
        self.backoff_max = DEFAULT_BACKOFF_MAX

        parts = urllib.parse.urlsplit(self._base_url)
        self._scheme = (parts.scheme or "").lower()
        self._host = (parts.hostname or "").lower()

        # No env-proxy interference; explicit, self-contained transport.
        self._opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))

    # ----- auth / transport safety -----
    @property
    def access_token(self) -> str:
        """The token the client currently authenticates with (refreshed by login)."""
        return self._access_token

    def _transport_is_safe(self) -> bool:
        """Whether it is safe to put a credential on the wire for this base URL."""
        if self._scheme == "https":
            return True
        if self._scheme == "http":
            host = self._host
            if host in _LOOPBACK_HOSTS or host.startswith("127."):
                return True
            return self._allow_insecure_http
        # Unknown scheme: be conservative unless explicitly allowed.
        return self._allow_insecure_http

    def _apply_auth(self, headers: Dict[str, str]) -> None:
        """Inject the credential: bearer JWT if present, else X-API-Key.

        Bearer wins over the API key. Raises :class:`InsecureTransportError` if a
        credential would be attached over an unsafe transport. A call with no
        credential (e.g. login on a fresh client) attaches nothing and never
        raises.
        """
        credential = bool(self._access_token or self._api_key)
        if not credential:
            return
        if not self._transport_is_safe():
            raise InsecureTransportError(
                "thready: refusing to send credentials over cleartext http to "
                f"non-loopback host {self._host!r}; pass allow_insecure_http=True "
                "to override"
            )
        if self._access_token:
            headers["Authorization"] = "Bearer " + self._access_token
        elif self._api_key:
            headers["X-API-Key"] = self._api_key

    # ----- request plumbing -----
    def _backoff(self, attempt: int) -> None:
        d = self.backoff_base * (2 ** (attempt - 1))
        if d > self.backoff_max or d <= 0:
            d = self.backoff_max
        time.sleep(d)

    def _parse_api_error(self, status: int, headers: Any, raw: bytes) -> ApiError:
        header_request_id = ""
        if headers is not None:
            header_request_id = headers.get("X-Request-Id", "") or ""
        env: Any = None
        try:
            env = json.loads(raw.decode("utf-8")) if raw else None
        except (ValueError, UnicodeDecodeError):
            env = None
        if isinstance(env, dict) and isinstance(env.get("error"), dict):
            e = env["error"]
            return ApiError(
                code=e.get("code") or _code_for_status(status),
                message=e.get("message", ""),
                status=e.get("status") or status,
                request_id=e.get("request_id") or header_request_id,
                trace_id=e.get("trace_id", "") or "",
                retry_after=e.get("retry_after"),
                details=e.get("details"),
            )
        msg = ""
        try:
            msg = raw.decode("utf-8").strip()
        except UnicodeDecodeError:
            msg = ""
        if not msg:
            msg = _code_for_status(status)
        return ApiError(
            code=_code_for_status(status),
            message=msg,
            status=status,
            request_id=header_request_id,
        )

    def _do(
        self,
        method: str,
        path: str,
        query: Optional[Dict[str, Any]] = None,
        body: Any = None,
        idempotency_key: str = "",
    ) -> Any:
        """Perform a request with JSON encode/decode, auth, typed error mapping,
        and (for idempotent GETs) capped-backoff retries on 503/429 and transient
        transport errors. Returns the decoded 2xx body, or ``None`` for 204."""
        data: Optional[bytes] = None
        if body is not None:
            data = json.dumps(body).encode("utf-8")

        url = self._base_url + path
        if query:
            url += "?" + urllib.parse.urlencode(query, doseq=True)

        attempts = 1
        if method == "GET":
            attempts = self.max_retries + 1

        last_err: Optional[Exception] = None
        for attempt in range(attempts):
            if attempt > 0:
                self._backoff(attempt)

            headers: Dict[str, str] = {"Accept": "application/json"}
            if data is not None:
                headers["Content-Type"] = "application/json"
            if idempotency_key:
                headers["Idempotency-Key"] = idempotency_key
            self._apply_auth(headers)  # may raise InsecureTransportError

            req = urllib.request.Request(url, data=data, headers=headers, method=method)
            try:
                resp = self._opener.open(req, timeout=self._timeout)
            except urllib.error.HTTPError as e:
                status = e.code
                err_body = e.read()
                if (
                    method == "GET"
                    and attempt < attempts - 1
                    and status in (503, 429)
                ):
                    last_err = self._parse_api_error(status, e.headers, err_body)
                    continue
                raise self._parse_api_error(status, e.headers, err_body)
            except urllib.error.URLError as e:
                last_err = TransportError(f"thready: {method} {path}: {e.reason}")
                if method == "GET" and attempt < attempts - 1:
                    continue
                raise last_err
            with resp:
                status = getattr(resp, "status", None) or resp.getcode()
                raw = resp.read()
            if status == 204 or not raw:
                return None
            return json.loads(raw.decode("utf-8"))

        # Only reached if a GET exhausted its retries.
        assert last_err is not None
        raise last_err

    # ----- typed methods over the /v1 surface -----
    def login(
        self, email: str, password: str, totp: Optional[str] = None
    ) -> TokenPair:
        """Exchange credentials (+ TOTP for admin tiers) for a token pair and
        store the access token for subsequent calls. ``POST /v1/auth/login``."""
        payload: Dict[str, Any] = {"email": email, "password": password}
        if totp is not None:
            payload["totp"] = totp
        data = self._do("POST", "/v1/auth/login", body=payload)
        tp = TokenPair.from_dict(data)
        if tp.access_token:
            self._access_token = tp.access_token
        return tp

    def list_channels(self) -> List[Channel]:
        """List the channels registered for the caller's tenant.
        ``GET /v1/channels``."""
        data = self._do("GET", "/v1/channels")
        return [Channel.from_dict(d) for d in (data or {}).get("data", [])]

    def create_channel(
        self,
        name: str,
        platform: str = "",
        external_ref: str = "",
        idempotency_key: Optional[str] = None,
    ) -> Channel:
        """Register a channel/group to read. Unsafe POST — carries an
        ``Idempotency-Key`` (auto-generated unless supplied).
        ``POST /v1/channels``."""
        key = idempotency_key or _new_idempotency_key()
        payload = {"name": name, "platform": platform, "external_ref": external_ref}
        data = self._do("POST", "/v1/channels", body=payload, idempotency_key=key)
        return Channel.from_dict(data)

    def get_channel_threads(self, channel_id: str) -> List[Thread]:
        """Fetch the threads for a channel.
        ``GET /v1/channels/{channelId}/threads``."""
        path = "/v1/channels/" + urllib.parse.quote(channel_id, safe="") + "/threads"
        data = self._do("GET", path)
        return [Thread.from_dict(d) for d in (data or {}).get("data", [])]

    def get_post(self, post_id: str) -> Post:
        """Fetch a single post by id. ``GET /v1/posts/{postId}``."""
        path = "/v1/posts/" + urllib.parse.quote(post_id, safe="")
        data = self._do("GET", path)
        return Post.from_dict(data)

    def reprocess(self, post_id: str, idempotency_key: Optional[str] = None) -> Job:
        """Force a fresh processing run for a post; returns the queued job
        (202 Accepted). Unsafe POST — carries an ``Idempotency-Key``.
        ``POST /v1/posts/{postId}/reprocess``."""
        key = idempotency_key or _new_idempotency_key()
        path = "/v1/posts/" + urllib.parse.quote(post_id, safe="") + "/reprocess"
        data = self._do("POST", path, body=None, idempotency_key=key)
        return Job.from_dict(data)

    def search(
        self,
        query: str,
        mode: Optional[str] = None,
        top_k: Optional[int] = None,
        sources: Optional[List[str]] = None,
        rerank: Optional[bool] = None,
    ) -> SearchResults:
        """Run a semantic / keyword / hybrid search. ``POST /v1/search``.

        ``mode`` is one of ``semantic|keyword|hybrid``; ``sources`` selects the
        corpora (``posts|generated|assets``)."""
        payload: Dict[str, Any] = {"query": query}
        if mode is not None:
            payload["mode"] = mode
        if sources is not None:
            payload["sources"] = sources
        if top_k is not None:
            payload["top_k"] = top_k
        if rerank is not None:
            payload["rerank"] = rerank
        data = self._do("POST", "/v1/search", body=payload)
        return SearchResults.from_dict(data)

    def list_skills(self) -> List[Skill]:
        """List the Skill-Graph knowledge units. ``GET /v1/skills``."""
        data = self._do("GET", "/v1/skills")
        return [Skill.from_dict(d) for d in (data or {}).get("data", [])]

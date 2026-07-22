"""Unit tests for the Helix Thready Python SDK (:mod:`client`).

The SDK is a *client*, so the honest test approach is to drive it against a REAL
``http.server`` mock of the gateway's ``/v1`` contract — bound to a free port on
127.0.0.1 in a background thread — that records the exact method / path / headers
/ body it receives and returns canned, contract-shaped JSON. Each test asserts
BOTH the request the SDK sent (auth header injected, Idempotency-Key present, the
right method+path) and the typed value it decoded back. No skips, no mocks of the
SDK's own internals, no network stubbing library — a genuine socket round-trip.
"""

from __future__ import annotations

import http.server
import json
import threading
import unittest
import urllib.parse

import client
from client import (
    ApiError,
    Channel,
    InsecureTransportError,
    Job,
    Post,
    SearchResults,
    Skill,
    ThreadyClient,
    TokenPair,
)


# ---------------------------------------------------------------------------
#  Mock gateway
# ---------------------------------------------------------------------------
class MockGateway:
    """State + routing for the mock ``/v1`` server. One instance per test so the
    recorded requests and tunable behaviours stay isolated."""

    def __init__(self) -> None:
        self.requests = []  # list of recorded request dicts
        # Tunable behaviours:
        self.skills_fail_times = 0  # leading 503s to emit on GET /v1/skills
        self.skills_calls = 0
        self.reprocess_calls = 0

    def route(self, h: "_Handler", rec: dict) -> None:
        method, path = rec["method"], rec["path"]

        if method == "POST" and path == "/v1/auth/login":
            body = json.loads(rec["body"] or b"{}")
            # Echo a token pair; the mock does light credential validation.
            if body.get("email") == "reject@thready.test":
                return h.send_error_envelope(
                    401, "unauthenticated", "bad credentials", "req-login-401"
                )
            return h.send_json(
                200,
                {
                    "access_token": "jwt-access-" + body.get("email", "x"),
                    "refresh_token": "jwt-refresh",
                    "token_type": "Bearer",
                    "expires_in": 900,
                    "refresh_expires_in": 604800,
                },
            )

        if method == "GET" and path == "/v1/channels":
            return h.send_json(
                200,
                {
                    "data": [
                        {
                            "id": "chan-1",
                            "account_id": "acct-a",
                            "name": "general",
                            "platform": "telegram",
                            "external_ref": "@g",
                            "created_at": "2026-07-22T09:00:00Z",
                        },
                        {
                            "id": "chan-2",
                            "account_id": "acct-a",
                            "name": "ops",
                            "platform": "max",
                            "external_ref": "@o",
                            "created_at": "2026-07-22T09:01:00Z",
                        },
                    ],
                    "meta": {"next_cursor": None, "total_estimate": 2},
                },
            )

        if method == "POST" and path == "/v1/channels":
            body = json.loads(rec["body"] or b"{}")
            return h.send_json(
                201,
                {
                    "id": "chan-9",
                    "account_id": "acct-a",
                    "name": body.get("name", ""),
                    "platform": body.get("platform", ""),
                    "external_ref": body.get("external_ref", ""),
                    "created_at": "2026-07-22T09:02:00Z",
                },
            )

        if method == "GET" and path.startswith("/v1/posts/") and path.endswith("/reprocess") is False:
            post_id = path[len("/v1/posts/"):]
            if post_id == "missing":
                return h.send_error_envelope(
                    404, "not_found", "post not found", "req-abc-123"
                )
            return h.send_json(
                200,
                {
                    "id": post_id,
                    "channel_id": "chan-1",
                    "account_id": "acct-a",
                    "body": "hello #research",
                    "hashtags": ["#research"],
                    "categories": ["research"],
                    "status": "succeeded",
                    "created_at": "2026-07-22T09:03:00Z",
                },
            )

        if method == "POST" and path.startswith("/v1/posts/") and path.endswith("/reprocess"):
            self.reprocess_calls += 1
            post_id = path[len("/v1/posts/"):-len("/reprocess")]
            return h.send_json(
                202,
                {
                    "job_id": "job-1",
                    "post_id": post_id,
                    "status": "queued",
                    "precedence": ["download", "convert", "analyze", "research", "reply"],
                    "queued_at": "2026-07-22T09:04:00Z",
                },
            )

        if method == "POST" and path == "/v1/search":
            return h.send_json(
                200,
                {
                    "results": [
                        {
                            "source_id": "post-1",
                            "kind": "post",
                            "score": 0.81,
                            "span": "section:1",
                            "snippet": "...benchmarks...",
                        }
                    ],
                    "took_ms": 7,
                    "embedder": "llama",
                },
            )

        if method == "GET" and path == "/v1/skills":
            self.skills_calls += 1
            if self.skills_calls <= self.skills_fail_times:
                return h.send_error_envelope(
                    503, "unavailable", "embedder warming up", "req-503"
                )
            return h.send_json(
                200,
                {
                    "data": [
                        {"id": "skill-download", "name": "download", "kind": "atomic", "sort_order": 1},
                        {"id": "skill-reply", "name": "reply", "kind": "atomic", "sort_order": 5},
                    ],
                    "meta": {"next_cursor": None},
                },
            )

        return h.send_error_envelope(404, "not_found", f"no route for {method} {path}", "req-noroute")


class _Handler(http.server.BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"

    def log_message(self, *args) -> None:  # silence test noise
        pass

    def _handle(self) -> None:
        gw: MockGateway = self.server.gateway  # type: ignore[attr-defined]
        length = int(self.headers.get("Content-Length", 0) or 0)
        raw = self.rfile.read(length) if length else b""
        parsed = urllib.parse.urlsplit(self.path)
        rec = {
            "method": self.command,
            "path": parsed.path,
            "query": urllib.parse.parse_qs(parsed.query),
            # self.headers is an http.client.HTTPMessage whose .get() is
            # case-insensitive — the honest way to assert on a header the wire
            # may have re-cased (e.g. X-API-Key -> X-Api-Key).
            "headers": self.headers,
            "body": raw,
        }
        gw.requests.append(rec)
        gw.route(self, rec)

    do_GET = _handle
    do_POST = _handle
    do_DELETE = _handle

    # --- response helpers ---
    def send_json(self, status: int, obj) -> None:
        payload = json.dumps(obj).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(payload)))
        self.end_headers()
        if payload:
            self.wfile.write(payload)

    def send_error_envelope(self, status: int, code: str, message: str, request_id: str) -> None:
        obj = {
            "error": {
                "code": code,
                "message": message,
                "status": status,
                "request_id": request_id,
                "trace_id": request_id,
            }
        }
        payload = json.dumps(obj).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("X-Request-Id", request_id)
        self.send_header("Content-Length", str(len(payload)))
        self.end_headers()
        self.wfile.write(payload)


class GatewayServerCase(unittest.TestCase):
    """Base case: spin up a real mock gateway on a free port for each test."""

    def setUp(self) -> None:
        self.gateway = MockGateway()
        self.httpd = http.server.ThreadingHTTPServer(("127.0.0.1", 0), _Handler)
        self.httpd.gateway = self.gateway  # type: ignore[attr-defined]
        # Small poll interval so tearDown's shutdown() returns promptly instead
        # of waiting out serve_forever's default 0.5s poll cycle.
        self.thread = threading.Thread(
            target=self.httpd.serve_forever, kwargs={"poll_interval": 0.01}, daemon=True
        )
        self.thread.start()
        host, port = self.httpd.server_address
        self.base_url = f"http://127.0.0.1:{port}"

    def tearDown(self) -> None:
        self.httpd.shutdown()
        self.httpd.server_close()
        self.thread.join(timeout=5)

    def new_client(self, **kwargs) -> ThreadyClient:
        c = ThreadyClient(self.base_url, **kwargs)
        # Keep retry sleeps tiny under test.
        c.backoff_base = 0.001
        c.backoff_max = 0.005
        return c

    def last_request(self) -> dict:
        return self.gateway.requests[-1]


# ---------------------------------------------------------------------------
#  Method + wire-contract tests
# ---------------------------------------------------------------------------
class TestLogin(GatewayServerCase):
    def test_sends_credentials_and_stores_token(self):
        c = self.new_client()
        tp = c.login("user@thready.test", "userpassword-123")
        self.assertIsInstance(tp, TokenPair)
        self.assertEqual(tp.access_token, "jwt-access-user@thready.test")
        self.assertEqual(tp.token_type, "Bearer")
        self.assertEqual(tp.expires_in, 900)
        # Request assertions.
        req = self.last_request()
        self.assertEqual(req["method"], "POST")
        self.assertEqual(req["path"], "/v1/auth/login")
        self.assertEqual(req["headers"].get("Content-Type"), "application/json")
        body = json.loads(req["body"])
        self.assertEqual(body["email"], "user@thready.test")
        self.assertEqual(body["password"], "userpassword-123")
        self.assertNotIn("totp", body)  # omitted when not supplied
        # Token stored → next call authenticates automatically.
        self.assertEqual(c.access_token, "jwt-access-user@thready.test")
        c.list_channels()
        self.assertEqual(
            self.last_request()["headers"].get("Authorization"),
            "Bearer jwt-access-user@thready.test",
        )

    def test_totp_included_when_supplied(self):
        c = self.new_client()
        c.login("admin@thready.test", "adminpassword-123", totp="123456")
        body = json.loads(self.last_request()["body"])
        self.assertEqual(body["totp"], "123456")

    def test_bad_credentials_maps_to_api_error(self):
        c = self.new_client()
        with self.assertRaises(ApiError) as ctx:
            c.login("reject@thready.test", "whatever-1234")
        self.assertEqual(ctx.exception.code, "unauthenticated")
        self.assertEqual(ctx.exception.status, 401)


class TestListChannels(GatewayServerCase):
    def test_injects_bearer_and_decodes_envelope(self):
        c = self.new_client(access_token="tok-1")
        chans = c.list_channels()
        req = self.last_request()
        self.assertEqual(req["method"], "GET")
        self.assertEqual(req["path"], "/v1/channels")
        self.assertEqual(req["headers"].get("Authorization"), "Bearer tok-1")
        self.assertEqual(len(chans), 2)
        self.assertIsInstance(chans[0], Channel)
        self.assertEqual(chans[0].id, "chan-1")
        self.assertEqual(chans[1].platform, "max")


class TestCreateChannel(GatewayServerCase):
    def test_sends_idempotency_key_and_body(self):
        c = self.new_client(access_token="tok-1")
        ch = c.create_channel("release", platform="telegram", external_ref="@rel")
        req = self.last_request()
        self.assertEqual(req["method"], "POST")
        self.assertEqual(req["path"], "/v1/channels")
        self.assertEqual(req["headers"].get("Content-Type"), "application/json")
        key = req["headers"].get("Idempotency-Key")
        self.assertTrue(key, "Idempotency-Key must be present on an unsafe POST")
        # Auto key is a UUIDv4.
        self.assertRegex(
            key,
            r"^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$",
        )
        body = json.loads(req["body"])
        self.assertEqual(body, {"name": "release", "platform": "telegram", "external_ref": "@rel"})
        self.assertIsInstance(ch, Channel)
        self.assertEqual(ch.id, "chan-9")
        self.assertEqual(ch.name, "release")

    def test_idempotency_key_override(self):
        c = self.new_client(access_token="tok-1")
        c.create_channel("x", platform="telegram", external_ref="@x", idempotency_key="fixed-key-42")
        self.assertEqual(self.last_request()["headers"].get("Idempotency-Key"), "fixed-key-42")


class TestGetPost(GatewayServerCase):
    def test_path_and_typed_decode(self):
        c = self.new_client(access_token="tok-1")
        post = c.get_post("post-1")
        req = self.last_request()
        self.assertEqual(req["method"], "GET")
        self.assertEqual(req["path"], "/v1/posts/post-1")
        self.assertIsInstance(post, Post)
        self.assertEqual(post.id, "post-1")
        self.assertEqual(post.status, "succeeded")
        self.assertEqual(post.hashtags, ["#research"])

    def test_404_maps_to_typed_api_error(self):
        c = self.new_client(access_token="tok-1")
        with self.assertRaises(ApiError) as ctx:
            c.get_post("missing")
        err = ctx.exception
        self.assertEqual(err.code, "not_found")
        self.assertEqual(err.status, 404)
        self.assertEqual(err.request_id, "req-abc-123")
        self.assertEqual(err.message, "post not found")
        self.assertFalse(err.retryable())


class TestReprocess(GatewayServerCase):
    def test_returns_job_with_idempotency_key(self):
        c = self.new_client(access_token="tok-1")
        job = c.reprocess("post-1")
        req = self.last_request()
        self.assertEqual(req["method"], "POST")
        self.assertEqual(req["path"], "/v1/posts/post-1/reprocess")
        self.assertTrue(req["headers"].get("Idempotency-Key"), "Idempotency-Key must be present")
        self.assertIsInstance(job, Job)
        self.assertEqual(job.job_id, "job-1")
        self.assertEqual(job.status, "queued")
        self.assertEqual(len(job.precedence), 5)
        self.assertEqual(job.precedence[0], "download")


class TestSearch(GatewayServerCase):
    def test_sends_body_and_decodes_results(self):
        c = self.new_client(access_token="tok-1")
        res = c.search(
            "vector database benchmarks",
            mode="hybrid",
            sources=["posts", "generated"],
            top_k=20,
            rerank=True,
        )
        req = self.last_request()
        self.assertEqual(req["method"], "POST")
        self.assertEqual(req["path"], "/v1/search")
        body = json.loads(req["body"])
        self.assertEqual(body["query"], "vector database benchmarks")
        self.assertEqual(body["mode"], "hybrid")
        self.assertEqual(body["top_k"], 20)
        self.assertEqual(body["sources"], ["posts", "generated"])
        self.assertEqual(body["rerank"], True)
        self.assertIsInstance(res, SearchResults)
        self.assertEqual(res.embedder, "llama")
        self.assertEqual(res.took_ms, 7)
        self.assertEqual(len(res.results), 1)
        self.assertEqual(res.results[0].source_id, "post-1")
        self.assertEqual(res.results[0].span, "section:1")

    def test_optional_fields_omitted(self):
        c = self.new_client(access_token="tok-1")
        c.search("just a query")
        body = json.loads(self.last_request()["body"])
        self.assertEqual(body, {"query": "just a query"})


class TestListSkills(GatewayServerCase):
    def test_decodes_envelope(self):
        c = self.new_client(access_token="tok-1")
        skills = c.list_skills()
        self.assertEqual(self.last_request()["path"], "/v1/skills")
        self.assertEqual(len(skills), 2)
        self.assertIsInstance(skills[0], Skill)
        self.assertEqual(skills[0].name, "download")
        self.assertEqual(skills[1].sort_order, 5)


# ---------------------------------------------------------------------------
#  Auth injection
# ---------------------------------------------------------------------------
class TestAuthInjection(GatewayServerCase):
    def test_api_key_header_when_no_token(self):
        c = self.new_client(api_key="sk-secret-123")
        c.list_channels()
        req = self.last_request()
        self.assertEqual(req["headers"].get("X-API-Key"), "sk-secret-123")
        self.assertIsNone(req["headers"].get("Authorization"))

    def test_bearer_wins_over_api_key(self):
        c = self.new_client(access_token="tok-1", api_key="sk-secret-123")
        c.list_channels()
        req = self.last_request()
        self.assertEqual(req["headers"].get("Authorization"), "Bearer tok-1")
        self.assertIsNone(req["headers"].get("X-API-Key"))

    def test_no_credential_sends_neither(self):
        c = self.new_client()
        c.list_channels()
        req = self.last_request()
        self.assertIsNone(req["headers"].get("Authorization"))
        self.assertIsNone(req["headers"].get("X-API-Key"))


# ---------------------------------------------------------------------------
#  Retry behaviour
# ---------------------------------------------------------------------------
class TestRetry(GatewayServerCase):
    def test_get_503_then_success_makes_two_requests(self):
        self.gateway.skills_fail_times = 1
        c = self.new_client(access_token="tok-1")
        skills = c.list_skills()
        self.assertEqual(self.gateway.skills_calls, 2, "one 503 must be retried once")
        self.assertEqual(len(skills), 2)

    def test_get_429_then_success(self):
        # Reuse skills route knob but with a 429 by patching the responder.
        self.gateway.skills_fail_times = 1
        # Force the failure to be a 429 instead of 503.
        orig_route = self.gateway.route

        def route(h, rec):
            if rec["method"] == "GET" and rec["path"] == "/v1/skills":
                self.gateway.skills_calls += 1
                if self.gateway.skills_calls <= self.gateway.skills_fail_times:
                    return h.send_error_envelope(429, "rate_limited", "slow down", "req-429")
                return h.send_json(200, {"data": [], "meta": {"next_cursor": None}})
            return orig_route(h, rec)

        self.gateway.route = route  # type: ignore[assignment]
        c = self.new_client(access_token="tok-1")
        c.list_skills()
        self.assertEqual(self.gateway.skills_calls, 2)

    def test_get_exhausted_returns_api_error_after_four_attempts(self):
        self.gateway.skills_fail_times = 99  # always 503
        c = self.new_client(access_token="tok-1")
        with self.assertRaises(ApiError) as ctx:
            c.list_skills()
        self.assertEqual(ctx.exception.code, "unavailable")
        # 1 initial + max_retries(3) = 4 attempts.
        self.assertEqual(self.gateway.skills_calls, 4)

    def test_unsafe_post_not_retried_on_503(self):
        # Patch reprocess to always 503; the client must NOT retry an unsafe POST.
        orig_route = self.gateway.route

        def route(h, rec):
            if rec["method"] == "POST" and rec["path"].endswith("/reprocess"):
                self.gateway.reprocess_calls += 1
                return h.send_error_envelope(503, "unavailable", "down", "req-p")
            return orig_route(h, rec)

        self.gateway.route = route  # type: ignore[assignment]
        c = self.new_client(access_token="tok-1")
        with self.assertRaises(ApiError):
            c.reprocess("post-1")
        self.assertEqual(self.gateway.reprocess_calls, 1, "unsafe POST must not be retried")


# ---------------------------------------------------------------------------
#  Insecure-transport guard
# ---------------------------------------------------------------------------
class TestInsecureTransportGuard(unittest.TestCase):
    def test_http_remote_with_credentials_raises(self):
        c = ThreadyClient("http://api.example.com", access_token="tok-1")
        with self.assertRaises(InsecureTransportError):
            c.list_channels()

    def test_http_remote_with_api_key_raises(self):
        c = ThreadyClient("http://api.example.com", api_key="sk-1")
        with self.assertRaises(InsecureTransportError):
            c.list_channels()

    def test_http_remote_without_credentials_does_not_raise_guard(self):
        # No credential → nothing to leak → guard must not fire. (It will fail
        # later with a transport error, which is NOT InsecureTransportError.)
        c = ThreadyClient("http://127.0.0.1:9", access_token=None, api_key=None)
        headers: dict = {}
        c._apply_auth(headers)  # must not raise
        self.assertEqual(headers, {})

    def test_http_loopback_with_credentials_allowed(self):
        for host in ("127.0.0.1", "localhost", "::1"):
            base = "http://[::1]:8080" if host == "::1" else f"http://{host}:8080"
            c = ThreadyClient(base, access_token="tok-1")
            headers: dict = {}
            c._apply_auth(headers)  # must not raise
            self.assertEqual(headers.get("Authorization"), "Bearer tok-1", host)

    def test_https_remote_with_credentials_allowed(self):
        c = ThreadyClient("https://api.example.com", access_token="tok-1")
        headers: dict = {}
        c._apply_auth(headers)  # must not raise
        self.assertEqual(headers.get("Authorization"), "Bearer tok-1")

    def test_allow_insecure_http_override(self):
        c = ThreadyClient(
            "http://api.example.com", access_token="tok-1", allow_insecure_http=True
        )
        headers: dict = {}
        c._apply_auth(headers)  # must not raise despite http+remote
        self.assertEqual(headers.get("Authorization"), "Bearer tok-1")


class TestInsecureTransportLoopbackEndToEnd(GatewayServerCase):
    def test_http_loopback_real_call_succeeds_with_token(self):
        # The mock server runs on http://127.0.0.1 — a real credentialed call
        # over loopback HTTP must be allowed and round-trip successfully.
        c = self.new_client(access_token="tok-1")
        chans = c.list_channels()
        self.assertEqual(len(chans), 2)
        self.assertEqual(self.last_request()["headers"].get("Authorization"), "Bearer tok-1")


# ---------------------------------------------------------------------------
#  Construction / misc
# ---------------------------------------------------------------------------
class TestConstruction(unittest.TestCase):
    def test_requires_base_url(self):
        with self.assertRaises(ValueError):
            ThreadyClient("")

    def test_trailing_slash_trimmed(self):
        c = ThreadyClient("https://x/v1/")
        self.assertEqual(c._base_url, "https://x/v1")

    def test_api_error_string_and_retryable(self):
        e = ApiError(code="rate_limited", message="slow down", status=429, request_id="req-9")
        self.assertIn("rate_limited", str(e))
        self.assertIn("req-9", str(e))
        self.assertTrue(e.retryable())
        self.assertFalse(ApiError(code="not_found", message="", status=404).retryable())


if __name__ == "__main__":
    unittest.main()

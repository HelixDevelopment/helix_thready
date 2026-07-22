// TDD suite for the Helix Thready TypeScript/ESM SDK.
//
// The SDK is a *client*; the honest unit-test approach is to exercise it
// against a REAL node:http server that mocks the gateway's `/v1` contract
// (methods, paths, headers, and the exact wire shapes the real rest_gateway
// emits) — NOT against a live gateway. Each test asserts the request the SDK
// sends (method, path, headers, body) and the typed value it decodes back.

import test from "node:test";
import assert from "node:assert/strict";
import http from "node:http";

import {
  ThreadyClient,
  ApiError,
  InsecureTransportError,
  Code,
} from "../src/client.mjs";

// ----- test harness: a real loopback mock server per test -----

/**
 * Start a real node:http server on a free loopback port. `handler` receives
 * (req, res, bodyText) with the request body already buffered. Returns
 * { baseUrl, close }.
 */
function startServer(handler) {
  return new Promise((resolve) => {
    const server = http.createServer((req, res) => {
      const chunks = [];
      req.on("data", (c) => chunks.push(c));
      req.on("end", () => {
        const bodyText = Buffer.concat(chunks).toString("utf8");
        handler(req, res, bodyText);
      });
    });
    server.listen(0, "127.0.0.1", () => {
      const { port } = server.address();
      resolve({
        baseUrl: `http://127.0.0.1:${port}`,
        close: () => new Promise((r) => server.close(r)),
      });
    });
  });
}

/** Run `fn(baseUrl)` against a fresh mock server, always closing it after. */
async function withServer(handler, fn) {
  const { baseUrl, close } = await startServer(handler);
  try {
    return await fn(baseUrl);
  } finally {
    await close();
  }
}

function writeJson(res, status, obj) {
  res.writeHead(status, { "Content-Type": "application/json" });
  res.end(JSON.stringify(obj));
}

function writeErrorEnvelope(res, status, code, message, requestId) {
  res.writeHead(status, {
    "Content-Type": "application/json",
    "X-Request-Id": requestId,
  });
  res.end(
    JSON.stringify({
      error: {
        code,
        message,
        status,
        request_id: requestId,
        trace_id: requestId,
      },
    }),
  );
}

/** A client with retry sleeps shrunk to keep retry tests fast. */
function newClient(baseUrl, extra = {}) {
  const c = new ThreadyClient({ baseUrl, ...extra });
  c._backoffBaseMs = 1;
  c._backoffMaxMs = 4;
  return c;
}

// ----- method / path / header / decode tests -----

test("login sends credentials and a subsequent call carries the bearer token", async () => {
  const wantToken = "jwt-access-abc123";
  let seenLoginBody = null;
  let seenCT = "";
  let seenAuthOnChannels = "MISSING";

  await withServer(
    (req, res, bodyText) => {
      if (req.method === "POST" && req.url === "/v1/auth/login") {
        seenCT = req.headers["content-type"] || "";
        seenLoginBody = JSON.parse(bodyText);
        return writeJson(res, 200, {
          access_token: wantToken,
          refresh_token: "jwt-refresh",
          token_type: "Bearer",
          expires_in: 900,
          refresh_expires_in: 604800,
        });
      }
      if (req.method === "GET" && req.url === "/v1/channels") {
        seenAuthOnChannels = req.headers["authorization"] || "";
        return writeJson(res, 200, { data: [] });
      }
      return writeJson(res, 404, {});
    },
    async (baseUrl) => {
      const c = newClient(baseUrl);
      const tp = await c.login({
        email: "user@thready.test",
        password: "userpassword-123",
      });
      assert.equal(seenCT, "application/json");
      assert.deepEqual(seenLoginBody, {
        email: "user@thready.test",
        password: "userpassword-123",
      });
      assert.equal(tp.access_token, wantToken);
      assert.equal(c.accessToken, wantToken, "client must store the token");

      await c.listChannels();
      assert.equal(seenAuthOnChannels, `Bearer ${wantToken}`);
    },
  );
});

test("listChannels injects the bearer header and decodes the list envelope", async () => {
  let seenMethod = "";
  let seenPath = "";
  let seenAuth = "";

  await withServer(
    (req, res) => {
      seenMethod = req.method;
      seenPath = req.url;
      seenAuth = req.headers["authorization"] || "";
      writeJson(res, 200, {
        data: [
          { id: "chan-1", account_id: "acct-a", name: "general", platform: "telegram", external_ref: "@g", created_at: "2023-11-14T00:00:00Z" },
          { id: "chan-2", account_id: "acct-a", name: "ops", platform: "max", external_ref: "@o", created_at: "2023-11-14T00:00:00Z" },
        ],
        meta: { next_cursor: null, total_estimate: 2 },
      });
    },
    async (baseUrl) => {
      const c = newClient(baseUrl, { accessToken: "tok-1" });
      const chans = await c.listChannels();
      assert.equal(seenMethod, "GET");
      assert.equal(seenPath, "/v1/channels");
      assert.equal(seenAuth, "Bearer tok-1");
      assert.equal(chans.length, 2);
      assert.equal(chans[0].id, "chan-1");
      assert.equal(chans[1].platform, "max");
    },
  );
});

test("createChannel sends an auto Idempotency-Key + JSON body and decodes the channel", async () => {
  let seenMethod = "";
  let seenPath = "";
  let seenKey = "MISSING";
  let seenCT = "";
  let seenBody = null;

  await withServer(
    (req, res, bodyText) => {
      seenMethod = req.method;
      seenPath = req.url;
      seenKey = req.headers["idempotency-key"] || "";
      seenCT = req.headers["content-type"] || "";
      seenBody = JSON.parse(bodyText);
      writeJson(res, 201, {
        id: "chan-9",
        account_id: "acct-a",
        name: seenBody.name,
        platform: seenBody.platform,
        external_ref: seenBody.external_ref,
        created_at: "2023-11-14T22:13:29Z",
      });
    },
    async (baseUrl) => {
      const c = newClient(baseUrl, { accessToken: "tok-1" });
      const ch = await c.createChannel({
        name: "release",
        platform: "telegram",
        externalRef: "@rel",
      });
      assert.equal(seenMethod, "POST");
      assert.equal(seenPath, "/v1/channels");
      assert.notEqual(seenKey, "", "Idempotency-Key must be present on an unsafe POST");
      assert.match(seenKey, /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/, "auto key is a UUID");
      assert.equal(seenCT, "application/json");
      assert.deepEqual(seenBody, { name: "release", platform: "telegram", external_ref: "@rel" });
      assert.equal(ch.id, "chan-9");
      assert.equal(ch.name, "release");
    },
  );
});

test("createChannel honours an explicit Idempotency-Key override", async () => {
  let seenKey = "MISSING";
  await withServer(
    (req, res) => {
      seenKey = req.headers["idempotency-key"] || "";
      writeJson(res, 201, { id: "chan-9" });
    },
    async (baseUrl) => {
      const c = newClient(baseUrl, { accessToken: "tok-1" });
      await c.createChannel(
        { name: "x", platform: "telegram", externalRef: "@x" },
        { idempotencyKey: "fixed-key-42" },
      );
      assert.equal(seenKey, "fixed-key-42");
    },
  );
});

test("getPost hits GET /v1/posts/{id} and decodes a typed post", async () => {
  let seenPath = "";
  await withServer(
    (req, res) => {
      seenPath = req.url;
      writeJson(res, 200, {
        id: "post-1",
        channel_id: "chan-1",
        account_id: "acct-a",
        body: "hello",
        hashtags: ["#research"],
        categories: ["research"],
        status: "succeeded",
        created_at: "2023-11-14T00:01:40Z",
      });
    },
    async (baseUrl) => {
      const c = newClient(baseUrl, { accessToken: "tok-1" });
      const post = await c.getPost("post-1");
      assert.equal(seenPath, "/v1/posts/post-1");
      assert.equal(post.id, "post-1");
      assert.equal(post.status, "succeeded");
      assert.equal(post.hashtags[0], "#research");
    },
  );
});

test("reprocess hits POST /v1/posts/{id}/reprocess, sends a key, decodes the job", async () => {
  let seenMethod = "";
  let seenPath = "";
  let seenKey = "MISSING";
  await withServer(
    (req, res) => {
      seenMethod = req.method;
      seenPath = req.url;
      seenKey = req.headers["idempotency-key"] || "";
      writeJson(res, 202, {
        job_id: "job-1",
        post_id: "post-1",
        status: "queued",
        precedence: ["download", "convert", "analyze", "research", "reply"],
        queued_at: "2023-11-14T00:00:01Z",
      });
    },
    async (baseUrl) => {
      const c = newClient(baseUrl, { accessToken: "tok-1" });
      const job = await c.reprocess("post-1");
      assert.equal(seenMethod, "POST");
      assert.equal(seenPath, "/v1/posts/post-1/reprocess");
      assert.notEqual(seenKey, "", "Idempotency-Key must be present on reprocess");
      assert.equal(job.job_id, "job-1");
      assert.equal(job.status, "queued");
      assert.equal(job.precedence.length, 5);
    },
  );
});

test("search POSTs the body (topK→top_k) and decodes ranked results", async () => {
  let seenMethod = "";
  let seenPath = "";
  let seenBody = null;
  await withServer(
    (req, res, bodyText) => {
      seenMethod = req.method;
      seenPath = req.url;
      seenBody = JSON.parse(bodyText);
      writeJson(res, 200, {
        results: [
          { source_id: "post-1", kind: "post", score: 0.81, span: "section:1", snippet: "…benchmarks…" },
        ],
        took_ms: 7,
        embedder: "llama",
      });
    },
    async (baseUrl) => {
      const c = newClient(baseUrl, { accessToken: "tok-1" });
      const res = await c.search({
        query: "vector database benchmarks",
        mode: "hybrid",
        sources: ["posts", "generated"],
        topK: 20,
        rerank: true,
      });
      assert.equal(seenMethod, "POST");
      assert.equal(seenPath, "/v1/search");
      assert.deepEqual(seenBody, {
        query: "vector database benchmarks",
        rerank: true,
        mode: "hybrid",
        sources: ["posts", "generated"],
        top_k: 20,
      });
      assert.equal(res.embedder, "llama");
      assert.equal(res.results.length, 1);
      assert.equal(res.results[0].source_id, "post-1");
    },
  );
});

test("listSkills decodes the list envelope", async () => {
  let seenPath = "";
  await withServer(
    (req, res) => {
      seenPath = req.url;
      writeJson(res, 200, {
        data: [
          { id: "skill-download", name: "download", kind: "atomic", sort_order: 1 },
          { id: "skill-reply", name: "reply", kind: "atomic", sort_order: 5 },
        ],
      });
    },
    async (baseUrl) => {
      const c = newClient(baseUrl, { accessToken: "tok-1" });
      const skills = await c.listSkills();
      assert.equal(seenPath, "/v1/skills");
      assert.equal(skills.length, 2);
      assert.equal(skills[0].name, "download");
      assert.equal(skills[1].sort_order, 5);
    },
  );
});

test("API-key auth sends X-API-Key and NOT Authorization", async () => {
  let seenAuth = "PRESENT";
  let seenKey = "";
  await withServer(
    (req, res) => {
      seenAuth = req.headers["authorization"] || "";
      seenKey = req.headers["x-api-key"] || "";
      writeJson(res, 200, { data: [] });
    },
    async (baseUrl) => {
      const c = newClient(baseUrl, { apiKey: "sk-secret-123" });
      await c.listChannels();
      assert.equal(seenKey, "sk-secret-123");
      assert.equal(seenAuth, "", "Authorization must be empty when using an API key");
    },
  );
});

test("bearer wins when both an access token and an API key are set", async () => {
  let seenAuth = "";
  let seenKey = "PRESENT";
  await withServer(
    (req, res) => {
      seenAuth = req.headers["authorization"] || "";
      seenKey = req.headers["x-api-key"] || "";
      writeJson(res, 200, { data: [] });
    },
    async (baseUrl) => {
      const c = newClient(baseUrl, { accessToken: "tok-win", apiKey: "sk-loses" });
      await c.listChannels();
      assert.equal(seenAuth, "Bearer tok-win");
      assert.equal(seenKey, "", "X-API-Key must not be sent when a bearer token is present");
    },
  );
});

// ----- error mapping -----

test("a 404 canonical envelope maps to a typed ApiError (code/status/requestId)", async () => {
  await withServer(
    (req, res) => {
      writeErrorEnvelope(res, 404, Code.NOT_FOUND, "post not found", "req-abc-123");
    },
    async (baseUrl) => {
      const c = newClient(baseUrl, { accessToken: "tok-1" });
      await assert.rejects(
        () => c.getPost("missing"),
        (err) => {
          assert.ok(err instanceof ApiError, `expected ApiError, got ${err?.name}`);
          assert.equal(err.code, Code.NOT_FOUND);
          assert.equal(err.status, 404);
          assert.equal(err.requestId, "req-abc-123");
          assert.equal(err.message, "post not found");
          assert.equal(err.retryable(), false);
          return true;
        },
      );
    },
  );
});

test("a non-envelope error body degrades to a status-derived ApiError", async () => {
  await withServer(
    (req, res) => {
      res.writeHead(500, { "Content-Type": "text/plain", "X-Request-Id": "req-500" });
      res.end("boom");
    },
    async (baseUrl) => {
      const c = newClient(baseUrl, { accessToken: "tok-1" });
      await assert.rejects(
        () => c.getPost("x"),
        (err) => {
          assert.ok(err instanceof ApiError);
          assert.equal(err.code, Code.INTERNAL);
          assert.equal(err.status, 500);
          assert.equal(err.requestId, "req-500");
          assert.equal(err.message, "boom");
          return true;
        },
      );
    },
  );
});

// ----- retries -----

test("an idempotent GET retries 503 → 200 (exactly 2 server calls)", async () => {
  let calls = 0;
  await withServer(
    (req, res) => {
      calls += 1;
      if (calls === 1) {
        return writeErrorEnvelope(res, 503, Code.UNAVAILABLE, "embedder warming up", "req-1");
      }
      writeJson(res, 200, { data: [{ id: "skill-download", name: "download", kind: "atomic", sort_order: 1 }] });
    },
    async (baseUrl) => {
      const c = newClient(baseUrl, { accessToken: "tok-1" });
      const skills = await c.listSkills();
      assert.equal(calls, 2, "one 503 must be retried exactly once");
      assert.equal(skills.length, 1);
    },
  );
});

test("an idempotent GET retries 429 then succeeds", async () => {
  let calls = 0;
  await withServer(
    (req, res) => {
      calls += 1;
      if (calls === 1) {
        return writeErrorEnvelope(res, 429, Code.RATE_LIMITED, "slow down", "req-2");
      }
      writeJson(res, 200, { data: [] });
    },
    async (baseUrl) => {
      const c = newClient(baseUrl, { accessToken: "tok-1" });
      await c.listChannels();
      assert.equal(calls, 2);
    },
  );
});

test("a GET that stays 503 exhausts retries and throws the typed ApiError (1 + maxRetries calls)", async () => {
  let calls = 0;
  await withServer(
    (req, res) => {
      calls += 1;
      writeErrorEnvelope(res, 503, Code.UNAVAILABLE, "still down", "req-x");
    },
    async (baseUrl) => {
      const c = newClient(baseUrl, { accessToken: "tok-1" });
      await assert.rejects(
        () => c.listSkills(),
        (err) => {
          assert.ok(err instanceof ApiError);
          assert.equal(err.code, Code.UNAVAILABLE);
          assert.equal(err.retryable(), true);
          return true;
        },
      );
      assert.equal(calls, 4, "1 initial + 3 retries = 4 attempts");
    },
  );
});

test("an unsafe POST is NOT retried on 503 (exactly 1 server call)", async () => {
  let calls = 0;
  await withServer(
    (req, res) => {
      calls += 1;
      writeErrorEnvelope(res, 503, Code.UNAVAILABLE, "down", "req-p");
    },
    async (baseUrl) => {
      const c = newClient(baseUrl, { accessToken: "tok-1" });
      await assert.rejects(() => c.reprocess("post-1"), (err) => err instanceof ApiError);
      assert.equal(calls, 1, "an unsafe POST must not be retried");
    },
  );
});

// ----- constructor + ApiError value semantics -----

test("the constructor requires baseUrl", () => {
  assert.throws(() => new ThreadyClient({}), /baseUrl is required/);
  assert.throws(() => new ThreadyClient({ baseUrl: "  " }), /baseUrl is required/);
  assert.doesNotThrow(() => new ThreadyClient({ baseUrl: "https://x/v1/" }));
});

test("ApiError renders a log-correlatable string and reports retryability", () => {
  const e = new ApiError({ code: Code.RATE_LIMITED, message: "slow down", status: 429, requestId: "req-9" });
  const s = e.toString();
  assert.match(s, /rate_limited/);
  assert.match(s, /req-9/);
  assert.equal(e.retryable(), true);
  assert.equal(new ApiError({ code: Code.NOT_FOUND }).retryable(), false);
});

// ----- security: insecure-transport guard -----

test("insecure-transport guard: http + remote host + credentials is refused before any send", async () => {
  // A non-loopback literal IP → no DNS, no socket: the guard throws first.
  for (const cred of [{ accessToken: "tok-secret" }, { apiKey: "sk-secret" }]) {
    const c = new ThreadyClient({ baseUrl: "http://203.0.113.7", ...cred });
    await assert.rejects(
      () => c.listChannels(),
      (err) => {
        assert.ok(err instanceof InsecureTransportError, `expected InsecureTransportError, got ${err?.name}`);
        return true;
      },
    );
  }
});

test("insecure-transport guard: http + 127.0.0.1 + credentials is allowed and sends the bearer", async () => {
  let seenAuth = "";
  await withServer(
    (req, res) => {
      seenAuth = req.headers["authorization"] || "";
      writeJson(res, 200, { data: [] });
    },
    async (baseUrl) => {
      // baseUrl is already http://127.0.0.1:<port> — loopback plaintext http.
      const c = newClient(baseUrl, { accessToken: "tok-loopback" });
      await c.listChannels();
      assert.equal(seenAuth, "Bearer tok-loopback");
    },
  );
});

test("insecure-transport guard: http + localhost + credentials is allowed (loopback)", async () => {
  // Point at localhost with a closed port: the guard must permit the send
  // (reaching the transport, which then fails with a NON-guard network error).
  const c = new ThreadyClient({
    baseUrl: "http://localhost:1",
    accessToken: "tok-1",
    timeoutMs: 150,
  });
  await assert.rejects(
    () => c.search({ query: "x" }), // POST → single attempt, no GET retry amplification
    (err) => {
      assert.ok(!(err instanceof InsecureTransportError), "localhost is loopback → must be allowed through");
      return true;
    },
  );
});

test("insecure-transport guard: https + remote + credentials is allowed through the guard", async () => {
  // https always bypasses the plaintext refusal, regardless of host. The send
  // reaches the transport and fails with a NON-guard network error (fast: a
  // literal IP means no DNS, bounded by timeoutMs).
  const c = new ThreadyClient({
    baseUrl: "https://203.0.113.7",
    accessToken: "tok-secret",
    timeoutMs: 150,
  });
  await assert.rejects(
    () => c.search({ query: "x" }),
    (err) => {
      assert.ok(!(err instanceof InsecureTransportError), "https must never be refused");
      return true;
    },
  );
});

test("insecure-transport guard: allowInsecureHttp opts into http + remote + credentials", async () => {
  const c = new ThreadyClient({
    baseUrl: "http://203.0.113.7",
    accessToken: "tok-secret",
    allowInsecureHttp: true,
    timeoutMs: 150,
  });
  await assert.rejects(
    () => c.search({ query: "x" }),
    (err) => {
      assert.ok(!(err instanceof InsecureTransportError), "allowInsecureHttp must suppress the refusal");
      return true;
    },
  );
});

test("insecure-transport guard: no credential ⇒ no refusal even on remote http", async () => {
  // An unauthenticated call (e.g. login before a token exists) is unaffected by
  // the guard. baseUrl is loopback here so it actually completes.
  await withServer(
    (req, res) => {
      // Assert the request carried neither credential header.
      assert.equal(req.headers["authorization"] || "", "");
      assert.equal(req.headers["x-api-key"] || "", "");
      writeJson(res, 200, { data: [] });
    },
    async (baseUrl) => {
      const c = newClient(baseUrl); // no credentials
      await c.listChannels();
    },
  );
});

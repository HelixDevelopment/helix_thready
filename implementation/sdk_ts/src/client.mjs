// Helix Thready — TypeScript/JavaScript SDK (ESM) for the REST `/v1` control API.
//
// Schema: docs/public/research/mvp/api/openapi.yaml; served by the
// implementation/rest_gateway module. Sibling implementations: the Go SDK
// (implementation/sdk_go) and Python SDK (implementation/sdk_py). The three
// clients speak the identical `/v1` wire contract.
//
// This module is stdlib-only: it imports nothing outside Node's built-ins
// (node:http, node:https, node:net, node:crypto). Because it is authored in
// ESM, no build/transpile step is needed to run it or its tests — the
// hand-written `client.d.ts` supplies the TypeScript typings for consumers.
//
// A ThreadyClient injects auth (a JWT bearer access token OR an X-API-Key —
// bearer wins), encodes/decodes JSON, maps every non-2xx response to a typed
// ApiError, retries idempotent GETs on transient 503/429 with capped
// exponential backoff, and stamps a fresh Idempotency-Key onto unsafe POSTs.
// It refuses to attach a credential to a plaintext-http request bound for a
// non-loopback host (throwing InsecureTransportError) unless explicitly opted
// out via allowInsecureHttp.

import http from "node:http";
import https from "node:https";
import { isIPv4 } from "node:net";
import { randomUUID } from "node:crypto";

// Default tuning for a freshly constructed client.
const DEFAULT_TIMEOUT_MS = 30_000;
const DEFAULT_MAX_RETRIES = 3;
const DEFAULT_BACKOFF_BASE_MS = 25;
const DEFAULT_BACKOFF_MAX_MS = 2_000;

/**
 * Stable, machine-readable error codes. The values mirror the canonical
 * taxonomy served by the gateway (see implementation/rest_gateway and
 * docs/.../api/error-model.md), which maps 1:1 with the Connect/gRPC canonical
 * codes so a single client-side handler works across REST and the event plane.
 * @readonly
 */
export const Code = Object.freeze({
  INVALID_ARGUMENT: "invalid_argument",
  UNPROCESSABLE: "unprocessable",
  UNAUTHENTICATED: "unauthenticated",
  PERMISSION_DENIED: "permission_denied",
  NOT_FOUND: "not_found",
  ALREADY_EXISTS: "already_exists",
  CONFLICT: "conflict",
  FAILED_PRECONDITION: "failed_precondition",
  RATE_LIMITED: "rate_limited",
  DEADLINE_EXCEEDED: "deadline_exceeded",
  UNAVAILABLE: "unavailable",
  INTERNAL: "internal",
});

const RETRYABLE_CODES = new Set([
  Code.RATE_LIMITED,
  Code.UNAVAILABLE,
  Code.DEADLINE_EXCEEDED,
]);

/**
 * InsecureTransportError is thrown instead of attaching a credential (an
 * "Authorization: Bearer …" or "X-API-Key: …" header) to a request that would
 * travel over plaintext http to a NON-loopback host. Sending a bearer token or
 * API key in the clear to a remote origin would expose it to any on-path
 * observer, so the SDK refuses by default. https (any host) and http to a
 * loopback host (127.0.0.1, ::1, localhost) are always allowed; set
 * `allowInsecureHttp: true` to opt out of the refusal on trusted networks.
 */
export class InsecureTransportError extends Error {
  constructor(
    message = "thready: refusing to send credentials over plaintext http to a non-loopback host; use https or set allowInsecureHttp: true",
  ) {
    super(message);
    this.name = "InsecureTransportError";
  }
}

/**
 * ApiError is the typed error surfaced for every non-2xx response. It is
 * decoded from the gateway's canonical failure envelope:
 *   {"error":{"code","message","status","request_id","trace_id","details":[…]}}
 * missing status/request_id are backfilled from the HTTP status line / headers.
 */
export class ApiError extends Error {
  /**
   * @param {{code?: string, message?: string, status?: number, requestId?: string,
   *          traceId?: string, retryAfter?: number|null, details?: Array<object>}} fields
   */
  constructor({
    code = Code.INTERNAL,
    message = "",
    status = 0,
    requestId = "",
    traceId = "",
    retryAfter = null,
    details = [],
  } = {}) {
    super(message || code);
    this.name = "ApiError";
    this.code = code;
    this.status = status;
    this.requestId = requestId;
    this.traceId = traceId;
    this.retryAfter = retryAfter;
    this.details = details;
  }

  /** A human-readable, log-correlatable rendering. */
  toString() {
    const base = `thready: ${this.code} (${this.status}): ${this.message}`;
    return this.requestId ? `${base} [request_id=${this.requestId}]` : base;
  }

  /** Whether the error's code is one the SDK considers transiently retryable. */
  retryable() {
    return RETRYABLE_CODES.has(this.code);
  }
}

/** Maps an HTTP status to a canonical Code for the non-envelope fallback path. */
function codeForStatus(status) {
  switch (status) {
    case 400:
      return Code.INVALID_ARGUMENT;
    case 401:
      return Code.UNAUTHENTICATED;
    case 403:
      return Code.PERMISSION_DENIED;
    case 404:
      return Code.NOT_FOUND;
    case 409:
      return Code.CONFLICT;
    case 412:
      return Code.FAILED_PRECONDITION;
    case 422:
      return Code.UNPROCESSABLE;
    case 429:
      return Code.RATE_LIMITED;
    case 503:
      return Code.UNAVAILABLE;
    case 504:
      return Code.DEADLINE_EXCEEDED;
    default:
      return Code.INTERNAL;
  }
}

/**
 * Reports whether host refers to the local machine: the literal "localhost",
 * or any loopback IP (127.0.0.0/8, ::1). The URL's hostname has already been
 * stripped of port and any IPv6 brackets.
 */
function isLoopbackHost(host) {
  if (host === "localhost" || host === "::1") return true;
  if (isIPv4(host)) return host.split(".")[0] === "127";
  return false;
}

const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

/**
 * ThreadyClient is a typed client for the Thready `/v1` API. It is safe to
 * reuse across concurrent calls.
 */
export class ThreadyClient {
  /**
   * @param {{baseUrl: string, accessToken?: string, apiKey?: string,
   *          timeoutMs?: number, allowInsecureHttp?: boolean}} config
   */
  constructor({
    baseUrl,
    accessToken = "",
    apiKey = "",
    timeoutMs = DEFAULT_TIMEOUT_MS,
    allowInsecureHttp = false,
  } = {}) {
    const base = String(baseUrl ?? "").trim().replace(/\/+$/, "");
    if (base === "") {
      throw new Error("thready: baseUrl is required");
    }
    // Validate the origin up front so a malformed baseUrl fails loudly.
    this._base = new URL(base);

    this._accessToken = accessToken || "";
    this._apiKey = apiKey || "";
    this._timeoutMs = timeoutMs > 0 ? timeoutMs : DEFAULT_TIMEOUT_MS;
    this._allowInsecureHttp = Boolean(allowInsecureHttp);

    // Retry/backoff tuning. Exposed as fields so tests can shrink the sleeps.
    this._maxRetries = DEFAULT_MAX_RETRIES;
    this._backoffBaseMs = DEFAULT_BACKOFF_BASE_MS;
    this._backoffMaxMs = DEFAULT_BACKOFF_MAX_MS;
  }

  /** The token the client currently authenticates with (set at construction or by login()). */
  get accessToken() {
    return this._accessToken;
  }

  // ----- public methods over the /v1 surface -----

  /**
   * Exchange credentials (plus TOTP for admin tiers) for a token pair and store
   * the returned access token on the client so later calls authenticate
   * automatically. POST /v1/auth/login.
   * @param {{email: string, password: string, totp?: string}} req
   */
  async login({ email, password, totp } = {}) {
    const body = { email, password };
    if (totp !== undefined && totp !== null && totp !== "") body.totp = totp;
    const tp = await this._do("POST", "/v1/auth/login", { body });
    if (tp && tp.access_token) this._accessToken = tp.access_token;
    return tp;
  }

  /** List the channels registered for the caller's tenant. GET /v1/channels. */
  async listChannels() {
    const env = await this._do("GET", "/v1/channels");
    return (env && env.data) || [];
  }

  /**
   * Register a channel/group to read. Unsafe POST → carries an Idempotency-Key
   * (auto UUIDv4 unless one is passed). POST /v1/channels.
   * @param {{name: string, platform?: string, externalRef?: string}} input
   * @param {{idempotencyKey?: string}} [opts]
   */
  async createChannel({ name, platform, externalRef } = {}, opts = {}) {
    const body = { name };
    if (platform !== undefined) body.platform = platform;
    if (externalRef !== undefined) body.external_ref = externalRef;
    return this._do("POST", "/v1/channels", {
      body,
      idempotencyKey: opts.idempotencyKey || randomUUID(),
    });
  }

  /** Fetch a single post by id. GET /v1/posts/{postId}. */
  async getPost(postId) {
    return this._do("GET", `/v1/posts/${encodeURIComponent(postId)}`);
  }

  /**
   * Force a fresh processing run for a post and return the queued job
   * (202 Accepted). Unsafe POST → carries an Idempotency-Key.
   * POST /v1/posts/{postId}/reprocess.
   * @param {string} postId
   * @param {{idempotencyKey?: string}} [opts]
   */
  async reprocess(postId, opts = {}) {
    return this._do(
      "POST",
      `/v1/posts/${encodeURIComponent(postId)}/reprocess`,
      { idempotencyKey: opts.idempotencyKey || randomUUID() },
    );
  }

  /**
   * Run a semantic / keyword / hybrid search over posts and generated
   * materials. POST /v1/search.
   * @param {{query: string, mode?: string, topK?: number,
   *          sources?: string[], rerank?: boolean}} req
   */
  async search({ query, mode, topK, sources, rerank } = {}) {
    const body = { query, rerank: Boolean(rerank) };
    if (mode !== undefined && mode !== "") body.mode = mode;
    if (Array.isArray(sources) && sources.length > 0) body.sources = sources;
    if (typeof topK === "number" && topK > 0) body.top_k = topK;
    return this._do("POST", "/v1/search", { body });
  }

  /** List the Skill-Graph knowledge units. GET /v1/skills. */
  async listSkills() {
    const env = await this._do("GET", "/v1/skills");
    return (env && env.data) || [];
  }

  // ----- internals -----

  /**
   * Decide whether it is safe to attach a credential to a request bound for
   * `urlObj`. https (or any non-http scheme) is always fine; plaintext http is
   * allowed only to a loopback host — or unconditionally when allowInsecureHttp
   * was opted into.
   */
  _transportAllowed(urlObj) {
    if (this._allowInsecureHttp) return true;
    if (urlObj.protocol !== "http:") return true; // https and other schemes are safe
    return isLoopbackHost(urlObj.hostname);
  }

  /** Build the outgoing header set, enforcing the credential-transport policy. */
  _buildHeaders(urlObj, { hasBody, idempotencyKey }) {
    const headers = { Accept: "application/json" };
    if (hasBody) headers["Content-Type"] = "application/json";
    if (idempotencyKey) headers["Idempotency-Key"] = idempotencyKey;

    const hasCredential = this._accessToken !== "" || this._apiKey !== "";
    if (hasCredential && !this._transportAllowed(urlObj)) {
      throw new InsecureTransportError();
    }
    // Bearer wins over API key.
    if (this._accessToken !== "") {
      headers["Authorization"] = `Bearer ${this._accessToken}`;
    } else if (this._apiKey !== "") {
      headers["X-API-Key"] = this._apiKey;
    }
    return headers;
  }

  /**
   * Perform a request: JSON encode/decode, auth injection, optional
   * Idempotency-Key, typed error mapping, and — for idempotent GETs — capped
   * exponential-backoff retries on 503/429 and transient transport errors.
   */
  async _do(method, path, { body = null, idempotencyKey = "" } = {}) {
    const urlObj = new URL(this._base.toString().replace(/\/+$/, "") + path);
    const bodyBytes =
      body !== null && body !== undefined
        ? Buffer.from(JSON.stringify(body), "utf8")
        : null;

    // Enforce the credential-transport policy ONCE, before any send: a refusal
    // here means no header was attached and no request left the process. The
    // decision does not change across retry attempts.
    const headers = this._buildHeaders(urlObj, {
      hasBody: bodyBytes !== null,
      idempotencyKey,
    });

    const attempts = method === "GET" ? this._maxRetries + 1 : 1;
    let lastErr = null;

    for (let attempt = 0; attempt < attempts; attempt++) {
      if (attempt > 0) await sleep(this._backoff(attempt));

      let res;
      try {
        res = await this._send(urlObj, method, headers, bodyBytes);
      } catch (err) {
        lastErr = err;
        if (method === "GET" && attempt < attempts - 1) continue; // transient transport error
        throw err;
      }

      // Retry idempotent GETs on transient upstream unavailability.
      if (
        method === "GET" &&
        attempt < attempts - 1 &&
        (res.statusCode === 503 || res.statusCode === 429)
      ) {
        lastErr = parseApiError(res);
        continue;
      }

      return decodeResponse(res);
    }
    throw lastErr;
  }

  /** Capped exponential backoff in ms for retry `attempt` (1-based). */
  _backoff(attempt) {
    const d = this._backoffBaseMs * 2 ** (attempt - 1);
    return d > this._backoffMaxMs || d <= 0 ? this._backoffMaxMs : d;
  }

  /** Low-level single HTTP send over node:http(s). Resolves {statusCode, headers, bodyText}. */
  _send(urlObj, method, headers, bodyBytes) {
    return new Promise((resolve, reject) => {
      const isHttps = urlObj.protocol === "https:";
      const mod = isHttps ? https : http;
      const options = {
        method,
        hostname: urlObj.hostname,
        port: urlObj.port || (isHttps ? 443 : 80),
        path: urlObj.pathname + urlObj.search,
        headers,
      };
      // AbortSignal.timeout bounds the WHOLE request — including the TCP/TLS
      // connect phase (req.setTimeout only arms after the socket connects, so it
      // cannot cap a hung connect to an unreachable host).
      if (this._timeoutMs > 0) options.signal = AbortSignal.timeout(this._timeoutMs);

      const req = mod.request(options, (res) => {
        const chunks = [];
        res.on("data", (c) => chunks.push(c));
        res.on("end", () =>
          resolve({
            statusCode: res.statusCode,
            headers: res.headers,
            bodyText: Buffer.concat(chunks).toString("utf8"),
          }),
        );
      });

      req.on("error", (err) => {
        const aborted =
          err && (err.name === "AbortError" || err.name === "TimeoutError" || err.code === "ABORT_ERR");
        const detail = aborted ? `timed out after ${this._timeoutMs}ms` : err.message;
        reject(new Error(`thready: ${method} ${urlObj.pathname}: ${detail}`));
      });

      if (bodyBytes) req.write(bodyBytes);
      req.end();
    });
  }
}

/** Render a response: a 2xx parses its JSON body (or null for empty/204); anything else becomes an ApiError. */
function decodeResponse(res) {
  const { statusCode, bodyText } = res;
  if (statusCode >= 200 && statusCode < 300) {
    if (statusCode === 204 || bodyText.trim() === "") return null;
    try {
      return JSON.parse(bodyText);
    } catch (err) {
      throw new Error(`thready: decode response body: ${err.message}`);
    }
  }
  throw parseApiError(res);
}

/**
 * Map a non-2xx response to a typed ApiError. Prefers the canonical
 * {"error":{code,message,status,request_id,…}} envelope, backfilling any
 * missing status/request_id from the HTTP status line and headers, and degrades
 * gracefully to a status-derived error for a non-envelope body.
 */
function parseApiError(res) {
  const { statusCode, headers, bodyText } = res;
  const headerRequestId = headers["x-request-id"] || "";

  let env = null;
  try {
    env = JSON.parse(bodyText);
  } catch {
    env = null;
  }

  if (env && env.error && typeof env.error === "object" && env.error.code) {
    const e = env.error;
    return new ApiError({
      code: e.code,
      message: e.message || "",
      status: e.status || statusCode,
      requestId: e.request_id || headerRequestId,
      traceId: e.trace_id || "",
      retryAfter: e.retry_after ?? null,
      details: Array.isArray(e.details) ? e.details : [],
    });
  }

  const message = bodyText.trim() || `HTTP ${statusCode}`;
  return new ApiError({
    code: codeForStatus(statusCode),
    message,
    status: statusCode,
    requestId: headerRequestId,
  });
}

export default ThreadyClient;

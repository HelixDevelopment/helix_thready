// -----------------------------------------------------------------------------
//  Helix Thready — TypeScript SDK client skeleton (thin idiomatic layer)
//  Classification : PUBLIC
//  Location       : docs/public/research/mvp/development/materials/sdk/ts/client.ts
//  Status         : Draft — v0.1 SKELETON (illustrative)
//  Revision       : 1 (2026-07-22) — swarm (development/materials)
//  Sources        : ../../../../api/sdk-strategy.md (§5 thin layer, §4 typescript-fetch),
//                   ../../../../api/sdk-examples.md (TS recipes R1/R2/R5/R6/R7),
//                   ../../../../api/error-model.md, ../../../../api/event-bus-contract.md
//
//  WHAT THIS IS (read before assuming anything works)
//    The HAND-WRITTEN thin idiomatic layer wrapping the openapi-generator
//    `typescript-fetch` GENERATED core (the same generator helix_proto uses,
//    sdk-strategy.md §4). Illustrates the generated-core + hand-written-wrapper
//    pattern with: config, auth injection, ONE example call (posts.list async
//    iterator, recipe R2) and the events subscription (events.subscribe, recipe R5).
//
//    ANTI-BLUFF: This is a SKELETON — NOT a compiled, tested SDK. The generated core
//    (imported below as `./gen`) is produced by `openapi-generator` and is NOT
//    included here; the transport bodies are stubbed with clear TODOs and throw. Do
//    not claim this works. Publish is gated on `tsc --noEmit` + the round-trip test
//    going GREEN (sdk-strategy.md §6, [GAP: #11], [GAP: #18]).
//
//  LAYERING RULE (sdk-strategy.md §6 check-no-handwritten)
//    Ergonomics live ONLY in this thin layer. `./gen` is regenerated from the
//    contract and MUST NOT be hand-edited — a hand edit fails the drift gate.
//
//  Package (proposed, [DEFAULT — adjustable], sdk-examples.md §10): @helix-thready/sdk
// -----------------------------------------------------------------------------

// import * as gen from "./gen"; // GENERATED (openapi-generator typescript-fetch) — never hand-edited.

// =============================================================================
// Auth & config (recipe R1 / R7)
// =============================================================================

/** API-key credential (non-interactive automation): Authorization: Bearer sk-… */
export interface ApiKeyAuth {
  apiKey: string;
}

/** JWT credential; the client refreshes transparently before expiry (recipe R7). */
export interface JwtAuth {
  accessToken: string;
  refreshToken: string;
  /** epoch ms of access-token expiry (tokenmanager IsExpired shape, [VERIFIED]). */
  expiresAt?: number;
}

export type Auth = ApiKeyAuth | JwtAuth;

const isJwt = (a: Auth): a is JwtAuth => "accessToken" in a;

/** Rotated token pair emitted after a transparent refresh (recipe R7). */
export interface TokenPair {
  accessToken: string;
  refreshToken: string;
  expiresAt?: number;
}

/** Retry policy — retries ONLY retryable codes with back-off + jitter (error-model §3). */
export interface RetryPolicy {
  maxAttempts: number;
  baseDelayMs: number;
  factor: number;
  capMs: number;
}

export const DEFAULT_RETRY: RetryPolicy = {
  maxAttempts: 5,
  baseDelayMs: 2000,
  factor: 2.0,
  capMs: 5 * 60 * 1000,
};

export interface ThreadyConfig {
  /** REST /v1 root, e.g. "https://thready.hxd3v.com/v1". */
  baseUrl: string;
  auth: Auth;
  retry?: RetryPolicy;
  /** Optional fetch impl (defaults to global fetch). */
  fetchImpl?: typeof fetch;
}

// =============================================================================
// Typed error model (recipe R6) — maps the wire Error envelope (error-model §3)
// =============================================================================

/** Stable error codes. Unknown values map to Code.Unknown (non-breaking, versioning.md). */
export enum Code {
  InvalidArgument = "invalid_argument",
  PermissionDenied = "permission_denied",
  Conflict = "conflict", // single-claim 409 (rest-endpoints §2.6)
  RateLimited = "rate_limited", // retryable
  Unavailable = "unavailable", // retryable; also the fail-loud hash-embedder 503
  DeadlineExceeded = "deadline_exceeded", // retryable
  Unknown = "unknown",
}

const RETRYABLE = new Set<Code>([Code.RateLimited, Code.Unavailable, Code.DeadlineExceeded]);

/** Idiomatic typed error carrying the stable code, trace id and structured details. */
export class ThreadyError extends Error {
  constructor(
    readonly code: Code,
    message: string,
    readonly traceId: string,
    /** seconds, from Retry-After (undefined if absent). */
    readonly retryAfter?: number,
    readonly details?: Array<Record<string, unknown>>,
  ) {
    super(message);
    this.name = "ThreadyError";
  }
}

// =============================================================================
// Domain types (illustrative subsets of the generated DTOs)
// =============================================================================

export interface Post {
  id: string;
  hashtags: string[];
  categories: string[];
}

export interface PostFilter {
  channelId?: string;
  /** processing enum: pending|running|succeeded|failed|skipped */
  status?: string;
  /** 1..200 */
  limit?: number;
}

export interface SearchRequest {
  query: string;
  mode?: "semantic" | "keyword" | "hybrid";
  sources?: string[];
  topK?: number;
  rerank?: boolean;
}

export interface SearchResult {
  /** MUST be a real provider; "hash" means the HashEmbedder stub is active ([GAP: #1]). */
  embedder: string;
  results: Array<{ sourceId: string; score: number; snippet: string }>;
}

export interface EventEnvelope {
  id: string;
  type: string;
  payload: Record<string, unknown>;
}

export interface EventFilter {
  /** glob supported, e.g. ["processing.*", "asset.ready"] (filter.ByGlob, [VERIFIED]). */
  types: string[];
  /** replay the last sticky value on connect. */
  replaySticky?: boolean;
}

// =============================================================================
// Client
// =============================================================================

type TokenRotatedListener = (pair: TokenPair) => void;

export class ThreadyClient {
  private readonly cfg: Required<Pick<ThreadyConfig, "baseUrl" | "auth">> & ThreadyConfig;
  private readonly fetchImpl: typeof fetch;
  private auth: Auth;
  private tokenRotatedListeners: TokenRotatedListener[] = [];

  readonly posts: PostsService;
  readonly search: SearchService;
  readonly events: EventsService;

  constructor(config: ThreadyConfig) {
    if (!config.baseUrl) throw new Error("ThreadyClient: baseUrl is required");
    if (!config.auth) throw new Error("ThreadyClient: auth is required ({ apiKey } or { accessToken, refreshToken })");
    this.cfg = { retry: DEFAULT_RETRY, ...config };
    this.fetchImpl = config.fetchImpl ?? fetch;
    this.auth = config.auth;
    this.posts = new PostsService(this);
    this.search = new SearchService(this);
    this.events = new EventsService(this);
  }

  /** Subscribe to transparent-refresh rotations so callers can persist the new pair (R7). */
  on(event: "tokenRotated", listener: TokenRotatedListener): void {
    if (event === "tokenRotated") this.tokenRotatedListeners.push(listener);
  }

  /**
   * INTERNAL: build auth headers, refreshing a near-expiry JWT first (recipe R1/R7).
   * The generated-core request functions call this to obtain the Authorization header.
   */
  async authHeaders(): Promise<Record<string, string>> {
    if (isJwt(this.auth)) {
      const jwt = this.auth;
      if (jwt.expiresAt !== undefined && jwt.expiresAt - Date.now() < 30_000) {
        const rotated = await this.refresh(jwt); // POST /auth/refresh; old refresh revoked server-side.
        this.auth = rotated;
        for (const l of this.tokenRotatedListeners) l(rotated);
      }
      return { Authorization: `Bearer ${(this.auth as JwtAuth).accessToken}` };
    }
    return { Authorization: `Bearer ${this.auth.apiKey}` };
  }

  private async refresh(_jwt: JwtAuth): Promise<TokenPair> {
    // TODO(skeleton): POST {baseUrl}/auth/refresh with the refresh token (authn-authz §3). Not wired.
    throw new ThreadyError(Code.Unknown, "JWT refresh not implemented in skeleton", "");
  }

  /** INTERNAL: config accessors for the service classes. */
  get baseUrl(): string {
    return this.cfg.baseUrl;
  }
  get retry(): RetryPolicy {
    return this.cfg.retry ?? DEFAULT_RETRY;
  }
  get fetcher(): typeof fetch {
    return this.fetchImpl;
  }
}

// =============================================================================
// Posts service — ONE example call (recipe R2: cursor pagination as async iterator)
// =============================================================================

export class PostsService {
  constructor(private readonly c: ThreadyClient) {}

  /**
   * List posts as an async iterator that hides meta.next_cursor (recipe R2):
   *
   *   for await (const post of client.posts.list({ channelId, status: "failed", limit: 100 })) {
   *     console.log(post.id, post.categories);
   *   }
   */
  async *list(filter: PostFilter = {}): AsyncGenerator<Post, void, void> {
    let cursor: string | undefined;
    // eslint-disable-next-line no-constant-condition
    while (true) {
      const { data, nextCursor } = await this.listPage(filter, cursor);
      for (const p of data) yield p;
      if (!nextCursor) return;
      cursor = nextCursor;
    }
  }

  private async listPage(
    _filter: PostFilter,
    _cursor: string | undefined,
  ): Promise<{ data: Post[]; nextCursor?: string }> {
    // TODO(skeleton): call the generated `listPosts` op with await this.c.authHeaders(),
    // apply the retry policy, map non-2xx bodies to ThreadyError, and return
    // { data: body.data, nextCursor: body.meta.next_cursor }. Not wired.
    throw new ThreadyError(Code.Unknown, "posts.list not implemented in skeleton", "");
  }
}

// =============================================================================
// Search service (recipe R4 stub — fail-loud embedder guard)
// =============================================================================

export class SearchService {
  constructor(private readonly c: ThreadyClient) {}

  /** Semantic/keyword/hybrid search. Callers SHOULD reject `res.embedder === "hash"` (R4). */
  async query(_req: SearchRequest): Promise<SearchResult> {
    // TODO(skeleton): call generated `search` op; a 503 maps to ThreadyError(Code.Unavailable)
    // — the fail-loud signal that the HashEmbedder stub is active ([GAP: #1]). Not wired.
    throw new ThreadyError(Code.Unknown, "search.query not implemented in skeleton", "");
  }
}

// =============================================================================
// Events service — the events subscription (recipe R5)
// =============================================================================

/** Live subscription: EventEmitter-style `.on(type, cb)` + `.close()`. Auto-reconnects. */
export class Subscription {
  private handlers = new Map<string, Array<(ev: EventEnvelope) => void>>();
  private closed = false;

  /** Register a handler for an event type (exact match; glob filtering is server-side). */
  on(type: string, handler: (ev: EventEnvelope) => void): this {
    const list = this.handlers.get(type) ?? [];
    list.push(handler);
    this.handlers.set(type, list);
    return this;
  }

  /** INTERNAL: dispatch a decoded frame to matching handlers. */
  dispatch(ev: EventEnvelope): void {
    for (const h of this.handlers.get(ev.type) ?? []) h(ev);
  }

  /** Acknowledge an event to advance the durable cursor (event-bus-contract §7). */
  async ack(_id: string): Promise<void> {
    /* TODO(skeleton): send ack frame / persist cursor. Not wired. */
  }

  close(): void {
    this.closed = true;
    /* TODO(skeleton): tear down the WS/SSE connection + reconnect loop. */
  }

  get isClosed(): boolean {
    return this.closed;
  }
}

export class EventsService {
  constructor(private readonly c: ThreadyClient) {}

  /**
   * Subscribe over SSE (browser) or WS (node); auto-reconnect + sticky reconcile (R5):
   *
   *   const sub = client.events.subscribe({ types: ["processing.*", "asset.ready"], replaySticky: true });
   *   sub.on("processing.completed", ev => console.log("done", ev.payload.post_id));
   */
  subscribe(_filter: EventFilter): Subscription {
    const sub = new Subscription();
    // TODO(skeleton): open WS/SSE to {baseUrl}/events with await this.c.authHeaders(),
    // send the EventFilter, then run a reconnect loop that: (a) on connect replays the
    // sticky value when replaySticky, (b) decodes frames into EventEnvelope and calls
    // sub.dispatch(ev), (c) on drop reconnects from the last ack'd id with back-off,
    // reconciling via getStickyEvent after long outages. Not wired.
    return sub;
  }
}

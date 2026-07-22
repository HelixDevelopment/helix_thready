// Hand-written TypeScript typings for the Helix Thready /v1 SDK.
//
// The runtime is authored in ESM (client.mjs), so these declarations are the
// "TypeScript" deliverable — no transpile/build step is required to run the
// client or its tests. Field names on the returned DTOs are the wire shapes
// (snake_case) the gateway actually emits, matching the sibling Go SDK's JSON
// tags (implementation/sdk_go) so a decode needs no transformation layer.

/** Stable, machine-readable error codes (mirror the gateway's canonical taxonomy). */
export type CodeValue =
  | "invalid_argument"
  | "unprocessable"
  | "unauthenticated"
  | "permission_denied"
  | "not_found"
  | "already_exists"
  | "conflict"
  | "failed_precondition"
  | "rate_limited"
  | "deadline_exceeded"
  | "unavailable"
  | "internal";

export const Code: {
  readonly INVALID_ARGUMENT: "invalid_argument";
  readonly UNPROCESSABLE: "unprocessable";
  readonly UNAUTHENTICATED: "unauthenticated";
  readonly PERMISSION_DENIED: "permission_denied";
  readonly NOT_FOUND: "not_found";
  readonly ALREADY_EXISTS: "already_exists";
  readonly CONFLICT: "conflict";
  readonly FAILED_PRECONDITION: "failed_precondition";
  readonly RATE_LIMITED: "rate_limited";
  readonly DEADLINE_EXCEEDED: "deadline_exceeded";
  readonly UNAVAILABLE: "unavailable";
  readonly INTERNAL: "internal";
};

/** A structured, machine-usable reason attached to an error. */
export interface ErrorDetail {
  field?: string;
  issue?: string;
  reason?: string;
}

/**
 * Thrown instead of attaching a credential to a plaintext-http request bound
 * for a non-loopback host (unless allowInsecureHttp is set).
 */
export class InsecureTransportError extends Error {
  readonly name: "InsecureTransportError";
  constructor(message?: string);
}

/** The typed error surfaced for every non-2xx response. */
export class ApiError extends Error {
  readonly name: "ApiError";
  readonly code: CodeValue;
  readonly status: number;
  readonly requestId: string;
  readonly traceId: string;
  readonly retryAfter: number | null;
  readonly details: ErrorDetail[];
  constructor(fields?: {
    code?: CodeValue;
    message?: string;
    status?: number;
    requestId?: string;
    traceId?: string;
    retryAfter?: number | null;
    details?: ErrorDetail[];
  });
  /** Whether the error's code is one the SDK considers transiently retryable. */
  retryable(): boolean;
  toString(): string;
}

// ----- request inputs -----

export interface ThreadyClientConfig {
  /** Gateway origin, e.g. "https://thready.hxd3v.com" (trailing slash trimmed). Required. */
  baseUrl: string;
  /** JWT bearer access token → sent as "Authorization: Bearer …". */
  accessToken?: string;
  /** Scoped API key → sent as "X-API-Key: …" (for non-interactive use). */
  apiKey?: string;
  /** Per-request timeout in milliseconds (default 30000). */
  timeoutMs?: number;
  /** Permit attaching credentials over plaintext http to a non-loopback host. Default false. */
  allowInsecureHttp?: boolean;
}

export interface LoginRequest {
  email: string;
  password: string;
  /** Required for admin tiers; omitted for standard users. */
  totp?: string;
}

export interface CreateChannelInput {
  name: string;
  platform?: string;
  externalRef?: string;
}

export interface SearchRequest {
  query: string;
  /** One of "semantic" | "keyword" | "hybrid". */
  mode?: string;
  topK?: number;
  /** Corpora to search: e.g. "posts" | "generated" | "assets". */
  sources?: string[];
  rerank?: boolean;
}

export interface IdempotencyOptions {
  /** Override the auto-generated Idempotency-Key on an unsafe POST. */
  idempotencyKey?: string;
}

// ----- response DTOs (wire shapes, snake_case) -----

export interface TokenPair {
  access_token: string;
  refresh_token: string;
  token_type: string;
  expires_in: number;
  refresh_expires_in: number;
}

export interface Channel {
  id: string;
  account_id: string;
  name: string;
  platform: string;
  external_ref: string;
  created_at: string;
}

export interface Post {
  id: string;
  channel_id: string;
  account_id: string;
  body: string;
  hashtags: string[];
  categories: string[];
  status: string;
  created_at: string;
}

export interface Job {
  job_id: string;
  post_id: string;
  status: string;
  precedence: string[];
  queued_at: string;
}

export interface SearchHit {
  source_id: string;
  kind: string;
  score: number;
  span: string | null;
  snippet: string;
}

export interface SearchResults {
  results: SearchHit[];
  took_ms: number;
  embedder: string;
}

export interface Skill {
  id: string;
  name: string;
  kind: string;
  sort_order: number;
}

/** A typed client for the Thready `/v1` API. Safe to reuse across concurrent calls. */
export class ThreadyClient {
  constructor(config: ThreadyClientConfig);
  /** The token the client currently authenticates with. */
  readonly accessToken: string;

  /** POST /v1/auth/login — stores the returned access token on the client. */
  login(req: LoginRequest): Promise<TokenPair>;
  /** GET /v1/channels. */
  listChannels(): Promise<Channel[]>;
  /** POST /v1/channels (sends an Idempotency-Key). */
  createChannel(input: CreateChannelInput, opts?: IdempotencyOptions): Promise<Channel>;
  /** GET /v1/posts/{postId}. */
  getPost(postId: string): Promise<Post>;
  /** POST /v1/posts/{postId}/reprocess (sends an Idempotency-Key). */
  reprocess(postId: string, opts?: IdempotencyOptions): Promise<Job>;
  /** POST /v1/search. */
  search(req: SearchRequest): Promise<SearchResults>;
  /** GET /v1/skills. */
  listSkills(): Promise<Skill[]>;
}

export default ThreadyClient;

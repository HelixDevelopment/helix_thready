package digital.vasic.thready;

import java.io.IOException;
import java.net.InetAddress;
import java.net.URI;
import java.net.URLEncoder;
import java.net.UnknownHostException;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.nio.charset.StandardCharsets;
import java.time.Duration;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.UUID;
import java.util.function.Function;

/**
 * ThreadyClient is the stdlib-only Java SDK client for the Helix Thready REST
 * {@code /v1} control API (docs/public/research/mvp/api/openapi.yaml; realized by the
 * implementation/rest_gateway module and mirrored from implementation/sdk_go).
 *
 * <p>It injects auth (a JWT bearer access token OR an {@code X-API-Key}; bearer wins),
 * encodes/decodes JSON via the hand-rolled {@link Json} codec, maps every non-2xx
 * response to a typed {@link ApiException}, retries idempotent GETs on transient
 * 503/429 with capped exponential backoff, and stamps a fresh {@code Idempotency-Key}
 * onto unsafe POSTs. Before sending, it refuses to attach a credential over plaintext
 * http to a non-loopback host (see {@link InsecureTransportException}).
 *
 * <p>Built on the JDK's {@link java.net.http.HttpClient}; imports no sibling module.
 */
public final class ThreadyClient {

    private final String baseUrl;
    private final String apiKey;
    private final boolean allowInsecureHttp;
    private final HttpClient http;

    /** Bearer access token; refreshed in place by a successful {@link #login}. */
    private volatile String accessToken;

    // Retry tuning (package-private so tests can shrink the sleeps). Idempotent GETs
    // get maxRetries extra attempts on 503/429; unsafe methods never retry.
    int maxRetries = 3;
    long backoffBaseMillis = 25;
    long backoffMaxMillis = 2000;

    /**
     * Construct a client.
     *
     * @param baseUrl           gateway origin, e.g. {@code https://thready.hxd3v.com} or
     *                          {@code https://thready.hxd3v.com/v1} or {@code http://127.0.0.1:8080};
     *                          a trailing {@code /} and a trailing {@code /v1} are normalized away.
     *                          Required.
     * @param accessToken       JWT bearer access token, or null.
     * @param apiKey            scoped API key (sent as {@code X-API-Key}), or null.
     * @param allowInsecureHttp when true, permits attaching a credential to a plaintext-http
     *                          request bound for a non-loopback host (default false refuses it).
     */
    public ThreadyClient(String baseUrl, String accessToken, String apiKey, boolean allowInsecureHttp) {
        String b = baseUrl == null ? "" : baseUrl.trim();
        while (b.endsWith("/")) {
            b = b.substring(0, b.length() - 1);
        }
        if (b.endsWith("/v1")) {
            b = b.substring(0, b.length() - "/v1".length());
        }
        while (b.endsWith("/")) {
            b = b.substring(0, b.length() - 1);
        }
        if (b.isEmpty()) {
            throw new IllegalArgumentException("thready: baseUrl is required");
        }
        this.baseUrl = b;
        this.accessToken = (accessToken == null || accessToken.isEmpty()) ? null : accessToken;
        this.apiKey = (apiKey == null || apiKey.isEmpty()) ? null : apiKey;
        this.allowInsecureHttp = allowInsecureHttp;
        this.http = HttpClient.newBuilder()
                .connectTimeout(Duration.ofSeconds(10))
                .followRedirects(HttpClient.Redirect.NORMAL)
                .build();
    }

    /** The token this client currently authenticates with (set at construction or by {@link #login}). */
    public String getAccessToken() {
        return accessToken;
    }

    // ----------------------------------------------------------------- methods

    /**
     * Exchange credentials (plus TOTP for admin tiers) for a token pair and store the
     * returned access token so subsequent calls authenticate automatically.
     * {@code POST /v1/auth/login}.
     */
    public TokenPair login(String email, String password, String totp) {
        Map<String, Object> body = new LinkedHashMap<>();
        body.put("email", email);
        body.put("password", password);
        if (totp != null && !totp.isEmpty()) {
            body.put("totp", totp);
        }
        String resp = execute("POST", "/v1/auth/login", null, Json.write(body), null);
        TokenPair tp = TokenPair.fromMap(requireObject(resp));
        if (tp.accessToken() != null && !tp.accessToken().isEmpty()) {
            this.accessToken = tp.accessToken();
        }
        return tp;
    }

    /** List the channels registered for the caller's tenant. {@code GET /v1/channels}. */
    public List<Channel> listChannels() {
        String resp = execute("GET", "/v1/channels", null, null, null);
        return decodeList(resp, Channel::fromMap);
    }

    /**
     * Register a channel/group to read. An unsafe POST: carries an auto-generated
     * {@code Idempotency-Key}. {@code POST /v1/channels}.
     */
    public Channel createChannel(String name, String platform, String externalRef) {
        return createChannel(name, platform, externalRef, null);
    }

    /**
     * Register a channel with an explicit {@code Idempotency-Key} override (reusing a
     * key with the same body replays the original result; a same-key/different-body
     * request is a 409 conflict).
     */
    public Channel createChannel(String name, String platform, String externalRef, String idempotencyKey) {
        Map<String, Object> body = new LinkedHashMap<>();
        body.put("name", name);
        body.put("platform", platform);
        body.put("external_ref", externalRef);
        String key = (idempotencyKey != null && !idempotencyKey.isEmpty())
                ? idempotencyKey : UUID.randomUUID().toString();
        String resp = execute("POST", "/v1/channels", null, Json.write(body), key);
        return Channel.fromMap(requireObject(resp));
    }

    /** Fetch a single post by id. {@code GET /v1/posts/{postId}}. */
    public Post getPost(String postId) {
        String resp = execute("GET", "/v1/posts/" + enc(postId), null, null, null);
        return Post.fromMap(requireObject(resp));
    }

    /**
     * Force a fresh processing run for a post and return the queued job (202 Accepted).
     * An unsafe POST: carries an auto-generated {@code Idempotency-Key}.
     * {@code POST /v1/posts/{postId}/reprocess}.
     */
    public ProcessingJob reprocess(String postId) {
        return reprocess(postId, null);
    }

    /** Reprocess with an explicit {@code Idempotency-Key} override. */
    public ProcessingJob reprocess(String postId, String idempotencyKey) {
        String key = (idempotencyKey != null && !idempotencyKey.isEmpty())
                ? idempotencyKey : UUID.randomUUID().toString();
        String resp = execute("POST", "/v1/posts/" + enc(postId) + "/reprocess", null, null, key);
        return ProcessingJob.fromMap(requireObject(resp));
    }

    /**
     * Run a semantic / keyword / hybrid search over posts and generated materials.
     * {@code POST /v1/search}.
     *
     * @param query   free-text query (required)
     * @param mode    one of {@code semantic|keyword|hybrid}; omitted when null/empty
     * @param topK    max results; omitted when &le; 0
     * @param sources corpora to search ({@code posts|generated|assets}); omitted when null
     * @param rerank  whether to rerank the fused result set
     */
    public SearchResult search(String query, String mode, int topK, List<String> sources, boolean rerank) {
        Map<String, Object> body = new LinkedHashMap<>();
        body.put("query", query);
        if (mode != null && !mode.isEmpty()) {
            body.put("mode", mode);
        }
        if (sources != null) {
            body.put("sources", sources);
        }
        if (topK > 0) {
            body.put("top_k", topK);
        }
        body.put("rerank", rerank);
        String resp = execute("POST", "/v1/search", null, Json.write(body), null);
        return SearchResult.fromMap(requireObject(resp));
    }

    /** List the Skill-Graph knowledge units. {@code GET /v1/skills}. */
    public List<Skill> listSkills() {
        String resp = execute("GET", "/v1/skills", null, null, null);
        return decodeList(resp, Skill::fromMap);
    }

    // ------------------------------------------------------------ HTTP engine

    /**
     * Perform a request with JSON body, auth injection, an optional Idempotency-Key,
     * typed error mapping, and — for idempotent GETs — capped exponential-backoff
     * retries on 503/429 and transient transport errors. Returns the raw 2xx body.
     */
    private String execute(String method, String path, String query, String bodyJson, String idempotencyKey) {
        String url = baseUrl + path + (query != null && !query.isEmpty() ? "?" + query : "");
        URI uri = URI.create(url);
        int attempts = method.equals("GET") ? maxRetries + 1 : 1;
        RuntimeException lastErr = null;

        for (int attempt = 0; attempt < attempts; attempt++) {
            if (attempt > 0) {
                sleepBackoff(attempt);
            }

            HttpRequest.Builder rb = HttpRequest.newBuilder(uri).timeout(Duration.ofSeconds(30));
            rb.header("Accept", "application/json");
            if (bodyJson != null) {
                rb.header("Content-Type", "application/json");
            }
            if (idempotencyKey != null) {
                rb.header("Idempotency-Key", idempotencyKey);
            }
            // Enforce the credential-transport policy BEFORE any send: a refusal here
            // throws InsecureTransportException and no request leaves the process.
            applyAuth(rb, uri);

            HttpRequest.BodyPublisher pub = bodyJson != null
                    ? HttpRequest.BodyPublishers.ofString(bodyJson, StandardCharsets.UTF_8)
                    : HttpRequest.BodyPublishers.noBody();
            rb.method(method, pub);

            HttpResponse<String> resp;
            try {
                resp = http.send(rb.build(), HttpResponse.BodyHandlers.ofString(StandardCharsets.UTF_8));
            } catch (InterruptedException ie) {
                Thread.currentThread().interrupt();
                throw new RuntimeException("thready: interrupted during " + method + " " + path, ie);
            } catch (IOException io) {
                lastErr = new RuntimeException("thready: " + method + " " + path + ": " + io.getMessage(), io);
                if (method.equals("GET") && attempt < attempts - 1) {
                    continue; // transient transport error on an idempotent GET
                }
                throw lastErr;
            }

            int sc = resp.statusCode();
            if (method.equals("GET") && attempt < attempts - 1 && (sc == 503 || sc == 429)) {
                lastErr = parseError(resp);
                continue;
            }
            if (sc >= 200 && sc < 300) {
                return resp.body();
            }
            throw parseError(resp);
        }
        throw lastErr; // GET retries exhausted
    }

    /**
     * Inject the credential: a bearer JWT when present, otherwise an X-API-Key. When a
     * credential is present, first enforce the transport policy — refuse (throw) rather
     * than attach it to plaintext http bound for a non-loopback host.
     */
    private void applyAuth(HttpRequest.Builder rb, URI uri) {
        String tok = this.accessToken;
        boolean hasCredential = (tok != null && !tok.isEmpty()) || apiKey != null;
        if (hasCredential && !isCredentialTransportAllowed(uri)) {
            throw new InsecureTransportException(
                    "thready: refusing to send credentials over plaintext http to non-loopback host \""
                            + uri.getHost() + "\"; use https or construct with allowInsecureHttp=true");
        }
        if (tok != null && !tok.isEmpty()) {
            rb.header("Authorization", "Bearer " + tok);
        } else if (apiKey != null) {
            rb.header("X-API-Key", apiKey);
        }
    }

    /**
     * Whether it is safe to attach a credential to a request bound for {@code uri}.
     * https (or any non-http scheme) is always fine; plaintext http is allowed only to
     * a loopback host — or unconditionally when allowInsecureHttp was opted into.
     * Package-private for direct testing.
     */
    boolean isCredentialTransportAllowed(URI uri) {
        if (allowInsecureHttp) {
            return true;
        }
        String scheme = uri.getScheme();
        if (scheme == null || !scheme.equalsIgnoreCase("http")) {
            return true; // https and other non-plaintext schemes are safe
        }
        return isLoopbackHost(uri.getHost());
    }

    /**
     * Whether host refers to the local machine: the literal "localhost", or any address
     * that resolves to a loopback address (127.0.0.0/8, ::1). IP literals resolve without
     * a DNS lookup; an unresolvable hostname is treated as non-loopback.
     */
    static boolean isLoopbackHost(String host) {
        if (host == null) {
            return false;
        }
        if (host.equalsIgnoreCase("localhost")) {
            return true;
        }
        try {
            return InetAddress.getByName(host).isLoopbackAddress();
        } catch (UnknownHostException e) {
            return false;
        }
    }

    private void sleepBackoff(int attempt) {
        long d = backoffBaseMillis << (attempt - 1);
        if (d > backoffMaxMillis || d <= 0) {
            d = backoffMaxMillis;
        }
        try {
            Thread.sleep(d);
        } catch (InterruptedException e) {
            Thread.currentThread().interrupt();
        }
    }

    /**
     * Map a non-2xx response to a typed {@link ApiException}, preferring the canonical
     * {@code {"error":{code,message,status,request_id,trace_id}}} envelope and
     * backfilling any missing status/request_id from the HTTP status line and headers.
     */
    private ApiException parseError(HttpResponse<String> resp) {
        int status = resp.statusCode();
        String requestIdHeader = resp.headers().firstValue("X-Request-Id").orElse("");
        String body = resp.body();
        try {
            Map<String, Object> parsed = Json.asMap(Json.parse(body));
            if (parsed != null) {
                Map<String, Object> err = Json.asMap(parsed.get("error"));
                if (err != null) {
                    String code = Json.str(err.get("code"));
                    String message = Json.str(err.get("message"));
                    int st = Json.intVal(err.get("status"), 0);
                    String reqId = Json.str(err.get("request_id"));
                    String traceId = Json.str(err.get("trace_id"));
                    if (st == 0) {
                        st = status;
                    }
                    if (reqId == null || reqId.isEmpty()) {
                        reqId = requestIdHeader;
                    }
                    if (code == null || code.isEmpty()) {
                        code = codeForStatus(status);
                    }
                    return new ApiException(code, message != null ? message : "", st, reqId,
                            traceId != null ? traceId : "");
                }
            }
        } catch (RuntimeException ignore) {
            // Non-envelope / non-JSON body: fall through to a status-derived error.
        }
        String msg = body == null ? "" : body.trim();
        if (msg.isEmpty()) {
            msg = "HTTP " + status;
        }
        return new ApiException(codeForStatus(status), msg, status, requestIdHeader, "");
    }

    /** Map an HTTP status to the canonical code for a non-envelope error body. */
    private static String codeForStatus(int status) {
        return switch (status) {
            case 400 -> "invalid_argument";
            case 401 -> "unauthenticated";
            case 403 -> "permission_denied";
            case 404 -> "not_found";
            case 409 -> "conflict";
            case 412 -> "failed_precondition";
            case 422 -> "unprocessable";
            case 429 -> "rate_limited";
            case 503 -> "unavailable";
            case 504 -> "deadline_exceeded";
            default -> "internal";
        };
    }

    // --------------------------------------------------------------- decoding

    private <T> List<T> decodeList(String body, Function<Map<String, Object>, T> mapper) {
        List<T> out = new ArrayList<>();
        Map<String, Object> env = Json.asMap(Json.parse(body));
        if (env != null) {
            List<Object> data = Json.asList(env.get("data"));
            if (data != null) {
                for (Object o : data) {
                    Map<String, Object> m = Json.asMap(o);
                    if (m != null) {
                        out.add(mapper.apply(m));
                    }
                }
            }
        }
        return out;
    }

    private static Map<String, Object> requireObject(String body) {
        Map<String, Object> m = Json.asMap(Json.parse(body));
        if (m == null) {
            throw new ApiException("internal", "expected a JSON object response body", 0, "", "");
        }
        return m;
    }

    private static String enc(String segment) {
        return URLEncoder.encode(segment, StandardCharsets.UTF_8).replace("+", "%20");
    }
}

package digital.vasic.thready;

import com.sun.net.httpserver.HttpExchange;
import com.sun.net.httpserver.HttpServer;

import java.io.IOException;
import java.net.InetSocketAddress;
import java.net.URI;
import java.nio.charset.StandardCharsets;
import java.util.ArrayList;
import java.util.Collections;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.concurrent.atomic.AtomicInteger;

/**
 * ThreadyClientTest is a self-contained, JUnit-free test runner for {@link ThreadyClient}.
 *
 * <p>The SDK is a <em>client</em>, so the honest approach exercises it against a
 * {@link com.sun.net.httpserver.HttpServer} (JDK built-in) that mocks the gateway's
 * {@code /v1} contract on a free loopback port — recording the exact method/path/headers/body
 * the SDK sends and returning canned contract JSON. Because JUnit is unavailable, a tiny
 * in-file harness ({@link #main}) invokes each {@code test*} method, prints
 * {@code PASS}/{@code FAIL name}, a {@code N passed / M failed} summary, and exits non-zero
 * on any failure.
 */
public final class ThreadyClientTest {

    private static int passed = 0;
    private static int failed = 0;

    public static void main(String[] args) {
        run("testLoginSendsCredentialsAndStoresToken", ThreadyClientTest::testLoginSendsCredentialsAndStoresToken);
        run("testListChannelsInjectsBearerAndDecodesEnvelope", ThreadyClientTest::testListChannelsInjectsBearerAndDecodesEnvelope);
        run("testApiKeyAuthSendsXApiKeyHeader", ThreadyClientTest::testApiKeyAuthSendsXApiKeyHeader);
        run("testNoCredentialSendsNoAuthHeaders", ThreadyClientTest::testNoCredentialSendsNoAuthHeaders);
        run("testCreateChannelSendsIdempotencyKeyAndBody", ThreadyClientTest::testCreateChannelSendsIdempotencyKeyAndBody);
        run("testCreateChannelIdempotencyKeyOverride", ThreadyClientTest::testCreateChannelIdempotencyKeyOverride);
        run("testGetPostDecodesTypedPost", ThreadyClientTest::testGetPostDecodesTypedPost);
        run("testReprocessReturnsJobWithIdempotencyKey", ThreadyClientTest::testReprocessReturnsJobWithIdempotencyKey);
        run("testSearchSendsBodyAndDecodesResults", ThreadyClientTest::testSearchSendsBodyAndDecodesResults);
        run("testListSkillsDecodesEnvelope", ThreadyClientTest::testListSkillsDecodesEnvelope);
        run("testNotFoundMapsToTypedApiException", ThreadyClientTest::testNotFoundMapsToTypedApiException);
        run("testRetryGet503ThenSuccessTwoHits", ThreadyClientTest::testRetryGet503ThenSuccessTwoHits);
        run("testRetryGetExhaustedReturnsApiError", ThreadyClientTest::testRetryGetExhaustedReturnsApiError);
        run("testPostNotRetriedOn503", ThreadyClientTest::testPostNotRetriedOn503);
        run("testInsecureTransportHttpRemoteRefused", ThreadyClientTest::testInsecureTransportHttpRemoteRefused);
        run("testInsecureTransportLoopbackAllowedAttachesHeader", ThreadyClientTest::testInsecureTransportLoopbackAllowedAttachesHeader);
        run("testInsecureTransportPolicyMatrix", ThreadyClientTest::testInsecureTransportPolicyMatrix);
        run("testConstructorRequiresBaseUrl", ThreadyClientTest::testConstructorRequiresBaseUrl);
        run("testApiExceptionRetryableAndToString", ThreadyClientTest::testApiExceptionRetryableAndToString);
        run("testJsonRoundTripEscapesAndShapes", ThreadyClientTest::testJsonRoundTripEscapesAndShapes);

        System.out.println();
        System.out.println(passed + " passed / " + failed + " failed");
        System.exit(failed > 0 ? 1 : 0);
    }

    // ============================================================ test cases

    static void testLoginSendsCredentialsAndStoresToken() throws Exception {
        try (MockServer srv = startServer(req -> {
            if (req.method.equals("POST") && req.path.equals("/v1/auth/login")) {
                Map<String, Object> tp = new LinkedHashMap<>();
                tp.put("access_token", "jwt-access-abc123");
                tp.put("refresh_token", "jwt-refresh");
                tp.put("token_type", "Bearer");
                tp.put("expires_in", 900);
                tp.put("refresh_expires_in", 604800);
                return new Resp(200, Json.write(tp));
            }
            return new Resp(200, "{\"data\":[]}"); // GET /v1/channels
        })) {
            ThreadyClient c = fastClient(srv.baseUrl(), null, null, false);

            TokenPair tp = c.login("user@thready.test", "userpassword-123", null);
            assertEq("jwt-access-abc123", tp.accessToken(), "login returns access token");
            assertEq(900L, tp.expiresIn(), "login decodes expires_in");
            assertEq("jwt-access-abc123", c.getAccessToken(), "client stores token");

            Req loginReq = srv.requests.get(0);
            assertEq("application/json", loginReq.contentType, "login Content-Type");
            Map<String, Object> sent = Json.asMap(Json.parse(loginReq.body));
            assertEq("user@thready.test", sent.get("email"), "login sends email");
            assertEq("userpassword-123", sent.get("password"), "login sends password");
            assertTrue(!sent.containsKey("totp"), "null totp omitted from body");

            c.listChannels();
            assertEq("Bearer jwt-access-abc123", srv.last().authorization, "subsequent call uses stored bearer");
        }
    }

    static void testListChannelsInjectsBearerAndDecodesEnvelope() throws Exception {
        try (MockServer srv = startServer(req -> new Resp(200,
                "{\"data\":["
                        + "{\"id\":\"chan-1\",\"account_id\":\"acct-a\",\"name\":\"general\",\"platform\":\"telegram\",\"external_ref\":\"@g\"},"
                        + "{\"id\":\"chan-2\",\"account_id\":\"acct-a\",\"name\":\"ops\",\"platform\":\"max\",\"external_ref\":\"@o\"}"
                        + "],\"meta\":{\"next_cursor\":null}}"))) {
            ThreadyClient c = fastClient(srv.baseUrl(), "tok-1", null, false);
            List<Channel> chans = c.listChannels();

            Req r = srv.last();
            assertEq("GET", r.method, "method is GET");
            assertEq("/v1/channels", r.path, "path is /v1/channels");
            assertEq("Bearer tok-1", r.authorization, "Authorization bearer injected");
            assertEq(2, chans.size(), "decoded channel count");
            assertEq("chan-1", chans.get(0).id(), "first channel id");
            assertEq("max", chans.get(1).platform(), "second channel platform");
        }
    }

    static void testApiKeyAuthSendsXApiKeyHeader() throws Exception {
        try (MockServer srv = startServer(req -> new Resp(200, "{\"data\":[]}"))) {
            ThreadyClient c = fastClient(srv.baseUrl(), null, "sk-secret-123", false);
            c.listChannels();

            Req r = srv.last();
            assertEq("sk-secret-123", r.apiKey, "X-API-Key header sent");
            assertEq(null, r.authorization, "Authorization empty when using API key");
        }
    }

    static void testNoCredentialSendsNoAuthHeaders() throws Exception {
        try (MockServer srv = startServer(req -> new Resp(200, "{\"data\":[]}"))) {
            ThreadyClient c = fastClient(srv.baseUrl(), null, null, false);
            c.listChannels();

            Req r = srv.last();
            assertEq(null, r.authorization, "no Authorization without credential");
            assertEq(null, r.apiKey, "no X-API-Key without credential");
        }
    }

    static void testCreateChannelSendsIdempotencyKeyAndBody() throws Exception {
        try (MockServer srv = startServer(req -> {
            Map<String, Object> body = Json.asMap(Json.parse(req.body));
            Map<String, Object> ch = new LinkedHashMap<>();
            ch.put("id", "chan-9");
            ch.put("account_id", "acct-a");
            ch.put("name", body.get("name"));
            ch.put("platform", body.get("platform"));
            ch.put("external_ref", body.get("external_ref"));
            ch.put("created_at", "2026-07-22T09:20:00Z");
            return new Resp(201, Json.write(ch));
        })) {
            ThreadyClient c = fastClient(srv.baseUrl(), "tok-1", null, false);
            Channel ch = c.createChannel("release", "telegram", "@rel");

            Req r = srv.last();
            assertEq("POST", r.method, "method is POST");
            assertEq("/v1/channels", r.path, "path is /v1/channels");
            assertEq("application/json", r.contentType, "Content-Type set on POST with body");
            assertTrue(r.idempotencyKey != null && !r.idempotencyKey.isEmpty(),
                    "Idempotency-Key auto-generated on unsafe POST");
            Map<String, Object> sent = Json.asMap(Json.parse(r.body));
            assertEq("release", sent.get("name"), "body carries name");
            assertEq("telegram", sent.get("platform"), "body carries platform");
            assertEq("@rel", sent.get("external_ref"), "body carries external_ref");
            assertEq("chan-9", ch.id(), "decoded channel id");
            assertEq("release", ch.name(), "decoded channel name echoed");
        }
    }

    static void testCreateChannelIdempotencyKeyOverride() throws Exception {
        try (MockServer srv = startServer(req -> new Resp(201, "{\"id\":\"chan-9\"}"))) {
            ThreadyClient c = fastClient(srv.baseUrl(), "tok-1", null, false);
            c.createChannel("x", "telegram", "@x", "fixed-key-42");
            assertEq("fixed-key-42", srv.last().idempotencyKey, "explicit Idempotency-Key override honored");
        }
    }

    static void testGetPostDecodesTypedPost() throws Exception {
        try (MockServer srv = startServer(req -> new Resp(200,
                "{\"id\":\"post-1\",\"channel_id\":\"chan-1\",\"account_id\":\"acct-a\",\"body\":\"hello\","
                        + "\"hashtags\":[\"#research\"],\"categories\":[\"research\"],\"status\":\"succeeded\","
                        + "\"created_at\":\"2026-07-22T09:12:00Z\"}"))) {
            ThreadyClient c = fastClient(srv.baseUrl(), "tok-1", null, false);
            Post p = c.getPost("post-1");

            assertEq("/v1/posts/post-1", srv.last().path, "path targets the post id");
            assertEq("post-1", p.id(), "decoded post id");
            assertEq("succeeded", p.status(), "decoded processing status");
            assertEq("#research", p.hashtags().get(0), "decoded hashtag");
            assertEq("research", p.categories().get(0), "decoded category");
        }
    }

    static void testReprocessReturnsJobWithIdempotencyKey() throws Exception {
        try (MockServer srv = startServer(req -> new Resp(202,
                "{\"job_id\":\"job-1\",\"post_id\":\"post-1\",\"status\":\"queued\","
                        + "\"precedence\":[\"download\",\"convert\",\"analyze\",\"research\",\"reply\"],"
                        + "\"queued_at\":\"2026-07-22T09:20:00Z\"}"))) {
            ThreadyClient c = fastClient(srv.baseUrl(), "tok-1", null, false);
            ProcessingJob job = c.reprocess("post-1");

            Req r = srv.last();
            assertEq("POST", r.method, "method is POST");
            assertEq("/v1/posts/post-1/reprocess", r.path, "path targets reprocess");
            assertTrue(r.idempotencyKey != null && !r.idempotencyKey.isEmpty(),
                    "Idempotency-Key sent on reprocess");
            assertEq("job-1", job.jobId(), "decoded job id");
            assertEq("queued", job.status(), "decoded job status");
            assertEq(5, job.precedence().size(), "decoded 5-stage precedence");
            assertEq("download", job.precedence().get(0), "precedence head is download");
        }
    }

    static void testSearchSendsBodyAndDecodesResults() throws Exception {
        try (MockServer srv = startServer(req -> new Resp(200,
                "{\"results\":[{\"source_id\":\"post-1\",\"kind\":\"post\",\"score\":0.81,"
                        + "\"span\":\"section:1\",\"snippet\":\"benchmarks\"}],\"took_ms\":7,\"embedder\":\"llama\"}"))) {
            ThreadyClient c = fastClient(srv.baseUrl(), "tok-1", null, false);
            SearchResult res = c.search("vector database benchmarks", "hybrid", 20, List.of("posts", "generated"), true);

            Req r = srv.last();
            assertEq("POST", r.method, "method is POST");
            assertEq("/v1/search", r.path, "path is /v1/search");
            Map<String, Object> sent = Json.asMap(Json.parse(r.body));
            assertEq("vector database benchmarks", sent.get("query"), "body carries query");
            assertEq("hybrid", sent.get("mode"), "body carries mode");
            assertEq(20L, sent.get("top_k"), "body carries top_k");
            assertEq(Boolean.TRUE, sent.get("rerank"), "body carries rerank");
            assertEq(List.of("posts", "generated"), sent.get("sources"), "body carries sources array");
            assertEq("llama", res.embedder(), "decoded embedder");
            assertEq(1, res.results().size(), "decoded result count");
            assertEq("post-1", res.results().get(0).sourceId(), "decoded hit source id");
            assertEq(7, res.tookMs(), "decoded took_ms");
        }
    }

    static void testListSkillsDecodesEnvelope() throws Exception {
        try (MockServer srv = startServer(req -> new Resp(200,
                "{\"data\":[{\"id\":\"skill-download\",\"name\":\"download\",\"kind\":\"atomic\",\"sort_order\":1},"
                        + "{\"id\":\"skill-reply\",\"name\":\"reply\",\"kind\":\"atomic\",\"sort_order\":5}]}"))) {
            ThreadyClient c = fastClient(srv.baseUrl(), "tok-1", null, false);
            List<Skill> skills = c.listSkills();

            assertEq("/v1/skills", srv.last().path, "path is /v1/skills");
            assertEq(2, skills.size(), "decoded skill count");
            assertEq("download", skills.get(0).name(), "first skill name");
            assertEq(5, skills.get(1).sortOrder(), "second skill sort_order");
        }
    }

    static void testNotFoundMapsToTypedApiException() throws Exception {
        try (MockServer srv = startServer(req ->
                errorEnvelope(404, "not_found", "post not found", "req-abc-123"))) {
            ThreadyClient c = fastClient(srv.baseUrl(), "tok-1", null, false);

            ApiException ex = null;
            try {
                c.getPost("missing");
            } catch (ApiException e) {
                ex = e;
            }
            assertTrue(ex != null, "404 surfaces an ApiException");
            assertEq("not_found", ex.code(), "ApiException.code");
            assertEq(404, ex.status(), "ApiException.status");
            assertEq("req-abc-123", ex.requestId(), "ApiException.requestId");
            assertEq("post not found", ex.getMessage(), "ApiException.message");
        }
    }

    static void testRetryGet503ThenSuccessTwoHits() throws Exception {
        AtomicInteger calls = new AtomicInteger();
        try (MockServer srv = startServer(req -> {
            if (calls.incrementAndGet() == 1) {
                return errorEnvelope(503, "unavailable", "embedder warming up", "req-1");
            }
            return new Resp(200,
                    "{\"data\":[{\"id\":\"skill-download\",\"name\":\"download\",\"kind\":\"atomic\",\"sort_order\":1}]}");
        })) {
            ThreadyClient c = fastClient(srv.baseUrl(), "tok-1", null, false);
            List<Skill> skills = c.listSkills();

            assertEq(2, calls.get(), "GET retried once: server saw exactly 2 hits");
            assertEq(1, skills.size(), "decoded body of the successful retry");
        }
    }

    static void testRetryGetExhaustedReturnsApiError() throws Exception {
        AtomicInteger calls = new AtomicInteger();
        try (MockServer srv = startServer(req -> {
            calls.incrementAndGet();
            return errorEnvelope(503, "unavailable", "still down", "req-x");
        })) {
            ThreadyClient c = fastClient(srv.baseUrl(), "tok-1", null, false);

            ApiException ex = null;
            try {
                c.listSkills();
            } catch (ApiException e) {
                ex = e;
            }
            assertTrue(ex != null && ex.code().equals("unavailable"),
                    "exhausted retries surface the last ApiException");
            assertEq(4, calls.get(), "1 initial + 3 retries = 4 attempts");
        }
    }

    static void testPostNotRetriedOn503() throws Exception {
        AtomicInteger calls = new AtomicInteger();
        try (MockServer srv = startServer(req -> {
            calls.incrementAndGet();
            return errorEnvelope(503, "unavailable", "down", "req-p");
        })) {
            ThreadyClient c = fastClient(srv.baseUrl(), "tok-1", null, false);

            ApiException ex = null;
            try {
                c.reprocess("post-1");
            } catch (ApiException e) {
                ex = e;
            }
            assertTrue(ex != null, "unsafe POST surfaces the ApiException");
            assertEq(1, calls.get(), "unsafe POST is NOT retried on 503");
        }
    }

    static void testInsecureTransportHttpRemoteRefused() {
        // A credential over plaintext http to a non-loopback host must be refused BEFORE send.
        ThreadyClient bearer = new ThreadyClient("http://remote.invalid", "tok-secret-xyz", null, false);
        assertThrows(InsecureTransportException.class, bearer::listChannels,
                "bearer over remote http is refused");

        ThreadyClient apikey = new ThreadyClient("http://remote.invalid", null, "sk-secret-xyz", false);
        assertThrows(InsecureTransportException.class, apikey::listChannels,
                "api key over remote http is refused");
    }

    static void testInsecureTransportLoopbackAllowedAttachesHeader() throws Exception {
        // The mock server runs on 127.0.0.1 over http — a genuine loopback-http transport.
        try (MockServer srv = startServer(req -> new Resp(200, "{\"data\":[]}"))) {
            ThreadyClient c = fastClient(srv.baseUrl(), "tok-loopback", null, false);
            c.listChannels(); // must NOT throw
            assertEq("Bearer tok-loopback", srv.last().authorization,
                    "http+127.0.0.1 is allowed and the bearer header is attached");
        }
    }

    static void testInsecureTransportPolicyMatrix() {
        ThreadyClient https = new ThreadyClient("https://api.thready.test", "tok", null, false);
        assertTrue(https.isCredentialTransportAllowed(URI.create("https://api.thready.test/v1/channels")),
                "https (any host) is allowed");

        ThreadyClient localhost = new ThreadyClient("http://localhost:8080", "tok", null, false);
        assertTrue(localhost.isCredentialTransportAllowed(URI.create("http://localhost:8080/v1/x")),
                "http+localhost is allowed");

        ThreadyClient loopback = new ThreadyClient("http://127.0.0.1:8080", "tok", null, false);
        assertTrue(loopback.isCredentialTransportAllowed(URI.create("http://127.0.0.1:8080/v1/x")),
                "http+127.0.0.1 is allowed");

        ThreadyClient remote = new ThreadyClient("http://remote.invalid", "tok", null, false);
        assertTrue(!remote.isCredentialTransportAllowed(URI.create("http://remote.invalid/v1/x")),
                "http+remote is refused by policy");

        ThreadyClient override = new ThreadyClient("http://remote.invalid", "tok", null, true);
        assertTrue(override.isCredentialTransportAllowed(URI.create("http://remote.invalid/v1/x")),
                "allowInsecureHttp overrides the refusal");
    }

    static void testConstructorRequiresBaseUrl() {
        assertThrows(IllegalArgumentException.class,
                () -> new ThreadyClient("", null, null, false), "empty baseUrl rejected");
        assertThrows(IllegalArgumentException.class,
                () -> new ThreadyClient(null, null, null, false), "null baseUrl rejected");
        // A trailing "/v1/" is normalized away, not rejected.
        ThreadyClient c = new ThreadyClient("https://x/v1/", "tok", null, false);
        assertTrue(c != null, "baseUrl with trailing /v1/ is accepted");
    }

    static void testApiExceptionRetryableAndToString() {
        ApiException rl = new ApiException("rate_limited", "slow down", 429, "req-9", "");
        assertTrue(rl.retryable(), "rate_limited is retryable");
        assertTrue(rl.toString().contains("rate_limited") && rl.toString().contains("req-9"),
                "toString carries code and request id");
        ApiException nf = new ApiException("not_found", "x", 404, "", "");
        assertTrue(!nf.retryable(), "not_found is not retryable");
    }

    static void testJsonRoundTripEscapesAndShapes() {
        Map<String, Object> obj = new LinkedHashMap<>();
        obj.put("s", "quote\"tab\tnl\nunicodeé");
        obj.put("n", 42);
        obj.put("f", 0.5);
        obj.put("b", true);
        obj.put("z", null);
        obj.put("arr", List.of("a", "b"));
        Map<String, Object> nested = new LinkedHashMap<>();
        nested.put("k", "v");
        obj.put("obj", nested);

        String encoded = Json.write(obj);
        Map<String, Object> back = Json.asMap(Json.parse(encoded));
        assertEq("quote\"tab\tnl\nunicodeé", back.get("s"), "string escapes round-trip");
        assertEq(42L, back.get("n"), "integer decodes to Long");
        assertEq(0.5, back.get("f"), "fraction decodes to Double");
        assertEq(Boolean.TRUE, back.get("b"), "boolean round-trips");
        assertTrue(back.containsKey("z") && back.get("z") == null, "null round-trips");
        assertEq("b", Json.asList(back.get("arr")).get(1), "nested array round-trips");
        assertEq("v", Json.asMap(back.get("obj")).get("k"), "nested object round-trips");
    }

    // ========================================================= harness + mock

    @FunctionalInterface
    interface ThrowingRunnable {
        void run() throws Exception;
    }

    private static void run(String name, ThrowingRunnable body) {
        try {
            body.run();
            System.out.println("PASS " + name);
            passed++;
        } catch (Throwable t) {
            System.out.println("FAIL " + name + ": " + t);
            failed++;
        }
    }

    private static void assertEq(Object want, Object got, String msg) {
        if (!Objects.equals(want, got)) {
            throw new AssertionError(msg + ": want <" + want + "> got <" + got + ">");
        }
    }

    private static void assertTrue(boolean cond, String msg) {
        if (!cond) {
            throw new AssertionError(msg);
        }
    }

    private static void assertThrows(Class<? extends Throwable> type, ThrowingRunnable body, String msg) {
        try {
            body.run();
        } catch (Throwable t) {
            if (type.isInstance(t)) {
                return;
            }
            throw new AssertionError(msg + ": expected " + type.getSimpleName()
                    + " but got " + t.getClass().getSimpleName() + " (" + t + ")");
        }
        throw new AssertionError(msg + ": expected " + type.getSimpleName() + " but nothing was thrown");
    }

    private static ThreadyClient fastClient(String baseUrl, String token, String apiKey, boolean allowInsecure) {
        ThreadyClient c = new ThreadyClient(baseUrl, token, apiKey, allowInsecure);
        c.backoffBaseMillis = 1; // keep retry sleeps tiny under test
        c.backoffMaxMillis = 5;
        return c;
    }

    /** A recorded inbound request. */
    static final class Req {
        String method;
        String path;
        String rawQuery;
        String authorization;
        String apiKey;
        String idempotencyKey;
        String contentType;
        String accept;
        String body;
    }

    /** A canned response (status + JSON body; empty body ⇒ no content). */
    record Resp(int status, String body) {
    }

    @FunctionalInterface
    interface Responder {
        Resp respond(Req req) throws IOException;
    }

    /** A running mock gateway on a free loopback port. */
    static final class MockServer implements AutoCloseable {
        final HttpServer server;
        final List<Req> requests;
        final int port;

        MockServer(HttpServer server, List<Req> requests) {
            this.server = server;
            this.requests = requests;
            this.port = server.getAddress().getPort();
        }

        String baseUrl() {
            return "http://127.0.0.1:" + port;
        }

        Req last() {
            synchronized (requests) {
                return requests.get(requests.size() - 1);
            }
        }

        @Override
        public void close() {
            server.stop(0);
        }
    }

    private static MockServer startServer(Responder responder) throws IOException {
        HttpServer server = HttpServer.create(new InetSocketAddress("127.0.0.1", 0), 0);
        List<Req> requests = Collections.synchronizedList(new ArrayList<>());
        server.createContext("/", (HttpExchange ex) -> {
            Req q = new Req();
            q.method = ex.getRequestMethod();
            q.path = ex.getRequestURI().getPath();
            q.rawQuery = ex.getRequestURI().getRawQuery();
            q.authorization = ex.getRequestHeaders().getFirst("Authorization");
            q.apiKey = ex.getRequestHeaders().getFirst("X-API-Key");
            q.idempotencyKey = ex.getRequestHeaders().getFirst("Idempotency-Key");
            q.contentType = ex.getRequestHeaders().getFirst("Content-Type");
            q.accept = ex.getRequestHeaders().getFirst("Accept");
            q.body = new String(ex.getRequestBody().readAllBytes(), StandardCharsets.UTF_8);
            requests.add(q);

            Resp resp;
            try {
                resp = responder.respond(q);
            } catch (Throwable t) {
                resp = new Resp(500, "{\"error\":{\"code\":\"internal\",\"message\":\"mock handler failed\"}}");
            }
            byte[] out = resp.body() == null ? new byte[0] : resp.body().getBytes(StandardCharsets.UTF_8);
            ex.getResponseHeaders().set("Content-Type", "application/json");
            if (out.length == 0) {
                ex.sendResponseHeaders(resp.status(), -1);
            } else {
                ex.sendResponseHeaders(resp.status(), out.length);
                ex.getResponseBody().write(out);
            }
            ex.close();
        });
        server.setExecutor(null);
        server.start();
        return new MockServer(server, requests);
    }

    /** Build the gateway's canonical failure envelope as a canned response. */
    private static Resp errorEnvelope(int status, String code, String message, String requestId) {
        Map<String, Object> err = new LinkedHashMap<>();
        err.put("code", code);
        err.put("message", message);
        err.put("status", status);
        err.put("request_id", requestId);
        err.put("trace_id", requestId);
        Map<String, Object> env = new LinkedHashMap<>();
        env.put("error", err);
        return new Resp(status, Json.write(env));
    }

    private ThreadyClientTest() {
    }
}

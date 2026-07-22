# EVIDENCE — Helix Thready Java SDK (`sdk_java`)

Physical proof that the SDK compiles clean and its test runner exits `0` with
**N/N assertions PASS**. All output below is the REAL captured run — no bluff.

- **Toolchain:** JDK 21 standard library only (`java.net.http.HttpClient`,
  `com.sun.net.httpserver.HttpServer`). No Maven/Gradle, no Jackson/Gson/JUnit.
- **Result:** `20 passed / 0 failed`, process exit code **`0`**.
- **Compile:** clean under `javac -Xlint:all` (0 warnings, 0 errors).

## Environment

```
$ javac -version
javac 21.0.11

$ java -version
openjdk version "21.0.11" 2026-04-21
OpenJDK Runtime Environment (Red_Hat-21.0.11.0.10-alt1) (build 21.0.11+10)
OpenJDK 64-Bit Server VM (Red_Hat-21.0.11.0.10-alt1) (build 21.0.11+10, mixed mode)
```

## Clean compile (`-Xlint:all`)

```
$ rm -rf out && mkdir -p out
$ javac -Xlint:all -d out $(find src -name '*.java')
compile: clean (0 warnings, 0 errors)
```

## Real test run

```
$ bash run.sh
PASS testLoginSendsCredentialsAndStoresToken
PASS testListChannelsInjectsBearerAndDecodesEnvelope
PASS testApiKeyAuthSendsXApiKeyHeader
PASS testNoCredentialSendsNoAuthHeaders
PASS testCreateChannelSendsIdempotencyKeyAndBody
PASS testCreateChannelIdempotencyKeyOverride
PASS testGetPostDecodesTypedPost
PASS testReprocessReturnsJobWithIdempotencyKey
PASS testSearchSendsBodyAndDecodesResults
PASS testListSkillsDecodesEnvelope
PASS testNotFoundMapsToTypedApiException
PASS testRetryGet503ThenSuccessTwoHits
PASS testRetryGetExhaustedReturnsApiError
PASS testPostNotRetriedOn503
PASS testInsecureTransportHttpRemoteRefused
PASS testInsecureTransportLoopbackAllowedAttachesHeader
PASS testInsecureTransportPolicyMatrix
PASS testConstructorRequiresBaseUrl
PASS testApiExceptionRetryableAndToString
PASS testJsonRoundTripEscapesAndShapes

20 passed / 0 failed
$ echo exit=$?
exit=0
```

## What each test proves (mapped to the requirement)

| Test | Requirement covered |
|------|---------------------|
| `testLoginSendsCredentialsAndStoresToken` | `POST /v1/auth/login` sends `{email,password}` (null `totp` omitted), decodes `TokenPair`, stores the access token so the next call sends `Authorization: Bearer …` |
| `testListChannelsInjectsBearerAndDecodesEnvelope` | `GET /v1/channels`, bearer injected, `{data:[…]}` envelope decoded |
| `testApiKeyAuthSendsXApiKeyHeader` | `X-API-Key` sent when only an API key is set; no `Authorization` |
| `testNoCredentialSendsNoAuthHeaders` | Neither auth header sent when no credential is configured |
| `testCreateChannelSendsIdempotencyKeyAndBody` | `POST /v1/channels` auto-stamps an `Idempotency-Key`, sends the JSON body, decodes `Channel` |
| `testCreateChannelIdempotencyKeyOverride` | Explicit `Idempotency-Key` override honored |
| `testGetPostDecodesTypedPost` | `GET /v1/posts/{id}` path + typed `Post` decode (hashtags/categories) |
| `testReprocessReturnsJobWithIdempotencyKey` | `POST /v1/posts/{id}/reprocess` (202) stamps `Idempotency-Key`, decodes `ProcessingJob` + precedence |
| `testSearchSendsBodyAndDecodesResults` | `POST /v1/search` body (`query,mode,top_k,sources,rerank`), decodes `SearchResult`/`SearchHit` |
| `testListSkillsDecodesEnvelope` | `GET /v1/skills` envelope decode |
| `testNotFoundMapsToTypedApiException` | 404 `{"error":{code,message,request_id}}` → typed `ApiException(code,status,requestId,message)` |
| `testRetryGet503ThenSuccessTwoHits` | Idempotent GET retried on 503 → **exactly 2 server hits** |
| `testRetryGetExhaustedReturnsApiError` | GET retries capped: 1 initial + 3 retries = 4 hits, then typed error |
| `testPostNotRetriedOn503` | Unsafe POST is **not** retried on 503 (1 hit) |
| `testInsecureTransportHttpRemoteRefused` | http + remote host + credential → `InsecureTransportException` **before send** (bearer AND api key) |
| `testInsecureTransportLoopbackAllowedAttachesHeader` | http + `127.0.0.1` allowed; bearer actually attached (verified server-side) |
| `testInsecureTransportPolicyMatrix` | https allowed, http+localhost allowed, http+127.0.0.1 allowed, http+remote refused, `allowInsecureHttp` override allowed |
| `testConstructorRequiresBaseUrl` | Empty/null baseUrl rejected; trailing `/v1/` normalized |
| `testApiExceptionRetryableAndToString` | `retryable()` semantics + `toString()` carries code/request id |
| `testJsonRoundTripEscapesAndShapes` | Hand-rolled `Json` codec: string escapes, `Long` vs `Double`, nested object/array round-trip |

## Reproduce

```
cd implementation/sdk_java
bash run.sh
echo exit=$?    # 0
```

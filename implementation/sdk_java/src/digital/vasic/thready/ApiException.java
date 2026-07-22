package digital.vasic.thready;

/**
 * ApiException is the typed error raised for every non-2xx response from the
 * Helix Thready {@code /v1} gateway. It is decoded from the canonical failure
 * envelope:
 *
 * <pre>{"error":{"code","message","status","request_id","trace_id"}}</pre>
 *
 * <p>The stable string {@link #code()} mirrors the Connect/gRPC canonical code set
 * (see docs/.../api/error-model.md) so a single client-side handler branches on
 * {@code code}/{@code status} identically across the REST and event planes.
 *
 * <p>It is unchecked so method signatures stay clean; callers catch it explicitly.
 */
public class ApiException extends RuntimeException {

    private static final long serialVersionUID = 1L;

    private final String code;
    private final int status;
    private final String requestId;
    private final String traceId;

    public ApiException(String code, String message, int status, String requestId, String traceId) {
        super(message);
        this.code = code;
        this.status = status;
        this.requestId = requestId;
        this.traceId = traceId;
    }

    /** Stable machine-readable error code (e.g. {@code not_found}, {@code unavailable}). */
    public String code() {
        return code;
    }

    /** Mirrored HTTP status code. */
    public int status() {
        return status;
    }

    /** Server request id for support/log correlation ({@code request_id}); may be empty. */
    public String requestId() {
        return requestId;
    }

    /** OpenTelemetry trace id ({@code trace_id}); may be empty. */
    public String traceId() {
        return traceId;
    }

    /** Whether the code is one the SDK may transparently retry. */
    public boolean retryable() {
        return "rate_limited".equals(code)
                || "unavailable".equals(code)
                || "deadline_exceeded".equals(code);
    }

    @Override
    public String toString() {
        StringBuilder sb = new StringBuilder("thready: ")
                .append(code).append(" (").append(status).append("): ").append(getMessage());
        if (requestId != null && !requestId.isEmpty()) {
            sb.append(" [request_id=").append(requestId).append(']');
        }
        return sb.toString();
    }
}

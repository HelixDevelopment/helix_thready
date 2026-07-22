package digital.vasic.thready;

/**
 * InsecureTransportException is thrown — <em>before any bytes leave the process</em> —
 * instead of attaching a credential ({@code Authorization: Bearer …} or {@code X-API-Key})
 * to a request that would travel over plaintext {@code http} to a NON-loopback host.
 *
 * <p>Sending a bearer token or API key in the clear to a remote origin would expose it to
 * any on-path observer, so the SDK refuses by default. {@code https} (any host) and
 * {@code http} to a loopback host ({@code 127.0.0.1}, {@code ::1}, {@code localhost}) are
 * always allowed; construct the client with {@code allowInsecureHttp = true} to opt out on
 * a trusted network.
 */
public class InsecureTransportException extends RuntimeException {

    private static final long serialVersionUID = 1L;

    public InsecureTransportException(String message) {
        super(message);
    }
}

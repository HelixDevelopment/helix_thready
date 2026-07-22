package digital.vasic.thready;

import java.util.Map;

/**
 * TokenPair is the {@code POST /v1/auth/login} (and refresh) success body: a
 * short-lived access token plus a rotating refresh token.
 */
public record TokenPair(
        String accessToken,
        String refreshToken,
        String tokenType,
        long expiresIn,
        long refreshExpiresIn) {

    static TokenPair fromMap(Map<String, Object> m) {
        return new TokenPair(
                Json.str(m.get("access_token")),
                Json.str(m.get("refresh_token")),
                Json.str(m.get("token_type")),
                Json.longVal(m.get("expires_in"), 0),
                Json.longVal(m.get("refresh_expires_in"), 0));
    }
}

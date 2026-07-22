package digital.vasic.thready;

import java.util.Map;

/** Channel is a registered messenger channel/group ({@code /v1/channels}). */
public record Channel(
        String id,
        String accountId,
        String name,
        String platform,
        String externalRef,
        String createdAt) {

    static Channel fromMap(Map<String, Object> m) {
        return new Channel(
                Json.str(m.get("id")),
                Json.str(m.get("account_id")),
                Json.str(m.get("name")),
                Json.str(m.get("platform")),
                Json.str(m.get("external_ref")),
                Json.str(m.get("created_at")));
    }
}

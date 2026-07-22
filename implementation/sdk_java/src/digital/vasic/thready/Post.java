package digital.vasic.thready;

import java.util.List;
import java.util.Map;

/** Post is a channel post with its processing status ({@code /v1/posts/{id}}). */
public record Post(
        String id,
        String channelId,
        String accountId,
        String body,
        List<String> hashtags,
        List<String> categories,
        String status,
        String createdAt) {

    static Post fromMap(Map<String, Object> m) {
        return new Post(
                Json.str(m.get("id")),
                Json.str(m.get("channel_id")),
                Json.str(m.get("account_id")),
                Json.str(m.get("body")),
                Json.strList(m.get("hashtags")),
                Json.strList(m.get("categories")),
                Json.str(m.get("status")),
                Json.str(m.get("created_at")));
    }
}

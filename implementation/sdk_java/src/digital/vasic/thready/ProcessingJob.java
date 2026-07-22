package digital.vasic.thready;

import java.util.List;
import java.util.Map;

/**
 * ProcessingJob is the async (re)processing job returned (202 Accepted) by
 * {@code reprocess}. The deterministic {@link #precedence()} encodes the Skill
 * dispatch order (download &gt; convert &gt; analyze &gt; research &gt; reply).
 */
public record ProcessingJob(
        String jobId,
        String postId,
        String status,
        List<String> precedence,
        String queuedAt) {

    static ProcessingJob fromMap(Map<String, Object> m) {
        return new ProcessingJob(
                Json.str(m.get("job_id")),
                Json.str(m.get("post_id")),
                Json.str(m.get("status")),
                Json.strList(m.get("precedence")),
                Json.str(m.get("queued_at")));
    }
}

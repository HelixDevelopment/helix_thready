package digital.vasic.thready;

import java.util.Map;

/** SearchHit is a single ranked result in a {@link SearchResult}. */
public record SearchHit(
        String sourceId,
        String kind,
        double score,
        String span,
        String snippet) {

    static SearchHit fromMap(Map<String, Object> m) {
        return new SearchHit(
                Json.str(m.get("source_id")),
                Json.str(m.get("kind")),
                Json.dblVal(m.get("score"), 0.0),
                Json.str(m.get("span")),
                Json.str(m.get("snippet")));
    }
}

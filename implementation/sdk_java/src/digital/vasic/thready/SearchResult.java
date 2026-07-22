package digital.vasic.thready;

import java.util.ArrayList;
import java.util.List;
import java.util.Map;

/**
 * SearchResult is the ranked result set returned by {@code POST /v1/search} plus
 * provenance. {@link #embedder()} echoes the active embedding provider (the gateway
 * fails loud with 503 unavailable when a non-semantic hash stub is active).
 */
public record SearchResult(
        List<SearchHit> results,
        int tookMs,
        String embedder) {

    static SearchResult fromMap(Map<String, Object> m) {
        List<SearchHit> hits = new ArrayList<>();
        List<Object> rs = Json.asList(m.get("results"));
        if (rs != null) {
            for (Object o : rs) {
                Map<String, Object> hm = Json.asMap(o);
                if (hm != null) {
                    hits.add(SearchHit.fromMap(hm));
                }
            }
        }
        return new SearchResult(hits, Json.intVal(m.get("took_ms"), 0), Json.str(m.get("embedder")));
    }
}

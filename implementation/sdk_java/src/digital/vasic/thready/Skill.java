package digital.vasic.thready;

import java.util.Map;

/**
 * Skill is a knowledge unit in the Skill-Graph DAG ({@code /v1/skills}).
 * {@link #sortOrder()} is the dispatch precedence within a stage.
 */
public record Skill(
        String id,
        String name,
        String kind,
        int sortOrder) {

    static Skill fromMap(Map<String, Object> m) {
        return new Skill(
                Json.str(m.get("id")),
                Json.str(m.get("name")),
                Json.str(m.get("kind")),
                Json.intVal(m.get("sort_order"), 0));
    }
}

package digital.vasic.thready;

import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

/**
 * Json is a small, dependency-free JSON encoder + recursive-descent parser.
 *
 * <p>The JDK ships no built-in JSON, and this SDK is stdlib-only, so this hand-rolled
 * codec covers exactly the contract shapes the Helix Thready {@code /v1} surface emits:
 * objects, arrays, strings (with the full JSON escape set including {@code \\uXXXX}),
 * numbers (integral values decode to {@link Long}, fractional/exponent values to
 * {@link Double}), booleans and null.
 *
 * <p>{@link #parse(String)} returns a tree of {@link Map} (object, insertion-ordered),
 * {@link List} (array), {@link String}, {@link Long}/{@link Double}, {@link Boolean} or
 * {@code null}. {@link #write(Object)} serializes the same value space back to a string.
 */
public final class Json {

    private Json() {
    }

    // ----------------------------------------------------------------- encode

    /** Serialize a value tree (Map/List/String/Number/Boolean/null) to JSON text. */
    public static String write(Object value) {
        StringBuilder sb = new StringBuilder();
        writeValue(sb, value);
        return sb.toString();
    }

    private static void writeValue(StringBuilder sb, Object v) {
        if (v == null) {
            sb.append("null");
        } else if (v instanceof String s) {
            writeString(sb, s);
        } else if (v instanceof Boolean b) {
            sb.append(b ? "true" : "false");
        } else if (v instanceof Number n) {
            sb.append(numberToString(n));
        } else if (v instanceof Map<?, ?> m) {
            writeObject(sb, m);
        } else if (v instanceof Iterable<?> it) {
            writeArray(sb, it);
        } else if (v instanceof Object[] arr) {
            sb.append('[');
            for (int i = 0; i < arr.length; i++) {
                if (i > 0) {
                    sb.append(',');
                }
                writeValue(sb, arr[i]);
            }
            sb.append(']');
        } else {
            // Unknown type: encode its string form so we never emit invalid JSON.
            writeString(sb, v.toString());
        }
    }

    private static void writeObject(StringBuilder sb, Map<?, ?> m) {
        sb.append('{');
        boolean first = true;
        for (Map.Entry<?, ?> e : m.entrySet()) {
            if (!first) {
                sb.append(',');
            }
            first = false;
            writeString(sb, String.valueOf(e.getKey()));
            sb.append(':');
            writeValue(sb, e.getValue());
        }
        sb.append('}');
    }

    private static void writeArray(StringBuilder sb, Iterable<?> it) {
        sb.append('[');
        boolean first = true;
        for (Object o : it) {
            if (!first) {
                sb.append(',');
            }
            first = false;
            writeValue(sb, o);
        }
        sb.append(']');
    }

    private static void writeString(StringBuilder sb, String s) {
        sb.append('"');
        for (int i = 0; i < s.length(); i++) {
            char c = s.charAt(i);
            switch (c) {
                case '"' -> sb.append("\\\"");
                case '\\' -> sb.append("\\\\");
                case '\n' -> sb.append("\\n");
                case '\r' -> sb.append("\\r");
                case '\t' -> sb.append("\\t");
                case '\b' -> sb.append("\\b");
                case '\f' -> sb.append("\\f");
                default -> {
                    if (c < 0x20) {
                        sb.append(String.format("\\u%04x", (int) c));
                    } else {
                        sb.append(c);
                    }
                }
            }
        }
        sb.append('"');
    }

    private static String numberToString(Number n) {
        if (n instanceof Double || n instanceof Float) {
            return String.valueOf(n.doubleValue());
        }
        return n.toString();
    }

    // ----------------------------------------------------------------- decode

    /** Parse JSON text into a value tree. Throws {@link IllegalArgumentException} on malformed input. */
    public static Object parse(String text) {
        if (text == null) {
            throw new IllegalArgumentException("json: null input");
        }
        Parser p = new Parser(text);
        p.skipWs();
        Object v = p.parseValue();
        p.skipWs();
        if (!p.atEnd()) {
            throw p.err("trailing content");
        }
        return v;
    }

    private static final class Parser {
        private final String s;
        private int pos;

        Parser(String s) {
            this.s = s;
        }

        boolean atEnd() {
            return pos >= s.length();
        }

        void skipWs() {
            while (pos < s.length()) {
                char c = s.charAt(pos);
                if (c == ' ' || c == '\t' || c == '\n' || c == '\r') {
                    pos++;
                } else {
                    break;
                }
            }
        }

        char peek() {
            if (pos >= s.length()) {
                throw err("unexpected end of input");
            }
            return s.charAt(pos);
        }

        char nextChar() {
            if (pos >= s.length()) {
                throw err("unexpected end of input");
            }
            return s.charAt(pos++);
        }

        Object parseValue() {
            skipWs();
            char c = peek();
            return switch (c) {
                case '{' -> parseObject();
                case '[' -> parseArray();
                case '"' -> parseString();
                case 't', 'f' -> parseBool();
                case 'n' -> parseNull();
                default -> parseNumber();
            };
        }

        Map<String, Object> parseObject() {
            expect('{');
            Map<String, Object> m = new LinkedHashMap<>();
            skipWs();
            if (peek() == '}') {
                nextChar();
                return m;
            }
            while (true) {
                skipWs();
                if (peek() != '"') {
                    throw err("expected string key");
                }
                String key = parseString();
                skipWs();
                expect(':');
                m.put(key, parseValue());
                skipWs();
                char c = nextChar();
                if (c == '}') {
                    break;
                }
                if (c != ',') {
                    throw err("expected ',' or '}'");
                }
            }
            return m;
        }

        List<Object> parseArray() {
            expect('[');
            List<Object> a = new ArrayList<>();
            skipWs();
            if (peek() == ']') {
                nextChar();
                return a;
            }
            while (true) {
                a.add(parseValue());
                skipWs();
                char c = nextChar();
                if (c == ']') {
                    break;
                }
                if (c != ',') {
                    throw err("expected ',' or ']'");
                }
            }
            return a;
        }

        String parseString() {
            expect('"');
            StringBuilder sb = new StringBuilder();
            while (true) {
                char c = nextChar();
                if (c == '"') {
                    break;
                }
                if (c == '\\') {
                    char e = nextChar();
                    switch (e) {
                        case '"' -> sb.append('"');
                        case '\\' -> sb.append('\\');
                        case '/' -> sb.append('/');
                        case 'n' -> sb.append('\n');
                        case 'r' -> sb.append('\r');
                        case 't' -> sb.append('\t');
                        case 'b' -> sb.append('\b');
                        case 'f' -> sb.append('\f');
                        case 'u' -> {
                            if (pos + 4 > s.length()) {
                                throw err("truncated \\u escape");
                            }
                            String hex = s.substring(pos, pos + 4);
                            pos += 4;
                            sb.append((char) Integer.parseInt(hex, 16));
                        }
                        default -> throw err("invalid escape '\\" + e + "'");
                    }
                } else {
                    sb.append(c);
                }
            }
            return sb.toString();
        }

        Object parseNumber() {
            int start = pos;
            if (!atEnd() && s.charAt(pos) == '-') {
                pos++;
            }
            while (!atEnd()) {
                char c = s.charAt(pos);
                if ((c >= '0' && c <= '9') || c == '.' || c == 'e' || c == 'E' || c == '+' || c == '-') {
                    pos++;
                } else {
                    break;
                }
            }
            String num = s.substring(start, pos);
            if (num.isEmpty() || num.equals("-")) {
                throw err("invalid number");
            }
            if (num.indexOf('.') >= 0 || num.indexOf('e') >= 0 || num.indexOf('E') >= 0) {
                return Double.parseDouble(num);
            }
            try {
                return Long.parseLong(num);
            } catch (NumberFormatException e) {
                return Double.parseDouble(num);
            }
        }

        Boolean parseBool() {
            if (s.startsWith("true", pos)) {
                pos += 4;
                return Boolean.TRUE;
            }
            if (s.startsWith("false", pos)) {
                pos += 5;
                return Boolean.FALSE;
            }
            throw err("invalid literal");
        }

        Object parseNull() {
            if (s.startsWith("null", pos)) {
                pos += 4;
                return null;
            }
            throw err("invalid literal");
        }

        void expect(char c) {
            char g = nextChar();
            if (g != c) {
                throw err("expected '" + c + "' but found '" + g + "'");
            }
        }

        IllegalArgumentException err(String m) {
            return new IllegalArgumentException("json: " + m + " at offset " + pos);
        }
    }

    // ------------------------------------------------------- coercion helpers

    /** Coerce a parsed value to a String (null-safe). */
    public static String str(Object o) {
        return o == null ? null : o.toString();
    }

    /** Coerce a parsed value to a long, falling back to {@code def}. */
    public static long longVal(Object o, long def) {
        if (o instanceof Number n) {
            return n.longValue();
        }
        if (o instanceof String s) {
            try {
                return Long.parseLong(s.trim());
            } catch (NumberFormatException e) {
                return def;
            }
        }
        return def;
    }

    /** Coerce a parsed value to an int, falling back to {@code def}. */
    public static int intVal(Object o, int def) {
        return (int) longVal(o, def);
    }

    /** Coerce a parsed value to a double, falling back to {@code def}. */
    public static double dblVal(Object o, double def) {
        if (o instanceof Number n) {
            return n.doubleValue();
        }
        if (o instanceof String s) {
            try {
                return Double.parseDouble(s.trim());
            } catch (NumberFormatException e) {
                return def;
            }
        }
        return def;
    }

    /** Coerce a parsed array to a List of Strings (empty list when absent/not-an-array). */
    public static List<String> strList(Object o) {
        List<String> r = new ArrayList<>();
        if (o instanceof List<?> l) {
            for (Object x : l) {
                r.add(x == null ? null : x.toString());
            }
        }
        return r;
    }

    /** View a parsed value as a JSON object, or null when it is not one. */
    @SuppressWarnings("unchecked")
    public static Map<String, Object> asMap(Object o) {
        return o instanceof Map ? (Map<String, Object>) o : null;
    }

    /** View a parsed value as a JSON array, or null when it is not one. */
    @SuppressWarnings("unchecked")
    public static List<Object> asList(Object o) {
        return o instanceof List ? (List<Object>) o : null;
    }
}

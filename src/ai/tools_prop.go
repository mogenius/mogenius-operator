package ai

// prop builds a JSON-schema property map for a scalar tool parameter.
// Enum values are optional; when provided they are stored as []any as required
// by the Ollama provider's type assertions.
func prop(typ, desc string, enum ...string) map[string]any {
	m := map[string]any{"type": typ, "description": desc}
	if len(enum) > 0 {
		e := make([]any, len(enum))
		for i, v := range enum {
			e[i] = v
		}
		m["enum"] = e
	}
	return m
}

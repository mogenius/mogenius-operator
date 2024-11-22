package test

import (
	"strings"

	"github.com/lithammer/dedent"
)

// Normalize indented yaml:
//   - remove preceeding whitespace
//   - trim spaces and add a final newline afterwards
func YamlSanitize(yaml string) string {
	yaml = dedent.Dedent(yaml)
	yaml = strings.TrimSpace(yaml)
	yaml = yaml + "\n"
	return yaml
}

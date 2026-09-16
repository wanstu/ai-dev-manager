package app

import (
	"os"
	"testing"
)

func TestExpandEnvironmentTemplateSupportsDefaultExpressions(t *testing.T) {
	const name = "ADM_MCP_TEST_FALLBACK_VALUE_7F3A"
	_ = os.Unsetenv(name)

	value, unresolved := expandEnvironmentTemplate("prefix-${" + name + ":-fallback}-suffix")
	if unresolved || value != "prefix-fallback-suffix" {
		t.Fatalf("unset default expansion value=%q unresolved=%v", value, unresolved)
	}
	t.Setenv(name, "configured")
	value, unresolved = expandEnvironmentTemplate("${" + name + ":-fallback}")
	if unresolved || value != "configured" {
		t.Fatalf("configured default expansion value=%q unresolved=%v", value, unresolved)
	}
	t.Setenv(name, "")
	value, unresolved = expandEnvironmentTemplate("${" + name + ":-fallback}")
	if unresolved || value != "fallback" {
		t.Fatalf("empty default expansion value=%q unresolved=%v", value, unresolved)
	}
	_ = os.Unsetenv(name)
	value, unresolved = expandEnvironmentTemplate("${" + name + "}")
	if !unresolved || value != "" {
		t.Fatalf("missing required expansion value=%q unresolved=%v", value, unresolved)
	}
}

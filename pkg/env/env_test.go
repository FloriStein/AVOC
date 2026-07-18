package env

import (
	"testing"

	"avoc/pkg/logger"
)

func TestOptionalOr_ReturnsEnvValueWhenSet(t *testing.T) {
	t.Setenv("GOSTYLE_ENV_TEST_KEY", "value")
	if got := OptionalOr("GOSTYLE_ENV_TEST_KEY", "fallback"); got != "value" {
		t.Errorf("OptionalOr() = %q, want %q", got, "value")
	}
}

func TestOptionalOr_ReturnsFallbackWhenUnset(t *testing.T) {
	if got := OptionalOr("GOSTYLE_ENV_TEST_KEY_UNSET", "fallback"); got != "fallback" {
		t.Errorf("OptionalOr() = %q, want %q", got, "fallback")
	}
}

func TestOptionalOr_ReturnsFallbackWhenEmptyString(t *testing.T) {
	t.Setenv("GOSTYLE_ENV_TEST_KEY_EMPTY", "")
	if got := OptionalOr("GOSTYLE_ENV_TEST_KEY_EMPTY", "fallback"); got != "fallback" {
		t.Errorf("OptionalOr() = %q, want %q", got, "fallback")
	}
}

// Require's empty-value branch calls log.Fatal, which exits the process
// (os.Exit) — like pkg/logger's Fatal itself, that path isn't exercised
// in-process here. Only the pass-through branch is testable this way.
func TestRequire_ReturnsEnvValueWhenSet(t *testing.T) {
	t.Setenv("GOSTYLE_ENV_TEST_REQUIRED", "value")
	if got := Require("GOSTYLE_ENV_TEST_REQUIRED", logger.New("test")); got != "value" {
		t.Errorf("Require() = %q, want %q", got, "value")
	}
}

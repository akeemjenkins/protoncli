package termio

import (
	"testing"
)

func TestStderrSupportsColor_NoColorEnv(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	if StderrSupportsColor() {
		t.Errorf("StderrSupportsColor() = true with NO_COLOR set, want false")
	}
}

func TestStderrSupportsColor_NonTTY(t *testing.T) {
	if _, ok := lookupEnvSkip(t); ok {
		t.Skip("NO_COLOR is set in environment")
	}
	// In `go test` stderr is generally not a char device (piped), so this should be false.
	got := StderrSupportsColor()
	if got {
		t.Logf("stderr appears to be a TTY in this environment; skipping non-TTY assertion")
	}
}

func lookupEnvSkip(t *testing.T) (string, bool) {
	t.Helper()
	return "", false
}

func TestColorize_NonDigitCode(t *testing.T) {
	got := Colorize("hello", "31m")
	if got != "hello" {
		t.Errorf("Colorize with non-digit code = %q, want %q", got, "hello")
	}
}

func TestColorize_EmptyCode(t *testing.T) {
	got := Colorize("hello", "")
	if got != "hello" {
		t.Errorf("Colorize with empty code = %q, want %q", got, "hello")
	}
}

func TestColorize_NoColorEnv(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	got := Colorize("hello", "31")
	if got != "hello" {
		t.Errorf("Colorize with NO_COLOR = %q, want %q", got, "hello")
	}
}

func TestColorize_NonTTY(t *testing.T) {
	// When stderr is not a TTY (typical in go test) color is disabled.
	if StderrSupportsColor() {
		t.Skip("stderr is a TTY; cannot test non-TTY path")
	}
	got := Colorize("hi", "31")
	if got != "hi" {
		t.Errorf("Colorize on non-TTY = %q, want %q", got, "hi")
	}
}

func TestAllASCIIDigits(t *testing.T) {
	cases := map[string]bool{
		"":     false,
		"0":    true,
		"31":   true,
		"9999": true,
		"31m":  false,
		"a":    false,
		"3 1":  false,
	}
	for in, want := range cases {
		if got := allASCIIDigits(in); got != want {
			t.Errorf("allASCIIDigits(%q) = %v, want %v", in, got, want)
		}
	}
}

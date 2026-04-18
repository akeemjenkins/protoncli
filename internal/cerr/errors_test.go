package cerr

import (
	"errors"
	"fmt"
	"testing"
)

func TestKindString(t *testing.T) {
	cases := []struct {
		k    Kind
		want string
	}{
		{KindAPI, "api"},
		{KindAuth, "auth"},
		{KindValidation, "validation"},
		{KindConfig, "config"},
		{KindIMAP, "imap"},
		{KindClassify, "classify"},
		{KindState, "state"},
		{KindDiscovery, "discovery"},
		{KindInternal, "internal"},
		{Kind(999), "unknown"},
	}
	for _, c := range cases {
		if got := c.k.String(); got != c.want {
			t.Errorf("Kind(%d).String()=%q want %q", c.k, got, c.want)
		}
	}
}

func TestConstructorsAndExitCodes(t *testing.T) {
	sentinel := errors.New("sentinel")
	cases := []struct {
		name string
		err  *Error
		kind Kind
		code int
		exit int
	}{
		{"validation", Validation("bad %s", "x"), KindValidation, 400, ExitCodeValidation},
		{"auth", Auth("no token"), KindAuth, 401, ExitCodeAuth},
		{"imap", IMAP(sentinel, "fail %d", 1), KindIMAP, 502, ExitCodeIMAP},
		{"classify", Classify(sentinel, "nope"), KindClassify, 500, ExitCodeClassify},
		{"state", State(sentinel, "db"), KindState, 500, ExitCodeState},
		{"config", Config("missing"), KindConfig, 400, ExitCodeConfig},
		{"discovery", Discovery(sentinel, "dns"), KindDiscovery, 500, ExitCodeDiscovery},
		{"internal", Internal(sentinel, "oops"), KindInternal, 500, ExitCodeInternal},
		{"api", API(404, "notFound", "nope", "hint://x"), KindAPI, 404, ExitCodeAPI},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.err.Kind != c.kind {
				t.Errorf("kind=%v want %v", c.err.Kind, c.kind)
			}
			if c.err.Code != c.code {
				t.Errorf("code=%d want %d", c.err.Code, c.code)
			}
			if c.err.ExitCode() != c.exit {
				t.Errorf("exit=%d want %d", c.err.ExitCode(), c.exit)
			}
			if c.err.Error() != c.err.Message {
				t.Errorf("Error()=%q want %q", c.err.Error(), c.err.Message)
			}
		})
	}
}

func TestExitCodeUnknownKind(t *testing.T) {
	e := &Error{Kind: Kind(999)}
	if got := e.ExitCode(); got != ExitCodeInternal {
		t.Errorf("unknown kind exit=%d want %d", got, ExitCodeInternal)
	}
}

func TestFormatNoArgs(t *testing.T) {
	e := Validation("literal untouched")
	if e.Message != "literal untouched" {
		t.Errorf("format without args should not call Sprintf, got %q", e.Message)
	}
	e2 := Validation("v=%d", 7)
	if e2.Message != "v=7" {
		t.Errorf("format with args got %q", e2.Message)
	}
}

func TestUnwrapAndIsAs(t *testing.T) {
	sentinel := errors.New("sentinel")
	e := IMAP(sentinel, "wrap")
	if !errors.Is(e, sentinel) {
		t.Fatalf("errors.Is should find sentinel")
	}
	var target *Error
	if !errors.As(e, &target) {
		t.Fatalf("errors.As should extract *Error")
	}
	if target.Kind != KindIMAP {
		t.Errorf("target kind=%v want imap", target.Kind)
	}
	if e.Unwrap() != sentinel {
		t.Errorf("Unwrap mismatch")
	}
	noCause := Validation("x")
	if noCause.Unwrap() != nil {
		t.Errorf("no cause should unwrap to nil")
	}
}

func TestFrom(t *testing.T) {
	if From(nil) != nil {
		t.Errorf("From(nil) should be nil")
	}
	orig := Validation("x")
	if From(orig) != orig {
		t.Errorf("From should return *Error unchanged")
	}
	plain := fmt.Errorf("boom")
	got := From(plain)
	if got.Kind != KindInternal {
		t.Errorf("kind=%v want internal", got.Kind)
	}
	if got.Message != "boom" {
		t.Errorf("msg=%q want boom", got.Message)
	}
	if !errors.Is(got, plain) {
		t.Errorf("wrapped plain should be findable via errors.Is")
	}
	wrapped := fmt.Errorf("outer: %w", orig)
	if From(wrapped) != orig {
		t.Errorf("From should find *Error via errors.As on wrapped chain")
	}
}

func TestExitCodeDocsCompletenessAndOrder(t *testing.T) {
	want := []int{
		ExitCodeOK,
		ExitCodeAPI,
		ExitCodeAuth,
		ExitCodeValidation,
		ExitCodeConfig,
		ExitCodeIMAP,
		ExitCodeClassify,
		ExitCodeState,
		ExitCodeDiscovery,
		ExitCodeInternal,
	}
	if len(ExitCodeDocs) != len(want) {
		t.Fatalf("ExitCodeDocs len=%d want %d", len(ExitCodeDocs), len(want))
	}
	seen := map[int]bool{}
	for i, d := range ExitCodeDocs {
		if d.Code != want[i] {
			t.Errorf("ExitCodeDocs[%d].Code=%d want %d", i, d.Code, want[i])
		}
		if d.Description == "" {
			t.Errorf("ExitCodeDocs[%d] missing description", i)
		}
		if seen[d.Code] {
			t.Errorf("duplicate code %d", d.Code)
		}
		seen[d.Code] = true
	}
}

func TestAPIHintPreserved(t *testing.T) {
	e := API(403, "accessNotConfigured", "enable it", "https://x/y")
	if e.Hint != "https://x/y" {
		t.Errorf("hint lost: %q", e.Hint)
	}
}

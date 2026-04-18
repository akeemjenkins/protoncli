package labels

import (
	"reflect"
	"sort"
	"testing"

	"github.com/BurntSushi/toml"
)

func TestLoadSucceeds(t *testing.T) {
	tx, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if tx == nil {
		t.Fatal("Load returned nil taxonomy")
	}
	if Default == nil {
		t.Fatal("Default taxonomy is nil")
	}
}

func TestMustLoad(t *testing.T) {
	tx := MustLoad()
	if tx == nil {
		t.Fatal("MustLoad returned nil")
	}
}

func TestCanonicalSetHasElevenLabels(t *testing.T) {
	want := []string{
		"Finance", "Health", "Jobs", "Newsletters", "Orders",
		"Promotions", "Security", "Services", "Signups", "Social", "Travel",
	}
	got := Default.Canonical()
	if len(got) != 11 {
		t.Fatalf("expected 11 canonical labels, got %d: %v", len(got), got)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Canonical mismatch:\n got: %v\nwant: %v", got, want)
	}
}

func TestIsCanonical(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"Orders", true},
		{"Finance", true},
		{"Signups", true},
		{"orders", false},
		{"NotALabel", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := Default.IsCanonical(tc.in); got != tc.want {
			t.Errorf("IsCanonical(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestCanonicalFor(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantOK  bool
	}{
		{"Orders", "Orders", true},
		{"orders", "Orders", true},
		{"ORDERS", "Orders", true},
		{"order", "Orders", true},
		{"Labels/Orders", "Orders", true},
		{"labels/orders", "Orders", true},
		{"  Orders  ", "Orders", true},
		{"amazon", "Orders", true},
		{"2fa", "Security", true},
		{"inmail", "Jobs", true},
		{"e-commerce", "Promotions", true},
		{"ssl-renewal", "Signups", true},
		{"fly.io", "Services", true},
		{"completely-unknown-label", "", false},
		{"", "", false},
		{"   ", "", false},
	}
	for _, tc := range cases {
		got, ok := Default.CanonicalFor(tc.in)
		if ok != tc.wantOK || got != tc.want {
			t.Errorf("CanonicalFor(%q) = (%q, %v), want (%q, %v)", tc.in, got, ok, tc.want, tc.wantOK)
		}
	}
}

func TestNormalizeDedupesAndPreservesOrder(t *testing.T) {
	in := []string{"order", "Finance", "orders", "unknownthing", "2fa", "finance", "amazon"}
	want := []string{"Orders", "Finance", "Security"}
	got := Default.Normalize(in)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Normalize = %v, want %v", got, want)
	}
}

func TestNormalizeEmpty(t *testing.T) {
	if got := Default.Normalize(nil); len(got) != 0 {
		t.Errorf("Normalize(nil) = %v, want empty", got)
	}
	if got := Default.Normalize([]string{"xxx", "yyy"}); len(got) != 0 {
		t.Errorf("Normalize(unknowns) = %v, want empty", got)
	}
}

func TestDescribe(t *testing.T) {
	if d := Default.Describe("Orders"); d == "" {
		t.Error("Describe(Orders) returned empty string")
	}
	if d := Default.Describe("NotALabel"); d != "" {
		t.Errorf("Describe(NotALabel) = %q, want empty", d)
	}
}

// TestAliasRoundTrip asserts every alias in data/labels.toml maps back to
// the canonical name it's listed under.
func TestAliasRoundTrip(t *testing.T) {
	var f tomlFile
	if err := toml.Unmarshal(embeddedData, &f); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(f.Labels) == 0 {
		t.Fatal("no labels parsed from embedded toml")
	}
	tx := Default
	names := make([]string, 0, len(f.Labels))
	for n := range f.Labels {
		names = append(names, n)
	}
	sort.Strings(names)
	total := 0
	for _, name := range names {
		lbl := f.Labels[name]
		for _, alias := range lbl.Aliases {
			total++
			got, ok := tx.CanonicalFor(alias)
			if !ok {
				t.Errorf("alias %q under %q did not resolve", alias, name)
				continue
			}
			if got != name {
				t.Errorf("alias %q under %q resolved to %q", alias, name, got)
			}
		}
	}
	if total < 300 {
		t.Errorf("expected >300 aliases across all labels, got %d", total)
	}
	t.Logf("verified %d aliases across %d canonical labels", total, len(names))
}

func TestLoadFromErrors(t *testing.T) {
	if _, err := loadFrom([]byte("not = = valid toml [[[")); err == nil {
		t.Error("expected parse error for invalid toml")
	}
	if _, err := loadFrom([]byte("")); err == nil {
		t.Error("expected error for empty taxonomy")
	}
	conflicting := []byte(`
[labels.A]
description = "a"
aliases = ["x"]

[labels.B]
description = "b"
aliases = ["x"]
`)
	if _, err := loadFrom(conflicting); err == nil {
		t.Error("expected conflict error for duplicate alias")
	}
	withEmptyAlias := []byte(`
[labels.A]
description = "a"
aliases = ["", "keep"]
`)
	tx, err := loadFrom(withEmptyAlias)
	if err != nil {
		t.Fatalf("loadFrom: %v", err)
	}
	if got, ok := tx.CanonicalFor("keep"); !ok || got != "A" {
		t.Errorf("CanonicalFor(keep) = (%q,%v)", got, ok)
	}
}

func TestMustLoadPanicsOnBadData(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic")
		}
	}()
	// Exercise the panic path by reusing MustLoad's logic via a local helper.
	mustLoadFrom([]byte("xxxxx = = ="))
}

func mustLoadFrom(data []byte) *Taxonomy {
	t, err := loadFrom(data)
	if err != nil {
		panic(err)
	}
	return t
}

func TestNormalizeKey(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Orders", "orders"},
		{"Labels/Orders", "orders"},
		{"Labels/Labels/Orders", "orders"},
		{"  foo  ", "foo"},
		{"a/b/c", "abc"},
		{"", ""},
	}
	for _, tc := range cases {
		if got := normalizeKey(tc.in); got != tc.want {
			t.Errorf("normalizeKey(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

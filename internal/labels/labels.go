// Package labels provides the canonical email-label taxonomy loaded from
// an embedded TOML file.
package labels

import (
	_ "embed"
	"fmt"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

//go:embed data/labels.toml
var embeddedData []byte

// Label is a canonical label with its description and aliases.
type Label struct {
	Name        string
	Description string
	Aliases     []string
}

// Taxonomy is the loaded label taxonomy.
type Taxonomy struct {
	labels    map[string]Label
	aliasToID map[string]string
}

type tomlFile struct {
	Labels map[string]tomlLabel `toml:"labels"`
}

type tomlLabel struct {
	Description string   `toml:"description"`
	Aliases     []string `toml:"aliases"`
}

// Load parses the embedded TOML taxonomy and returns a Taxonomy.
func Load() (*Taxonomy, error) {
	return loadFrom(embeddedData)
}

func loadFrom(data []byte) (*Taxonomy, error) {
	var f tomlFile
	if err := toml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("labels: parse toml: %w", err)
	}
	if len(f.Labels) == 0 {
		return nil, fmt.Errorf("labels: no labels found in embedded taxonomy")
	}
	t := &Taxonomy{
		labels:    make(map[string]Label, len(f.Labels)),
		aliasToID: make(map[string]string),
	}
	for name, l := range f.Labels {
		lbl := Label{
			Name:        name,
			Description: l.Description,
			Aliases:     append([]string(nil), l.Aliases...),
		}
		t.labels[name] = lbl
		t.aliasToID[normalizeKey(name)] = name
		for _, a := range l.Aliases {
			key := normalizeKey(a)
			if key == "" {
				continue
			}
			if existing, ok := t.aliasToID[key]; ok && existing != name {
				return nil, fmt.Errorf("labels: alias %q maps to both %q and %q", a, existing, name)
			}
			t.aliasToID[key] = name
		}
	}
	return t, nil
}

// MustLoad returns a Taxonomy or panics on error.
func MustLoad() *Taxonomy {
	t, err := Load()
	if err != nil {
		panic(err)
	}
	return t
}

// Default is the package-level taxonomy loaded at init time.
var Default = MustLoad()

// Canonical returns the sorted list of canonical label names.
func (t *Taxonomy) Canonical() []string {
	out := make([]string, 0, len(t.labels))
	for name := range t.labels {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// IsCanonical reports whether name is a canonical label.
func (t *Taxonomy) IsCanonical(name string) bool {
	_, ok := t.labels[name]
	return ok
}

// CanonicalFor returns the canonical label name for an alias, if known.
func (t *Taxonomy) CanonicalFor(alias string) (string, bool) {
	key := normalizeKey(alias)
	if key == "" {
		return "", false
	}
	name, ok := t.aliasToID[key]
	return name, ok
}

// Normalize applies CanonicalFor to each label, drops unknowns, and
// dedupes while preserving first-seen order.
func (t *Taxonomy) Normalize(labels []string) []string {
	seen := make(map[string]struct{}, len(labels))
	out := make([]string, 0, len(labels))
	for _, l := range labels {
		name, ok := t.CanonicalFor(l)
		if !ok {
			continue
		}
		if _, dup := seen[name]; dup {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	return out
}

// Describe returns the description for a canonical label, or "" if unknown.
func (t *Taxonomy) Describe(name string) string {
	if l, ok := t.labels[name]; ok {
		return l.Description
	}
	return ""
}

// normalizeKey lowercases and strips "labels/" prefix, whitespace, and slashes.
func normalizeKey(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	for strings.HasPrefix(s, "labels/") {
		s = s[len("labels/"):]
	}
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "\t", "")
	s = strings.ReplaceAll(s, "/", "")
	return s
}

package termio

import "testing"

func TestSanitizeForTerminal(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"plain ascii", "hello world", "hello world"},
		{"preserve newline", "a\nb", "a\nb"},
		{"preserve tab", "a\tb", "a\tb"},
		{"strip ESC", "a\x1bb", "ab"},
		{"strip BEL", "a\x07b", "ab"},
		{"strip BS", "a\x08b", "ab"},
		{"strip CR", "a\rb", "ab"},
		{"strip NUL", "a\x00b", "ab"},
		{"strip DEL", "a\x7fb", "ab"},
		{"strip LRE", "a\u202Ab", "ab"},
		{"strip RLE", "a\u202Bb", "ab"},
		{"strip PDF", "a\u202Cb", "ab"},
		{"strip LRO", "a\u202Db", "ab"},
		{"strip RLO", "a\u202Eb", "ab"},
		{"strip LRI", "a\u2066b", "ab"},
		{"strip RLI", "a\u2067b", "ab"},
		{"strip FSI", "a\u2068b", "ab"},
		{"strip PDI", "a\u2069b", "ab"},
		{"strip ZWSP", "a\u200Bb", "ab"},
		{"strip ZWNJ", "a\u200Cb", "ab"},
		{"strip ZWJ", "a\u200Db", "ab"},
		{"strip BOM", "a\uFEFFb", "ab"},
		{"strip LSEP", "a\u2028b", "ab"},
		{"strip PSEP", "a\u2029b", "ab"},
		{"keep japanese", "日本語", "日本語"},
		{"keep accented", "café", "café"},
		{"keep greek", "αβγ", "αβγ"},
		{"mixed", "hi\x1b[31mRED\u202Eevil\nok", "hi[31mRED" + "evil\nok"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := SanitizeForTerminal(tc.in)
			if got != tc.want {
				t.Errorf("SanitizeForTerminal(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestIsDangerousUnicode(t *testing.T) {
	dangerous := []rune{
		0x202A, 0x202B, 0x202C, 0x202D, 0x202E,
		0x2066, 0x2067, 0x2068, 0x2069,
		0x200B, 0x200C, 0x200D, 0xFEFF,
		0x2028, 0x2029,
	}
	for _, r := range dangerous {
		if !IsDangerousUnicode(r) {
			t.Errorf("IsDangerousUnicode(%U) = false, want true", r)
		}
	}
	safe := []rune{'a', ' ', '\n', '\t', '日', 'é', 0x00, 0x2027, 0x206A}
	for _, r := range safe {
		if IsDangerousUnicode(r) {
			t.Errorf("IsDangerousUnicode(%U) = true, want false", r)
		}
	}
}

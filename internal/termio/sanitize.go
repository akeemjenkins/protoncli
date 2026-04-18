package termio

import "strings"

const (
	bidiLREStart rune = 0x202A
	bidiLREEnd   rune = 0x202E
	bidiLRIStart rune = 0x2066
	bidiLRIEnd   rune = 0x2069

	zwSpace      rune = 0x200B
	zwNonJoiner  rune = 0x200C
	zwJoiner     rune = 0x200D
	byteOrderBOM rune = 0xFEFF

	lineSeparator      rune = 0x2028
	paragraphSeparator rune = 0x2029
)

// IsDangerousUnicode reports whether r is a bidi override, zero-width, or line/paragraph separator.
func IsDangerousUnicode(r rune) bool {
	switch {
	case r >= bidiLREStart && r <= bidiLREEnd:
		return true
	case r >= bidiLRIStart && r <= bidiLRIEnd:
		return true
	case r == zwSpace, r == zwNonJoiner, r == zwJoiner, r == byteOrderBOM:
		return true
	case r == lineSeparator, r == paragraphSeparator:
		return true
	}
	return false
}

// SanitizeForTerminal strips ASCII control chars (except \n, \t) and dangerous Unicode from s.
func SanitizeForTerminal(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r == '\n' || r == '\t' {
			b.WriteRune(r)
			continue
		}
		if r < 0x20 || r == 0x7F {
			continue
		}
		if IsDangerousUnicode(r) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

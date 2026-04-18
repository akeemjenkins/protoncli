package parse

import (
	"bytes"
	"errors"
	"html"
	"io"
	"mime"
	"regexp"
	"strings"

	"github.com/emersion/go-message/mail"
)

const (
	defaultMaxInlineBytes = int64(1024 * 1024) // 1MB per inline part cap
)

// BestBodyText extracts a best-effort body text from an RFC822 message.
// It prefers text/plain, otherwise falls back to text/html (converted to text),
// otherwise returns the first inline part.
func BestBodyText(r io.Reader) (string, error) {
	mr, err := mail.CreateReader(r)
	if err != nil && mr == nil {
		return "", err
	}

	var (
		bestPlain string
		bestHTML  string
		firstAny  string
	)

	for {
		p, perr := mr.NextPart()
		if errors.Is(perr, io.EOF) {
			break
		}
		if perr != nil && p == nil {
			// Unknown charset/encoding can surface here, but if we don't have a part,
			// we can't proceed.
			return "", perr
		}

		switch p.Header.(type) {
		case *mail.InlineHeader:
			b, _ := io.ReadAll(io.LimitReader(p.Body, defaultMaxInlineBytes))
			s := strings.TrimSpace(string(b))
			if s == "" {
				continue
			}

			mediaType := headerMediaType(p.Header)
			switch strings.ToLower(mediaType) {
			case "text/plain", "":
				if len(s) > len(bestPlain) {
					bestPlain = s
				}
			case "text/html":
				if len(s) > len(bestHTML) {
					bestHTML = s
				}
			default:
				// Ignore other inline types.
			}

			if firstAny == "" {
				firstAny = s
			}
		case *mail.AttachmentHeader:
			// Ignore attachments for now.
		default:
			// Unknown part type, ignore.
		}
	}

	if bestPlain != "" {
		return normalizeText(bestPlain), nil
	}
	if bestHTML != "" {
		return normalizeText(htmlToText(bestHTML)), nil
	}
	if firstAny != "" {
		return normalizeText(firstAny), nil
	}
	return "", nil
}

func Snippet(s string, maxChars int) string {
	s = strings.TrimSpace(s)
	if maxChars <= 0 {
		return ""
	}
	if len(s) <= maxChars {
		return s
	}
	// Keep it simple: truncate by bytes; input is UTF-8 but for snippets this is OK.
	return strings.TrimSpace(s[:maxChars]) + "…"
}

func headerMediaType(h interface{ Get(string) string }) string {
	ct := strings.TrimSpace(h.Get("Content-Type"))
	if ct == "" {
		return ""
	}
	mediaType, _, err := mime.ParseMediaType(ct)
	if err != nil {
		// If parsing fails, fall back to raw ct up to semicolon.
		if i := strings.Index(ct, ";"); i >= 0 {
			return strings.TrimSpace(ct[:i])
		}
		return ct
	}
	return mediaType
}

var (
	reScript = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	reStyle  = regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	reTag    = regexp.MustCompile(`(?is)<[^>]+>`)
	reWS     = regexp.MustCompile(`[\t\r\f\v ]+`)
	reNL3    = regexp.MustCompile(`\n{3,}`)
)

func htmlToText(s string) string {
	// Remove scripts/styles, then tags.
	s = reScript.ReplaceAllString(s, "\n")
	s = reStyle.ReplaceAllString(s, "\n")
	s = reTag.ReplaceAllString(s, "\n")
	s = html.UnescapeString(s)
	return s
}

func normalizeText(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = reWS.ReplaceAllString(s, " ")
	s = reNL3.ReplaceAllString(s, "\n\n")
	// Trim each line and drop empty trailing lines.
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = strings.TrimSpace(lines[i])
	}
	s = strings.TrimSpace(strings.Join(lines, "\n"))
	// Collapse repeated blank lines again after trimming.
	s = reNL3.ReplaceAllString(s, "\n\n")
	// Avoid massive strings with weird nulls.
	s = strings.Map(func(r rune) rune {
		if r == 0 {
			return -1
		}
		return r
	}, s)
	// Ensure stable normalization for consumers.
	s = bytes.NewBufferString(s).String()
	return s
}

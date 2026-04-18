// Package llm provides high-level LLM verbs built on top of internal/ollama:
// Summarize / Digest / Extract. It defines the shared message type and common
// helpers used by all three verbs.
package llm

import (
	"fmt"
	"strings"
)

// Message is a normalized email payload passed into LLM verbs. Callers are
// responsible for populating these fields from their IMAP fetch pipeline.
type Message struct {
	UID     uint32
	Mailbox string
	Subject string
	From    string
	Date    string
	Body    string
}

// CommonOpts bundles options shared by every LLM verb. Individual verbs may
// embed or replicate these fields in their specialized opts struct.
type CommonOpts struct {
	// MaxInput caps the number of body bytes sent to the model. 0 disables
	// the cap (not recommended for long messages).
	MaxInput int
	// UserContext is free-form text appended to the user prompt, allowing
	// callers to inject per-run context ("focus on invoices", etc.).
	UserContext string
	// Model overrides the default model name; when empty the caller's
	// configured default is used.
	Model string
}

// buildUserPrompt formats a single message into a canonical textual prompt,
// trimming the body to maxInput bytes. extra is optional free-form context
// appended at the end (e.g. the user-supplied --user-context flag value).
func buildUserPrompt(m Message, maxInput int, extra string) string {
	body := m.Body
	if maxInput > 0 && len(body) > maxInput {
		body = body[:maxInput]
	}
	var b strings.Builder
	if m.From != "" {
		fmt.Fprintf(&b, "From: %s\n", m.From)
	}
	if m.Subject != "" {
		fmt.Fprintf(&b, "Subject: %s\n", m.Subject)
	}
	if m.Date != "" {
		fmt.Fprintf(&b, "Date: %s\n", m.Date)
	}
	if body != "" {
		b.WriteString("\nBody:\n")
		b.WriteString(body)
	}
	if strings.TrimSpace(extra) != "" {
		b.WriteString("\n\nAdditional context from the user:\n")
		b.WriteString(extra)
	}
	return b.String()
}

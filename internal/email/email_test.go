package email

import (
	"testing"
	"time"
)

func TestMessageZeroValue(t *testing.T) {
	var m Message
	if m.UID != 0 || m.UIDValidity != 0 {
		t.Errorf("expected zero UIDs, got %v / %v", m.UID, m.UIDValidity)
	}
	if m.Mailbox != "" || m.Subject != "" {
		t.Errorf("expected empty string fields on zero value")
	}
	if !m.Date.IsZero() {
		t.Errorf("expected zero date")
	}
}

func TestMessagePopulate(t *testing.T) {
	ts := time.Date(2024, 5, 1, 12, 0, 0, 0, time.UTC)
	m := Message{
		Mailbox:     "INBOX",
		UIDValidity: 123,
		UID:         42,
		MessageID:   "<id@host>",
		From:        "alice@example.com",
		Subject:     "hi",
		Date:        ts,
		Body:        "body",
		BodySnippet: "body",
	}
	if m.Mailbox != "INBOX" || m.UIDValidity != 123 || m.UID != 42 {
		t.Errorf("unexpected values: %+v", m)
	}
	if !m.Date.Equal(ts) {
		t.Errorf("date mismatch: %v", m.Date)
	}
	if m.Body != m.BodySnippet {
		t.Errorf("expected body==snippet")
	}
}

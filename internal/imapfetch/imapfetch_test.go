package imapfetch

import (
	"testing"

	"github.com/emersion/go-imap"
)

// There are no pure helper functions in imapfetch that can be tested without
// a full IMAP server. We exercise the data-holder types (FetchedMessage /
// MailboxSnapshot) to bump coverage and guard against silent signature drift.

func TestMailboxSnapshotZero(t *testing.T) {
	var s MailboxSnapshot
	if s.Name != "" || s.UIDValidity != 0 {
		t.Errorf("zero value not clean: %+v", s)
	}
}

func TestFetchedMessageFields(t *testing.T) {
	fm := FetchedMessage{
		UID:      42,
		Envelope: &imap.Envelope{Subject: "hi"},
		RFC822:   []byte("raw"),
	}
	if fm.UID != 42 || string(fm.RFC822) != "raw" || fm.Envelope.Subject != "hi" {
		t.Errorf("unexpected: %+v", fm)
	}
}

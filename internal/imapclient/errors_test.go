package imapclient

import (
	"errors"
	"fmt"
	"io"
	"net"
	"testing"
)

func TestIsConnectionError(t *testing.T) {
	netErr := &net.OpError{Op: "read", Net: "tcp", Err: errors.New("connection reset by peer")}

	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"io.EOF", io.EOF, true},
		{"net.OpError_reset", netErr, true},
		{"connection_closed_substring", errors.New("imap: connection closed"), true},
		{"not_logged_in_lowercase", errors.New("server said: not logged in"), true},
		{"not_logged_in_capital", errors.New("Not logged in: session expired"), true},
		{"use_of_closed_network", errors.New("write tcp: use of closed network connection"), true},
		{"broken_pipe", errors.New("write: broken pipe"), true},
		{"i_o_timeout", errors.New("read tcp 127.0.0.1:1143: i/o timeout"), true},
		{"unrelated_error", errors.New("mailbox does not exist"), false},
		{"empty_error", errors.New(""), false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsConnectionError(tc.err); got != tc.want {
				t.Fatalf("IsConnectionError(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

func TestWrapIfConnection_MarksSentinel(t *testing.T) {
	orig := errors.New("connection reset by peer")
	wrapped := WrapIfConnection(orig)
	if wrapped == nil {
		t.Fatal("expected non-nil")
	}
	if !errors.Is(wrapped, ErrConnection) {
		t.Fatalf("expected wrapped to satisfy errors.Is(ErrConnection); got %v", wrapped)
	}
	if !errors.Is(wrapped, orig) {
		t.Fatalf("expected wrapped to preserve original cause")
	}
}

func TestWrapIfConnection_PassThroughUnrelated(t *testing.T) {
	orig := errors.New("mailbox does not exist")
	if got := WrapIfConnection(orig); got != orig {
		t.Fatalf("unrelated error should be returned unchanged; got %v", got)
	}
}

func TestWrapIfConnection_Nil(t *testing.T) {
	if got := WrapIfConnection(nil); got != nil {
		t.Fatalf("nil in, nil out; got %v", got)
	}
}

func TestWrapIfConnection_Idempotent(t *testing.T) {
	orig := errors.New("connection closed")
	once := WrapIfConnection(orig)
	twice := WrapIfConnection(once)
	if once != twice {
		t.Fatalf("Wrap of an already-wrapped error should be a no-op; got %v vs %v", once, twice)
	}
}

func TestWrap_MatchesWrapIfConnection(t *testing.T) {
	orig := fmt.Errorf("surrounding: %w", io.EOF)
	if !IsConnectionError(Wrap(orig)) {
		t.Fatal("Wrap(io.EOF chain) should still be a connection error")
	}
}

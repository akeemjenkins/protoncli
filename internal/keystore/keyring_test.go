package keystore

import (
	"errors"
	"testing"

	"github.com/zalando/go-keyring"
)

// All keyring tests use MockInit — the library ships an in-memory mock that
// replaces every backend call. This keeps tests deterministic on every OS.

func TestKeyringBackend_RoundTrip(t *testing.T) {
	keyring.MockInit()
	kb := newKeyringBackend()

	if !kb.Available() {
		t.Fatal("keyring backend should always be Available")
	}
	want := Credentials{Username: "alice@proton.me", Password: "bridge-pw"}
	if err := kb.Set(want); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := kb.Get()
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestKeyringBackend_DeleteThenGet(t *testing.T) {
	keyring.MockInit()
	kb := newKeyringBackend()
	if err := kb.Set(Credentials{Username: "u", Password: "p"}); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := kb.Delete(); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := kb.Get(); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get err = %v, want ErrNotFound", err)
	}
	// Idempotent.
	if err := kb.Delete(); err != nil {
		t.Errorf("Delete idempotent: %v", err)
	}
}

func TestKeyringBackend_Name(t *testing.T) {
	kb := newKeyringBackend()
	if kb.Name() != "system keyring" {
		t.Errorf("name = %q", kb.Name())
	}
}

func TestKeyringBackend_GetNotFound(t *testing.T) {
	keyring.MockInit()
	kb := newKeyringBackend()
	if _, err := kb.Get(); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get err = %v, want ErrNotFound", err)
	}
}

func TestKeyringBackend_LegacyPayloadWithoutSeparator(t *testing.T) {
	keyring.MockInit()
	// Write a raw value that lacks the \x00 separator.
	if err := keyring.Set(ServiceName, KeyringAccount, "legacy-blob"); err != nil {
		t.Fatalf("seed: %v", err)
	}
	kb := newKeyringBackend()
	got, err := kb.Get()
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Username != "" || got.Password != "legacy-blob" {
		t.Errorf("legacy payload mapping unexpected: %+v", got)
	}
}

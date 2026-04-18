package keystore

import (
	"errors"
	"strings"

	"github.com/zalando/go-keyring"
)

// keyringBackend stores the IMAP credential pair in the OS keyring.
//
// Encoding: the username and password are packed as "username\x00password" and
// stored as a single keyring entry under service=ServiceName / account=KeyringAccount.
// Packing into one entry keeps the Set/Delete operations atomic from the caller's
// point of view (no partial writes across username- and password-only entries).
//
// Availability: the go-keyring library reports errors at call time on systems
// where no backend is reachable (e.g. Linux without libsecret). Available()
// therefore always returns true and the caller must handle Set/Get errors by
// falling back to another backend. This avoids a side-effect probe on boot.
type keyringBackend struct{}

func newKeyringBackend() *keyringBackend { return &keyringBackend{} }

// Name satisfies Backend.
func (k *keyringBackend) Name() string { return "system keyring" }

// Available satisfies Backend. See type-level godoc for rationale.
func (k *keyringBackend) Available() bool { return true }

// Get reads the packed credential entry from the OS keyring.
func (k *keyringBackend) Get() (Credentials, error) {
	raw, err := keyring.Get(ServiceName, KeyringAccount)
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return Credentials{}, ErrNotFound
		}
		return Credentials{}, err
	}
	idx := strings.IndexByte(raw, 0x00)
	if idx < 0 {
		// Older/foreign payload — treat the entire value as the password for
		// some resilience; username stays empty so callers fail loudly.
		return Credentials{Password: raw}, nil
	}
	return Credentials{Username: raw[:idx], Password: raw[idx+1:]}, nil
}

// Set writes the packed credential entry to the OS keyring.
func (k *keyringBackend) Set(c Credentials) error {
	packed := c.Username + "\x00" + c.Password
	return keyring.Set(ServiceName, KeyringAccount, packed)
}

// Delete removes the credential entry from the OS keyring. A missing entry is
// not an error — Delete is idempotent.
func (k *keyringBackend) Delete() error {
	err := keyring.Delete(ServiceName, KeyringAccount)
	if err == nil {
		return nil
	}
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return err
}

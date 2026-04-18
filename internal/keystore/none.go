package keystore

import "fmt"

// noneBackend is the fallback used when no real backend is available. It
// reports the problem on Set/Delete so callers can surface a clear hint.
type noneBackend struct{}

func (n *noneBackend) Name() string   { return "none" }
func (n *noneBackend) Available() bool { return true }

func (n *noneBackend) Get() (Credentials, error) { return Credentials{}, ErrNotFound }

func (n *noneBackend) Set(Credentials) error {
	return fmt.Errorf("no credential backend available: install libsecret (Linux) or set PM_KEYSTORE_PASSPHRASE to enable the encrypted-file backend")
}

func (n *noneBackend) Delete() error {
	return fmt.Errorf("no credential backend available: nothing to delete")
}

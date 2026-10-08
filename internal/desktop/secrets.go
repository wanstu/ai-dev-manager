package desktop

import (
	"errors"

	"ai-dev-manager-v2/internal/secretvault"
)

type secretManagementBackend interface {
	SecretList() ([]secretvault.Metadata, error)
	SecretSet(string, string) (secretvault.Metadata, error)
	SecretDelete(string) error
}

func (a *Adapter) ListSecrets() ([]secretvault.Metadata, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	b, ok := a.management.(secretManagementBackend)
	if !ok {
		return nil, errors.New("connected ADM Gateway does not support encrypted credentials")
	}
	return b.SecretList()
}
func (a *Adapter) SaveSecret(name, value string) (secretvault.Metadata, error) {
	if err := a.ready(); err != nil {
		return secretvault.Metadata{}, err
	}
	b, ok := a.management.(secretManagementBackend)
	if !ok {
		return secretvault.Metadata{}, errors.New("connected ADM Gateway does not support encrypted credentials")
	}
	return b.SecretSet(name, value)
}
func (a *Adapter) DeleteSecret(name string) error {
	if err := a.ready(); err != nil {
		return err
	}
	b, ok := a.management.(secretManagementBackend)
	if !ok {
		return errors.New("connected ADM Gateway does not support encrypted credentials")
	}
	return b.SecretDelete(name)
}

//go:build !windows

package secretvault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
)

// Linux/headless fallback: AES-256-GCM with a separate 0600 per-install key.
// The host service account and root still have access. Back up the key with
// the ciphertext; deleting it makes existing credentials unrecoverable.
func localKey(path string) ([]byte, error) {
	name := path + ".key"
	key, err := os.ReadFile(name)
	if err == nil {
		info, err := os.Stat(name)
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
			return nil, fmt.Errorf("secret key permissions must be 0600")
		}
		if len(key) != 32 {
			return nil, fmt.Errorf("secret key invalid")
		}
		return key, nil
	}
	if !os.IsNotExist(err) {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(name), 0700); err != nil {
		return nil, err
	}
	key = make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if os.IsExist(err) {
		return localKey(path)
	}
	if err != nil {
		return nil, err
	}
	if _, err := file.Write(key); err != nil {
		file.Close()
		os.Remove(name)
		return nil, err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		os.Remove(name)
		return nil, err
	}
	if err := file.Close(); err != nil {
		return nil, err
	}
	return key, nil
}
func gcmFor(path string) (cipher.AEAD, error) {
	key, err := localKey(path)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
func protect(src []byte, path string) ([]byte, error) {
	gcm, err := gcmFor(path)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, src, nil), nil
}
func unprotect(src []byte, path string) ([]byte, error) {
	gcm, err := gcmFor(path)
	if err != nil {
		return nil, err
	}
	if len(src) < gcm.NonceSize() {
		return nil, fmt.Errorf("secret ciphertext invalid")
	}
	return gcm.Open(nil, src[:gcm.NonceSize()], src[gcm.NonceSize():], nil)
}

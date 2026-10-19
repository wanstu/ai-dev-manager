package sandbox

import (
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"math/big"
	"strings"
	"unicode"
)

const (
	windowsSandboxUserPrefix = "adm_sb_"
	windowsSandboxUserHashLen = 12
	sandboxPasswordLength     = 32
)

type LocalAccount struct {
	Username string `json:"username"`
	SID      string `json:"sid"`
}

func StableWindowsSandboxUsername(ownerSID string) (string, error) {
	ownerSID = strings.TrimSpace(ownerSID)
	if ownerSID == "" {
		return "", fmt.Errorf("owner SID is required")
	}
	sum := sha256.Sum256([]byte(strings.ToUpper(ownerSID)))
	return windowsSandboxUserPrefix + fmt.Sprintf("%x", sum[:])[:windowsSandboxUserHashLen], nil
}

func GenerateSandboxPassword() (string, error) {
	const (
		lower   = "abcdefghijkmnopqrstuvwxyz"
		upper   = "ABCDEFGHJKLMNPQRSTUVWXYZ"
		digits  = "23456789"
		special = "!@#$%_-+="
	)
	all := lower + upper + digits + special

	result := make([]byte, sandboxPasswordLength)
	required := []string{lower, upper, digits, special}
	for i, alphabet := range required {
		value, err := randomByteFrom(alphabet)
		if err != nil {
			return "", err
		}
		result[i] = value
	}
	for i := len(required); i < len(result); i++ {
		value, err := randomByteFrom(all)
		if err != nil {
			return "", err
		}
		result[i] = value
	}
	for i := len(result) - 1; i > 0; i-- {
		jValue, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if err != nil {
			return "", fmt.Errorf("shuffle sandbox password: %w", err)
		}
		j := int(jValue.Int64())
		result[i], result[j] = result[j], result[i]
	}
	return string(result), nil
}

func ValidateSandboxPassword(password string) error {
	if len(password) < 24 {
		return fmt.Errorf("sandbox password must contain at least 24 characters")
	}
	var lower, upper, digit, special bool
	for _, r := range password {
		switch {
		case unicode.IsLower(r):
			lower = true
		case unicode.IsUpper(r):
			upper = true
		case unicode.IsDigit(r):
			digit = true
		default:
			special = true
		}
	}
	if !lower || !upper || !digit || !special {
		return fmt.Errorf("sandbox password must contain lowercase, uppercase, digit, and special characters")
	}
	return nil
}

func randomByteFrom(alphabet string) (byte, error) {
	if alphabet == "" {
		return 0, fmt.Errorf("password alphabet is empty")
	}
	value, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
	if err != nil {
		return 0, fmt.Errorf("generate sandbox password: %w", err)
	}
	return alphabet[value.Int64()], nil
}

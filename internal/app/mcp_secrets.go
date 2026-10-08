package app

import (
	"fmt"
	"regexp"
	"strings"

	"ai-dev-manager-v2/internal/secretvault"
)

var secretTemplateRE = regexp.MustCompile(`\$\{secret:([A-Za-z][A-Za-z0-9_.-]{0,79})\}`)

func (s *Service) SecretList() ([]secretvault.Metadata, error) { return s.Secrets.List() }
func (s *Service) SecretSet(name, value string) (secretvault.Metadata, error) {
	if err := s.Secrets.Set(name, value); err != nil {
		return secretvault.Metadata{}, err
	}
	return secretvault.Metadata{Name: name}, nil
}
func (s *Service) SecretDelete(name string) error {
	if !secretvault.ValidName(name) {
		return fmt.Errorf("invalid secret name")
	}
	mcps, err := s.MCPs.List()
	if err != nil {
		return err
	}
	token := "${secret:" + name + "}"
	for _, m := range mcps {
		for _, v := range m.HeaderRefs {
			if strings.Contains(v, token) {
				return fmt.Errorf("secret is referenced by MCP %q", m.Name)
			}
		}
		for _, v := range m.EnvRefs {
			if strings.Contains(v, token) {
				return fmt.Errorf("secret is referenced by MCP %q", m.Name)
			}
		}
	}
	return s.Secrets.Delete(name)
}

func (s *Service) expandMCPTemplate(template string) (string, bool) {
	// Only whole secret references or a single explicit bearer/header prefix:
	// never run env interpolation over decrypted secret plaintext.
	match := secretTemplateRE.FindStringIndex(template)
	if match != nil {
		if strings.Contains(template[:match[0]], "$") || strings.Contains(template[match[1]:], "$") {
			return "", true
		}
		name := template[match[0]+len("${secret:") : match[1]-1]
		value, err := s.Secrets.Get(name)
		if err != nil {
			return "", true
		}
		return template[:match[0]] + value + template[match[1]:], false
	}
	if strings.Contains(template, "${secret:") {
		return "", true
	}
	return s.expandEnvironmentTemplate(template)
}

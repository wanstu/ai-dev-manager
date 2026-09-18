package dotenv

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"ai-dev-manager-v2/internal/configpath"
)

const FileName = ".env"

// LoadDefaultFiles loads ADM dotenv configuration with this precedence:
//
// explicit process environment > executable-directory .env > ~/.config/adm/.env
//
// Missing files are ignored. The executable-directory file may override values
// from the user configuration file, but neither file overrides variables that
// were already present in the process environment.
func LoadDefaultFiles() error {
	userPath, err := configpath.File(FileName)
	if err != nil {
		return fmt.Errorf("resolve ADM config path for .env: %w", err)
	}
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable path for .env: %w", err)
	}
	return loadLayeredFiles(userPath, filepath.Join(filepath.Dir(executable), FileName))
}

// LoadFromExecutableDir is retained for callers that explicitly want only the
// executable-directory .env.
func LoadFromExecutableDir() error {
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable path for .env: %w", err)
	}
	return LoadFile(filepath.Join(filepath.Dir(executable), FileName), false)
}

func loadLayeredFiles(userPath, executablePath string) error {
	explicit := currentEnvironmentKeys()
	merged := map[string]string{}

	load := func(path string) error {
		if strings.TrimSpace(path) == "" {
			return nil
		}
		values, err := ParseFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		for key, value := range values {
			merged[key] = value
		}
		return nil
	}

	if err := load(userPath); err != nil {
		return err
	}
	if filepath.Clean(executablePath) != filepath.Clean(userPath) {
		if err := load(executablePath); err != nil {
			return err
		}
	}
	for key, value := range merged {
		if explicit[key] {
			continue
		}
		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("set %s from ADM .env: %w", key, err)
		}
	}
	return nil
}

func currentEnvironmentKeys() map[string]bool {
	result := map[string]bool{}
	for _, item := range os.Environ() {
		key, _, ok := strings.Cut(item, "=")
		if ok {
			result[key] = true
		}
	}
	return result
}

// LoadFile reads dotenv-style KEY=VALUE pairs and applies them to the current
// process environment. When override is false, keys that already exist in the
// process environment are left unchanged.
func LoadFile(path string, override bool) error {
	values, err := ParseFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for key, value := range values {
		if !override {
			if _, exists := os.LookupEnv(key); exists {
				continue
			}
		}
		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("set %s from %s: %w", key, path, err)
		}
	}
	return nil
}

// ParseFile parses a .env file into environment key/value strings. Values are
// always strings; DB_ROW=1000 and DB_ENABLE=FALSE are not type-converted.
func ParseFile(path string) (map[string]string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	values, err := Parse(content)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return values, nil
}

// Parse supports a conservative dotenv subset:
//
//	KEY=value
//	KEY="value with spaces"
//	KEY='literal value'
//	export KEY=value
//
// Blank lines and # comments are ignored. Double quoted values support common
// backslash escapes. Inline comments are recognized only for unquoted values
// when # is preceded by whitespace.
func Parse(content []byte) (map[string]string, error) {
	values := map[string]string{}
	scanner := bufio.NewScanner(bytes.NewReader(content))
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("line %d: expected KEY=VALUE", lineNumber)
		}
		key = strings.TrimSpace(key)
		if !validKey(key) {
			return nil, fmt.Errorf("line %d: invalid environment key %q", lineNumber, key)
		}
		parsed, err := parseValue(strings.TrimSpace(value), lineNumber)
		if err != nil {
			return nil, err
		}
		values[key] = parsed
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return values, nil
}

func validKey(key string) bool {
	if key == "" {
		return false
	}
	for i, r := range key {
		if i == 0 {
			if r != '_' && !unicode.IsLetter(r) {
				return false
			}
			continue
		}
		if r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func parseValue(value string, lineNumber int) (string, error) {
	if value == "" {
		return "", nil
	}
	switch value[0] {
	case '\'', '"':
		return parseQuotedValue(value, lineNumber)
	default:
		return stripInlineComment(value), nil
	}
}

func parseQuotedValue(value string, lineNumber int) (string, error) {
	quote := value[0]
	var builder strings.Builder
	escaped := false
	for i := 1; i < len(value); i++ {
		ch := value[i]
		if quote == '"' && escaped {
			switch ch {
			case 'n':
				builder.WriteByte('\n')
			case 'r':
				builder.WriteByte('\r')
			case 't':
				builder.WriteByte('\t')
			case '\\', '"':
				builder.WriteByte(ch)
			default:
				builder.WriteByte(ch)
			}
			escaped = false
			continue
		}
		if quote == '"' && ch == '\\' {
			escaped = true
			continue
		}
		if ch == quote {
			remainder := strings.TrimSpace(value[i+1:])
			if remainder != "" && !strings.HasPrefix(remainder, "#") {
				return "", fmt.Errorf("line %d: unexpected text after quoted value", lineNumber)
			}
			return builder.String(), nil
		}
		builder.WriteByte(ch)
	}
	if escaped {
		builder.WriteByte('\\')
	}
	return "", fmt.Errorf("line %d: unterminated quoted value", lineNumber)
}

func stripInlineComment(value string) string {
	for i, r := range value {
		if r != '#' {
			continue
		}
		if i == 0 || unicode.IsSpace(rune(value[i-1])) {
			return strings.TrimSpace(value[:i])
		}
	}
	return strings.TrimSpace(value)
}

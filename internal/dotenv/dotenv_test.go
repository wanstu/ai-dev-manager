package dotenv

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseSupportsQuotedSpacesAmpersandAndStringValues(t *testing.T) {
	values, err := Parse([]byte(`
# local config
DB_NAME="ABDC"
DB_PWD="ABC D & "
DB_ROW=1000
DB_ENALBLE=FALSE
export ADM_V2_URL=http://127.0.0.1:43137 # inline comment
SINGLE='literal # value'
`))
	if err != nil {
		t.Fatal(err)
	}
	assertValue(t, values, "DB_NAME", "ABDC")
	assertValue(t, values, "DB_PWD", "ABC D & ")
	assertValue(t, values, "DB_ROW", "1000")
	assertValue(t, values, "DB_ENALBLE", "FALSE")
	assertValue(t, values, "ADM_V2_URL", "http://127.0.0.1:43137")
	assertValue(t, values, "SINGLE", "literal # value")
}

func TestLoadFileDoesNotOverrideExistingEnvironmentByDefault(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	if err := os.WriteFile(path, []byte("ADM_DOTENV_EXISTING=file\nADM_DOTENV_NEW=from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ADM_DOTENV_EXISTING", "process")
	if err := os.Unsetenv("ADM_DOTENV_NEW"); err != nil {
		t.Fatal(err)
	}
	defer os.Unsetenv("ADM_DOTENV_NEW")

	if err := LoadFile(path, false); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("ADM_DOTENV_EXISTING"); got != "process" {
		t.Fatalf("existing env = %q, want process", got)
	}
	if got := os.Getenv("ADM_DOTENV_NEW"); got != "from-file" {
		t.Fatalf("new env = %q, want from-file", got)
	}
}

func TestLoadFileMissingIsNoop(t *testing.T) {
	if err := LoadFile(filepath.Join(t.TempDir(), FileName), false); err != nil {
		t.Fatal(err)
	}
}

func TestParseRejectsMalformedLines(t *testing.T) {
	for name, content := range map[string]string{
		"missing equals": "DB_NAME\n",
		"bad key":        "DB-NAME=value\n",
		"unterminated":   "DB_NAME=\"value\n",
		"trailing text":  "DB_NAME=\"value\" trailing\n",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse([]byte(content)); err == nil {
				t.Fatal("Parse succeeded, want error")
			}
		})
	}
}

func assertValue(t *testing.T, values map[string]string, key, want string) {
	t.Helper()
	if got := values[key]; got != want {
		t.Fatalf("%s = %q, want %q", key, got, want)
	}
}

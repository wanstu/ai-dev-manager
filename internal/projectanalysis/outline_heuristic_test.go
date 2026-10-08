package projectanalysis

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFixture(t *testing.T, root, path, source string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
}

func installFixtureIndex(t *testing.T, root string, result Result) {
	t.Helper()
	for path, data := range map[string]string{
		OverviewRelativePath:      result.Markdown,
		IndexManifestRelativePath: result.ManifestJSON,
		IndexSymbolsRelativePath:  result.SymbolsJSONL,
		IndexCallsRelativePath:    result.CallsJSONL,
		IndexFilesRelativePath:    result.FilesJSONL,
	} {
		writeFixture(t, root, path, data)
	}
}

func TestPHPMethodQualifiedNamesAndExactLookup(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "src/Controller.php", `<?php
namespace Demo\App;
// class Fake {}
class First {
    public function run() { $text = "}"; }
}
class Second {
    public static function run() {}
}
function run() {}
`)
	result, err := Analyze(root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if result.PHPFiles != 1 {
		t.Fatalf("PHP files = %d", result.PHPFiles)
	}
	for _, want := range []string{
		`"qualified_name":"Demo\\App\\First::run"`,
		`"qualified_name":"Demo\\App\\Second::run"`,
		`"qualified_name":"Demo\\App\\run"`,
		`"kind":"method"`,
	} {
		if !strings.Contains(result.SymbolsJSONL, want) {
			t.Fatalf("missing %s in symbols: %s", want, result.SymbolsJSONL)
		}
	}
	if strings.Contains(result.SymbolsJSONL, "Fake") {
		t.Fatalf("comment declarations were indexed: %s", result.SymbolsJSONL)
	}
	installFixtureIndex(t, root, result)
	byClass, err := QueryIndex(root, IndexQuery{Query: "Demo\\App\\Second::run", Exact: true})
	if err != nil || byClass.Returned != 1 {
		t.Fatalf("class method exact lookup: result=%+v err=%v", byClass, err)
	}
	byMethod, err := QueryIndex(root, IndexQuery{Query: "run", Exact: true})
	if err != nil || byMethod.Returned != 3 {
		t.Fatalf("method name exact lookup: result=%+v err=%v", byMethod, err)
	}
}

func TestJavaScriptAndTypeScriptStaticIndex(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "ui/app.js", `// function Fake() {}
const text = "class NotReal {}";
export function perform() {}
export async function load() {}
export class Widget {
  method() {
    function nested() {}
  }
}
export const createWidget = (name) => name;
`)
	writeFixture(t, root, "ui/types.ts", `export interface Config { value: string }
export type ConfigId = string;
export function parseConfig(input: Config) { return input; }
`)
	result, err := Analyze(root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if result.JSFiles != 1 || result.TSFiles != 1 {
		t.Fatalf("JS/TS counts = %d/%d", result.JSFiles, result.TSFiles)
	}
	if !strings.Contains(strings.Join(result.Languages, ","), "JavaScript,TypeScript") {
		t.Fatalf("languages=%v", result.Languages)
	}
	for _, want := range []string{
		`"name":"perform"`,
		`"name":"load"`,
		`"name":"Widget"`,
		`"name":"createWidget"`,
		`"name":"Config"`,
		`"name":"ConfigId"`,
		`"name":"parseConfig"`,
	} {
		if !strings.Contains(result.SymbolsJSONL, want) {
			t.Fatalf("missing %s in symbols: %s", want, result.SymbolsJSONL)
		}
	}
	for _, forbidden := range []string{`"name":"Fake"`, `"name":"NotReal"`, `"name":"nested"`} {
		if strings.Contains(result.SymbolsJSONL, forbidden) {
			t.Fatalf("unexpected %s in symbols: %s", forbidden, result.SymbolsJSONL)
		}
	}
	installFixtureIndex(t, root, result)
	found, err := QueryIndex(root, IndexQuery{Query: "createWidget", Exact: true, Language: "JavaScript"})
	if err != nil || found.Returned != 1 || found.Matches[0].Path != "ui/app.js" {
		t.Fatalf("JS exact lookup: result=%+v err=%v", found, err)
	}
	foundTS, err := QueryIndex(root, IndexQuery{Query: "Config", Exact: true, Language: "TypeScript"})
	if err != nil || foundTS.Returned != 1 {
		t.Fatalf("TS exact lookup: result=%+v err=%v", foundTS, err)
	}
}

package runtime

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"ai-dev-manager/internal/model"
)

func TestReadOnlyFilesAndSearchAreReadableButNotWritable(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("hello world\nsecond line\n"), 0o644); err != nil {
		t.Fatalf("WriteFile a.txt error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "binary.bin"), []byte{'x', 0, 'h', 'e', 'l', 'l', 'o'}, 0o644); err != nil {
		t.Fatalf("WriteFile binary error = %v", err)
	}

	runtime := mustNative(t, root, model.Policy{Mode: string(ModeReadOnly)})
	data, info, err := runtime.Read("a.txt", 0)
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if string(data) != "hello world\nsecond line\n" || info.Path != "a.txt" {
		t.Fatalf("unexpected read result: data=%q info=%+v", data, info)
	}

	entries, err := runtime.Tree(".", TreeOptions{})
	if err != nil {
		t.Fatalf("Tree() error = %v", err)
	}
	if len(entries) != 2 || entries[0].Path != "a.txt" || entries[1].Path != "binary.bin" {
		t.Fatalf("unexpected tree: %+v", entries)
	}

	matches, err := runtime.Search(SearchOptions{Path: ".", Query: "hello"})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	want := []SearchMatch{{Path: "a.txt", Line: 1, Text: "hello world"}}
	if !reflect.DeepEqual(matches, want) {
		t.Fatalf("Search() = %+v, want %+v", matches, want)
	}

	_, err = runtime.Write("new.txt", []byte("x"), false)
	assertRuntimeErrorKind(t, err, ErrReadOnly)
	_, err = runtime.Edit("a.txt", "hello", "bye", 1)
	assertRuntimeErrorKind(t, err, ErrReadOnly)
	_, err = runtime.Delete("a.txt")
	assertRuntimeErrorKind(t, err, ErrReadOnly)
}

func TestWorkspaceWriteCanWriteAndExactEditAtomically(t *testing.T) {
	root := t.TempDir()
	runtime := mustNative(t, root, model.Policy{Mode: string(ModeWorkspaceWrite)})

	write, err := runtime.Write(filepath.Join("nested", "file.txt"), []byte("alpha beta\nalpha\n"), true)
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if write.Path != filepath.Join("nested", "file.txt") || write.Bytes != len("alpha beta\nalpha\n") {
		t.Fatalf("unexpected write result: %+v", write)
	}

	edit, err := runtime.Edit(filepath.Join("nested", "file.txt"), "beta", "gamma", 1)
	if err != nil {
		t.Fatalf("Edit() error = %v", err)
	}
	if edit.Replacements != 1 || edit.Path != filepath.Join("nested", "file.txt") {
		t.Fatalf("unexpected edit result: %+v", edit)
	}
	data, _, err := runtime.Read(filepath.Join("nested", "file.txt"), 0)
	if err != nil {
		t.Fatalf("Read() after edit error = %v", err)
	}
	if string(data) != "alpha gamma\nalpha\n" {
		t.Fatalf("edited file = %q", data)
	}

	before := append([]byte(nil), data...)
	_, err = runtime.Edit(filepath.Join("nested", "file.txt"), "alpha", "changed", 1)
	assertRuntimeErrorKind(t, err, ErrInvalidEdit)
	after, _, readErr := runtime.Read(filepath.Join("nested", "file.txt"), 0)
	if readErr != nil {
		t.Fatalf("Read() after failed edit error = %v", readErr)
	}
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("failed edit modified file: before=%q after=%q", before, after)
	}
}

func TestWorkspaceWriteCanDeleteSingleFileSafely(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "remove.txt"), []byte("remove me"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "dir"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	runtime := mustNative(t, root, model.Policy{Mode: string(ModeWorkspaceWrite)})

	deleted, err := runtime.Delete("remove.txt")
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if deleted.Path != "remove.txt" {
		t.Fatalf("Delete().Path = %q, want remove.txt", deleted.Path)
	}
	if _, err := os.Stat(filepath.Join(root, "remove.txt")); !os.IsNotExist(err) {
		t.Fatalf("deleted file still exists or unexpected stat error: %v", err)
	}

	_, err = runtime.Delete("missing.txt")
	assertRuntimeErrorKind(t, err, ErrNotFound)
	_, err = runtime.Delete("dir")
	assertRuntimeErrorKind(t, err, ErrInvalidPath)
	_, err = runtime.Delete(".")
	assertRuntimeErrorKind(t, err, ErrInvalidPath)
	_, err = runtime.Delete(filepath.Join(".git", "config"))
	assertRuntimeErrorKind(t, err, ErrPathBlocked)
	_, err = runtime.Delete(filepath.Join(".ai-dev-manager", "runtime", "state.json"))
	assertRuntimeErrorKind(t, err, ErrPathBlocked)
}

func TestExactEditAdaptsConsistentLFAndCRLF(t *testing.T) {
	tests := []struct {
		name    string
		before  string
		oldText string
		newText string
		want    string
	}{
		{
			name:    "CRLF file accepts LF edit and preserves CRLF",
			before:  "alpha\r\nbeta\r\ngamma\r\n",
			oldText: "alpha\nbeta\n",
			newText: "alpha\nchanged\n",
			want:    "alpha\r\nchanged\r\ngamma\r\n",
		},
		{
			name:    "LF file accepts CRLF edit and preserves LF",
			before:  "alpha\nbeta\ngamma\n",
			oldText: "alpha\r\nbeta\r\n",
			newText: "alpha\r\nchanged\r\n",
			want:    "alpha\nchanged\ngamma\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "edit.txt")
			if err := os.WriteFile(path, []byte(tt.before), 0o644); err != nil {
				t.Fatalf("WriteFile() error = %v", err)
			}
			runtime := mustNative(t, root, model.Policy{Mode: string(ModeWorkspaceWrite)})

			edit, err := runtime.Edit("edit.txt", tt.oldText, tt.newText, 1)
			if err != nil {
				t.Fatalf("Edit() error = %v", err)
			}
			if edit.Replacements != 1 {
				t.Fatalf("Edit().Replacements = %d, want 1", edit.Replacements)
			}
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("ReadFile() error = %v", err)
			}
			if string(got) != tt.want {
				t.Fatalf("edited file = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExactEditNewlineFallbackRemainsStrict(t *testing.T) {
	tests := []struct {
		name    string
		before  string
		oldText string
		newText string
	}{
		{
			name:    "adapted replacement count must match",
			before:  "alpha\r\nbeta\r\nalpha\r\nbeta\r\n",
			oldText: "alpha\nbeta\n",
			newText: "changed\n",
		},
		{
			name:    "mixed target newlines are not normalized",
			before:  "alpha\r\nbeta\ngamma\r\n",
			oldText: "alpha\nbeta\n",
			newText: "changed\n",
		},
		{
			name:    "whitespace differences stay exact",
			before:  "alpha\r\n  beta\r\n",
			oldText: "alpha\n beta\n",
			newText: "changed\n",
		},
		{
			name:    "mixed replacement newlines are rejected",
			before:  "alpha\r\nbeta\r\n",
			oldText: "alpha\nbeta\n",
			newText: "changed\nnext\r\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "edit.txt")
			if err := os.WriteFile(path, []byte(tt.before), 0o644); err != nil {
				t.Fatalf("WriteFile() error = %v", err)
			}
			runtime := mustNative(t, root, model.Policy{Mode: string(ModeWorkspaceWrite)})

			_, err := runtime.Edit("edit.txt", tt.oldText, tt.newText, 1)
			assertRuntimeErrorKind(t, err, ErrInvalidEdit)
			got, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Fatalf("ReadFile() error = %v", readErr)
			}
			if string(got) != tt.before {
				t.Fatalf("failed edit modified file: got=%q want=%q", got, tt.before)
			}
		})
	}
}

func TestFileAndSearchLimitsAreEnforced(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("match\nmatch\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	runtime := mustNative(t, root, model.Policy{Mode: string(ModeReadOnly)})

	_, _, err := runtime.Read("a.txt", hardReadBytes+1)
	assertRuntimeErrorKind(t, err, ErrLimitExceeded)
	_, err = runtime.Tree(".", TreeOptions{MaxDepth: hardTreeDepth + 1})
	assertRuntimeErrorKind(t, err, ErrLimitExceeded)
	_, err = runtime.Search(SearchOptions{Path: ".", Query: "match", MaxMatches: 1})
	assertRuntimeErrorKind(t, err, ErrLimitExceeded)
}

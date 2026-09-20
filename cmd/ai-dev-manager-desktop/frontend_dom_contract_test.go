package main

import (
	"io/fs"
	"regexp"
	"strings"
	"testing"
)

func TestFrontendElementIDsExistInIndex(t *testing.T) {
	assets, err := frontendAssets()
	if err != nil {
		t.Fatal(err)
	}
	index, err := fs.ReadFile(assets, "index.html")
	if err != nil {
		t.Fatal(err)
	}

	htmlIDPattern := regexp.MustCompile("\\bid=[\"']([^\"']+)[\"']")
	htmlIDs := make(map[string]struct{})
	for _, match := range htmlIDPattern.FindAllSubmatch(index, -1) {
		id := string(match[1])
		if _, exists := htmlIDs[id]; exists {
			t.Errorf("desktop index contains duplicate element id %q", id)
			continue
		}
		htmlIDs[id] = struct{}{}
	}

	jsIDPattern := regexp.MustCompile("getElementById\\(\\s*[\"']([^\"']+)[\"']\\s*\\)")
	entries, err := fs.ReadDir(assets, ".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".js") {
			continue
		}
		data, err := fs.ReadFile(assets, entry.Name())
		if err != nil {
			t.Fatal(err)
		}
		for _, match := range jsIDPattern.FindAllSubmatch(data, -1) {
			id := string(match[1])
			if _, ok := htmlIDs[id]; !ok {
				t.Errorf("%s references missing HTML element id %q", entry.Name(), id)
			}
		}
	}
}

func TestFrontendElementsObjectHasUniqueKeys(t *testing.T) {
	assets, err := frontendAssets()
	if err != nil {
		t.Fatal(err)
	}
	javascript, err := fs.ReadFile(assets, "app.js")
	if err != nil {
		t.Fatal(err)
	}

	elementKeyPattern := regexp.MustCompile("\\b([A-Za-z_$][A-Za-z0-9_$]*)\\s*:\\s*document\\.getElementById\\(")
	seen := make(map[string]struct{})
	for _, match := range elementKeyPattern.FindAllSubmatch(javascript, -1) {
		key := string(match[1])
		if _, exists := seen[key]; exists {
			t.Errorf("elements object contains duplicate key %q", key)
			continue
		}
		seen[key] = struct{}{}
	}
}

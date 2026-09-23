package main

import (
	"io/fs"
	"regexp"
	"testing"
)

func TestFrontendElementIDsExistInHTMLSurface(t *testing.T) {
	assets, err := frontendAssets()
	if err != nil {
		t.Fatal(err)
	}

	htmlIDPattern := regexp.MustCompile("\\bid=[\"']([^\"']+)[\"']")
	jsIDPattern := regexp.MustCompile("getElementById\\(\\s*[\"']([^\"']+)[\"']\\s*\\)")
	assignedIDPattern := regexp.MustCompile("\\.id\\s*=\\s*[\"']([^\"']+)[\"']")

	loadIDs := func(name string) map[string]struct{} {
		data, err := fs.ReadFile(assets, name)
		if err != nil {
			t.Fatal(err)
		}
		ids := make(map[string]struct{})
		for _, match := range htmlIDPattern.FindAllSubmatch(data, -1) {
			id := string(match[1])
			if _, exists := ids[id]; exists {
				t.Errorf("%s contains duplicate element id %q", name, id)
				continue
			}
			ids[id] = struct{}{}
		}
		return ids
	}

	surfaces := map[string]map[string]struct{}{
		"index.html":    loadIDs("index.html"),
		"web-auth.html": loadIDs("web-auth.html"),
	}

	entries, err := fs.ReadDir(assets, ".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || len(entry.Name()) < 3 || entry.Name()[len(entry.Name())-3:] != ".js" {
			continue
		}
		target := "index.html"
		if entry.Name() == "web-auth.js" {
			target = "web-auth.html"
		}
		data, err := fs.ReadFile(assets, entry.Name())
		if err != nil {
			t.Fatal(err)
		}
		dynamicIDs := make(map[string]struct{})
		for _, match := range assignedIDPattern.FindAllSubmatch(data, -1) {
			dynamicIDs[string(match[1])] = struct{}{}
		}
		for _, match := range jsIDPattern.FindAllSubmatch(data, -1) {
			id := string(match[1])
			if _, ok := surfaces[target][id]; ok {
				continue
			}
			if _, ok := dynamicIDs[id]; ok {
				continue
			}
			t.Errorf("%s references missing %s element id %q", entry.Name(), target, id)
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

package main

import (
	"io/fs"
	"regexp"
	"strings"
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


func TestWorkspacePageUsesChineseCompactContract(t *testing.T) {
	assets, err := frontendAssets()
	if err != nil {
		t.Fatal(err)
	}
	html, err := fs.ReadFile(assets, "index.html")
	if err != nil {
		t.Fatal(err)
	}
	javascript, err := fs.ReadFile(assets, "app.js")
	if err != nil {
		t.Fatal(err)
	}
	styles, err := fs.ReadFile(assets, "styles.css")
	if err != nil {
		t.Fatal(err)
	}

	htmlText := string(html)
	for _, expected := range []string{"data-management-page=\"workspaces\"", ">工作区</h2>", ">添加工作区</button>", "<span>环境</span>", "环境数量最多", "暂无工作区"} {
		if !strings.Contains(htmlText, expected) {
			t.Fatalf("workspace HTML missing localized contract %q", expected)
		}
	}
	for _, legacy := range []string{">Workspaces</h2>", ">添加 Workspace</button>", "<span>Environment</span>", "Environment 最多", "暂无 Workspace"} {
		if strings.Contains(htmlText, legacy) {
			t.Fatalf("workspace HTML still exposes mixed-language label %q", legacy)
		}
	}

	jsText := string(javascript)
	for _, expected := range []string{"workspace-item", "workspace-path", "发现项目", "查看环境", "meta.textContent = `环境 "} {
		if !strings.Contains(jsText, expected) {
			t.Fatalf("workspace renderer missing compact/localized contract %q", expected)
		}
	}
	for _, legacy := range []string{"Discover projects", "查看 Environments", "`Environments ${safeNumber"} {
		if strings.Contains(jsText, legacy) {
			t.Fatalf("workspace renderer still exposes mixed-language label %q", legacy)
		}
	}

	cssText := string(styles)
	for _, expected := range []string{"container-type: inline-size", "@container (max-width: 720px)", ".workspace-item .workspace-path", ".workspace-list-toolbar"} {
		if !strings.Contains(cssText, expected) {
			t.Fatalf("workspace CSS missing responsive contract %q", expected)
		}
	}
}

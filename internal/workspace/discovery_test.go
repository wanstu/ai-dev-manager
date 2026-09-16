package workspace

import (
	"ai-dev-manager-v2/internal/model"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func discoveryWrite(t *testing.T, root, path string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("private-content-sentinel"), 0600); err != nil {
		t.Fatal(err)
	}
}
func discoveryScan(t *testing.T, root string, r model.DiscoveryRequest) model.DiscoveryReport {
	t.Helper()
	out, err := ScanDiscovery(context.Background(), root, model.DiscoveryScope{WorkspaceID: "ws-fixture"}, r, true)
	if err != nil {
		t.Fatal(err)
	}
	return out
}
func hasReason(out model.DiscoveryReport, reason string) bool {
	for _, v := range out.StopReasons {
		if v == reason {
			return true
		}
	}
	return false
}
func TestDiscoveryCandidatesQueriesAndMetadataOnly(t *testing.T) {
	root := t.TempDir()
	for _, p := range []string{"go.mod", "p1/go.mod", "p1/go.work", "p2/package.json", "p2/nested/Cargo.toml", "p3/readme.txt", "other/p2/pyproject.toml", ".env", ".git/config", "node_modules/fake/package.json", "p1/build/fake/go.mod"} {
		discoveryWrite(t, root, p)
	}
	out := discoveryScan(t, root, model.DiscoveryRequest{})
	if out.Truncated {
		t.Fatalf("unexpected partial: %+v", out)
	}
	got := map[string]model.ProjectCandidate{}
	for _, c := range out.Candidates {
		got[c.Root] = c
	}
	for _, p := range []string{".", "p1", "p2", "p3", "p2/nested", "other/p2"} {
		if _, ok := got[p]; !ok {
			t.Fatalf("missing candidate %s: %+v", p, got)
		}
	}
	if len(got["p1"].Markers) != 2 || got["p3"].Evidence != "directory-only" {
		t.Fatal("marker coalescing or ordinary directory failed")
	}
	if len(got["."].Markers) != 2 {
		t.Fatal(".git metadata not recognized")
	}
	if out.SkippedDirectories != 3 {
		t.Fatalf("excluded=%d", out.SkippedDirectories)
	}
	data, _ := json.Marshal(out)
	if strings.Contains(string(data), "private-content-sentinel") || strings.Contains(string(data), "fake/package.json") || strings.Contains(string(data), ".git/config") {
		t.Fatalf("unexpected content/ignored traversal: %s", data)
	}
	q := discoveryScan(t, root, model.DiscoveryRequest{Query: "p2"})
	if len(q.Candidates) != 3 || q.Candidates[0].Root != "p2" || q.Candidates[0].QueryMatch != "exact_path" || q.Candidates[1].QueryMatch != "exact_name" {
		t.Fatalf("query=%+v", q.Candidates)
	}
	none := discoveryScan(t, root, model.DiscoveryRequest{Query: "no-match"})
	if len(none.Candidates) != 0 || none.Truncated {
		t.Fatal("false no-match")
	}
	sub := discoveryScan(t, root, model.DiscoveryRequest{Path: "p2"})
	if sub.ScanPath != "p2" || sub.Candidates[0].SuggestedEnvironmentRoot == "." {
		t.Fatal("subpath lost authority-relative root")
	}
	repeat := discoveryScan(t, root, model.DiscoveryRequest{})
	out.ObservedAt = time.Time{}
	repeat.ObservedAt = time.Time{}
	if !reflect.DeepEqual(out, repeat) {
		t.Fatal("unrestricted scan order changed")
	}
	empty := discoveryScan(t, t.TempDir(), model.DiscoveryRequest{})
	if empty.Truncated || len(empty.Candidates) != 0 || len(empty.Digest) != 1 || !empty.Digest[0].ChildrenComplete {
		t.Fatalf("empty=%+v", empty)
	}
}

func TestDiscoveryBudgetsAndBoundedEnumeration(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 150; i++ {
		discoveryWrite(t, root, fmt.Sprintf("p%03d-%s/go.mod", i, strings.Repeat("x", 65)))
	}
	limited := discoveryScan(t, root, model.DiscoveryRequest{MaxEntries: 5})
	if limited.VisitedEntries != 5 || limited.ReadBatches != 1 || !hasReason(limited, "entry_limit") {
		t.Fatalf("unbounded enumeration: %+v", limited)
	}
	candidates := discoveryScan(t, root, model.DiscoveryRequest{MaxCandidates: 2})
	if len(candidates.Candidates) != 2 || candidates.OmittedCandidates != 148 || !hasReason(candidates, "candidate_limit") {
		t.Fatalf("candidate bound: %+v", candidates)
	}
	digest := discoveryScan(t, root, model.DiscoveryRequest{MaxDigestEntries: 2})
	if len(digest.Digest) != 2 || digest.OmittedDigestEntries != 149 || !hasReason(digest, "digest_limit") {
		t.Fatalf("digest bound: %+v", digest)
	}
	bytes := discoveryScan(t, root, model.DiscoveryRequest{MaxOutputBytes: 4096, MaxCandidates: 200, MaxDigestEntries: 500})
	encoded, _ := json.Marshal(bytes)
	if len(encoded) > 4096 || !hasReason(bytes, "output_byte_limit") || bytes.OmittedCandidates == 0 {
		t.Fatalf("bytes=%d report=%+v", len(encoded), bytes)
	}
	deep := discoveryScan(t, root, model.DiscoveryRequest{MaxDepth: 1})
	if !hasReason(deep, "depth_limit") {
		t.Fatal("depth unbounded")
	}
	for _, d := range deep.Digest {
		if d.Path != "." && d.ChildrenComplete {
			t.Fatal("unvisited directory says complete")
		}
	}
	for _, r := range []model.DiscoveryRequest{{MaxDepth: -1}, {MaxDepth: 9}, {MaxEntries: 10001}, {MaxCandidates: 201}, {MaxDigestEntries: 501}, {MaxOutputBytes: 262145}, {MaxOutputBytes: 4095}, {Query: strings.Repeat("界", 201)}} {
		if _, err := ScanDiscovery(context.Background(), root, model.DiscoveryScope{}, r, true); err == nil {
			t.Fatalf("invalid limit accepted: %+v", r)
		}
	}
	markers := t.TempDir()
	for i := 0; i < 80; i++ {
		discoveryWrite(t, markers, fmt.Sprintf("project%d.csproj", i))
	}
	marked := discoveryScan(t, markers, model.DiscoveryRequest{})
	if marked.OmittedMarkers != 16 || len(marked.Candidates[0].Markers) != 64 || !hasReason(marked, "marker_limit") {
		t.Fatal("marker output unbounded")
	}
}

func TestDiscoveryInvalidPathsLinksAndRoot(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	discoveryWrite(t, outside, "outside-secret-project/go.mod")
	for _, path := range []string{"../outside", "a/../b", outside, "C:\\outside", "\\\\server\\share", "missing"} {
		if _, err := ScanDiscovery(context.Background(), root, model.DiscoveryScope{}, model.DiscoveryRequest{Path: path}, true); err == nil {
			t.Fatalf("accepted path %q", path)
		}
	}
	discoveryWrite(t, root, "regular.txt")
	if _, err := ScanDiscovery(context.Background(), filepath.Join(root, "regular.txt"), model.DiscoveryScope{}, model.DiscoveryRequest{}, true); err == nil {
		t.Fatal("file root accepted")
	}
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Skipf("OS symlink creation unavailable: %v", err)
	}
	out := discoveryScan(t, root, model.DiscoveryRequest{})
	data, _ := json.Marshal(out)
	if out.SkippedLinks != 1 || !hasReason(out, "links_skipped") || strings.Contains(string(data), "outside-secret-project") {
		t.Fatal("link followed")
	}
	if _, err := ScanDiscovery(context.Background(), root, model.DiscoveryScope{}, model.DiscoveryRequest{Path: "link"}, true); err == nil {
		t.Fatal("explicit link accepted")
	}
	if _, err := ScanDiscovery(context.Background(), filepath.Join(root, "link"), model.DiscoveryScope{}, model.DiscoveryRequest{}, true); err == nil {
		t.Fatal("replaced root link accepted")
	}
}

func TestDiscoveryCancellationAndDisappearingChild(t *testing.T) {
	root := t.TempDir()
	discoveryWrite(t, root, "child/go.mod")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	out, err := ScanDiscovery(ctx, root, model.DiscoveryScope{}, model.DiscoveryRequest{}, true)
	if err != nil || out.VisitedEntries != 0 || !hasReason(out, "canceled") {
		t.Fatalf("canceled=%+v err=%v", out, err)
	}
	out, err = scanDiscovery(context.Background(), root, model.DiscoveryScope{}, model.DiscoveryRequest{}, true, func(path string) {
		if path == "child" {
			if err := os.RemoveAll(filepath.Join(root, path)); err != nil {
				t.Fatal(err)
			}
		}
	})
	if err != nil || !hasReason(out, "directory_unreadable") || len(out.Diagnostics) != 1 {
		t.Fatalf("partial=%+v err=%v", out, err)
	}
	if strings.Contains(out.Diagnostics[0].Reason, root) {
		t.Fatal("absolute path leaked in diagnostic")
	}
	ctx, cancel = context.WithCancel(context.Background())
	discoveryWrite(t, root, "child/go.mod")
	out, err = scanDiscovery(ctx, root, model.DiscoveryScope{}, model.DiscoveryRequest{}, true, func(path string) {
		if path == "child" {
			cancel()
		}
	})
	if err != nil || !hasReason(out, "canceled") || out.VisitedEntries != 1 {
		t.Fatalf("mid-scan cancellation=%+v %v", out, err)
	}
}

func TestDiscoveryUnreadableRootAndChild(t *testing.T) {
	root := t.TempDir()
	discoveryWrite(t, root, "child/go.mod")
	child := filepath.Join(root, "child")
	if err := os.Chmod(child, 0); err != nil {
		t.Skipf("chmod unsupported: %v", err)
	}
	defer os.Chmod(child, 0755)
	f, err := os.Open(child)
	if err == nil {
		_, err = f.ReadDir(1)
		f.Close()
	}
	if err == nil {
		t.Skip("permission denial not enforceable for this OS/user")
	}
	out := discoveryScan(t, root, model.DiscoveryRequest{})
	if !hasReason(out, "directory_unreadable") {
		t.Fatal("child failure hidden")
	}
	if _, err := ScanDiscovery(context.Background(), child, model.DiscoveryScope{}, model.DiscoveryRequest{}, true); err == nil {
		t.Fatal("unreadable root succeeded")
	}
}

package environment

import (
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	"ai-dev-manager-v2/internal/model"
	"ai-dev-manager-v2/internal/store"
	"ai-dev-manager-v2/internal/workspace"
)

func TestWriterConcurrentIndependentServices(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	sa, sb := store.New(path), store.New(path)
	wa, wb := workspace.New(sa), workspace.New(sb)
	a, b := New(sa, wa), New(sb, wb)
	ws, err := wa.Add(t.TempDir(), "shared-state")
	if err != nil {
		t.Fatal(err)
	}
	envA, err := a.Create(ws.ID, "a", "")
	if err != nil {
		t.Fatal(err)
	}
	envB, err := b.Create(ws.ID, "b", "")
	if err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	type result struct {
		id, owner string
		err       error
	}
	results := make(chan result, 2)
	var wg sync.WaitGroup
	for _, item := range []struct {
		s         *Service
		id, owner string
	}{{a, envA.ID, "owner-a"}, {b, envB.ID, "owner-b"}} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := item.s.AcquireWriter(item.id, item.owner)
			results <- result{item.id, item.owner, err}
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	var winner result
	successes := 0
	for r := range results {
		if r.err == nil {
			successes++
			winner = r
		}
	}
	if successes != 1 {
		t.Fatalf("overlapping roots acquired %d writers", successes)
	}
	// Independent Store mutation must preserve the winner's lease.
	if err := sb.Update(func(state *model.State) error { state.GlobalMemory["checkpoint"] = "saved"; return nil }); err != nil {
		t.Fatal(err)
	}
	current, err := a.Get(winner.id)
	if err != nil || current.Writer == nil || current.Writer.Owner != winner.owner {
		t.Fatalf("lost writer %+v %v", current.Writer, err)
	}
	before := current.Writer.ID
	if _, err := b.TakeoverWriter(winner.id, "admin"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.HeartbeatWriter(winner.id, winner.owner); err == nil {
		t.Fatal("old owner renewed replaced lease")
	}
	if _, err := a.ReleaseWriter(winner.id, winner.owner, false); err == nil {
		t.Fatal("old owner released replaced lease")
	}
	current, err = b.Get(winner.id)
	if err != nil || current.Writer == nil || current.Writer.Owner != "admin" || current.Writer.ID == before {
		t.Fatalf("takeover %+v %v", current.Writer, err)
	}
	state, err := sa.Load()
	if err != nil || state.GlobalMemory["checkpoint"] != "saved" {
		t.Fatal(fmt.Errorf("memory lost during takeover: %v", err))
	}
}

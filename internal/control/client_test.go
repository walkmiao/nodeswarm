package control

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func clientHB(id string, ps ...ClientProcess) Heartbeat {
	return Heartbeat{ID: id, Client: &ClientSnapshot{Backend: "observe", Processes: ps, ObservedAt: time.Now().UTC().Format(time.RFC3339)}}
}
func TestClientBindingIdentityCapacityAndPersistence(t *testing.T) {
	dir := t.TempDir()
	r, err := NewRegistry(dir)
	if err != nil {
		t.Fatal(err)
	}
	id, tok, err := r.Enroll(EnrollInput{Name: "desktop", Kind: "client", OS: "windows"}, "127.0.0.1")
	if err != nil || !r.Authorize(id, tok) {
		t.Fatalf("enroll client: %v", err)
	}
	ps := []ClientProcess{{PID: 101, StartTime: "boot-1", Name: "Client.exe", Path: "C:/game/Client.exe"}, {PID: 102, StartTime: "boot-2", Name: "Client.exe"}}
	if _, err = r.Report(clientHB(id, ps...), "127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	_, err = r.UpdateClientBindings(id, []ClientBinding{{PID: 101, StartTime: "old", NodeID: "node-a"}}, map[string]int{"node-a": 1})
	if err == nil {
		t.Fatal("stale process identity should be rejected")
	}
	current, err := r.UpdateClientBindings(id, []ClientBinding{{PID: 101, StartTime: "boot-1", NodeID: "node-a"}}, map[string]int{"node-a": 1})
	if err != nil || current.ClientBindings[0].Name != "Client.exe" {
		t.Fatalf("binding failed: %+v %v", current, err)
	}
	_, err = r.UpdateClientBindings(id, []ClientBinding{{PID: 101, StartTime: "boot-1", NodeID: "node-a"}, {PID: 102, StartTime: "boot-2", NodeID: "node-a"}}, map[string]int{"node-a": 1})
	if err == nil {
		t.Fatal("capacity not enforced")
	}
	fresh, err := NewRegistry(dir)
	if err != nil {
		t.Fatal(err)
	}
	v, _ := fresh.Find(id)
	if v.Kind != "client" || len(v.ClientBindings) != 1 {
		t.Fatalf("role/bindings not persisted: %+v", v)
	}
	if _, err = fresh.Report(clientHB(id, ClientProcess{PID: 101, StartTime: "new", Name: "Client.exe"}), "127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	v, _ = fresh.Find(id)
	if len(v.ClientBindings) != 0 {
		t.Fatalf("PID reuse should unbind: %+v", v.ClientBindings)
	}
}
func TestClientCapacityConcurrentClaim(t *testing.T) {
	r, err := NewRegistry(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{}
	for i := 0; i < 2; i++ {
		id, _, e := r.Enroll(EnrollInput{Kind: "client", Name: "desktop"}, "127.0.0.1")
		if e != nil {
			t.Fatal(e)
		}
		if _, e = r.Report(clientHB(id, ClientProcess{PID: 123, StartTime: "start", Name: "Client.exe"}), "127.0.0.1"); e != nil {
			t.Fatal(e)
		}
		ids = append(ids, id)
	}
	var successes atomic.Int32
	var wg sync.WaitGroup
	for _, id := range ids {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			_, err := r.UpdateClientBindings(id, []ClientBinding{{PID: 123, StartTime: "start", NodeID: "x"}}, map[string]int{"x": 1})
			if err == nil {
				successes.Add(1)
			}
		}(id)
	}
	wg.Wait()
	if successes.Load() != 1 {
		t.Fatalf("expected exactly one capacity claim, got %d", successes.Load())
	}
}

func TestStaleClientInventoryRejectedAndPruneIsDurable(t *testing.T) {
	dir := t.TempDir()
	r, err := NewRegistry(dir)
	if err != nil {
		t.Fatal(err)
	}
	id, _, err := r.Enroll(EnrollInput{Kind: "client", Name: "desktop"}, "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	proc := ClientProcess{PID: 101, Name: "Client.exe", StartTime: "startup-1"}
	if _, err := r.Report(clientHB(id, proc), "127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.UpdateClientBindings(id, []ClientBinding{{PID: 101, StartTime: proc.StartTime, NodeID: "proxy"}}, map[string]int{"proxy": 1}); err != nil {
		t.Fatal(err)
	}
	// An exit/reuse reported by heartbeat must persist immediately, not merely after a later heartbeat.
	if _, err := r.Report(clientHB(id, ClientProcess{PID: 101, Name: "Client.exe", StartTime: "startup-2"}), "127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	restored, err := NewRegistry(dir)
	if err != nil {
		t.Fatal(err)
	}
	view, ok := restored.Find(id)
	if !ok || len(view.ClientBindings) != 0 {
		t.Fatalf("stale binding resurrected after restart: %+v", view.ClientBindings)
	}
	restored.mu.Lock()
	for i := range restored.agents {
		if restored.agents[i].ID == id {
			restored.agents[i].LastSeen = time.Now().Add(-3 * time.Minute).UTC().Format(time.RFC3339)
		}
	}
	restored.mu.Unlock()
	if _, err := restored.UpdateClientBindings(id, []ClientBinding{{PID: 101, StartTime: "startup-2", NodeID: "proxy"}}, map[string]int{"proxy": 1}); err == nil {
		t.Fatal("must reject outdated heartbeat")
	}
}

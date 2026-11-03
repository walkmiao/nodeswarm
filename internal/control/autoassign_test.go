package control

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestAutoAssignPreservesExistingBalancesAndHonorsQuota(t *testing.T) {
	r, err := NewRegistry(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	id, _, err := r.Enroll(EnrollInput{Kind: "client", Name: "desk"}, "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	procs := make([]ClientProcess, 5)
	for i := range procs {
		procs[i] = ClientProcess{PID: 100 + i, StartTime: fmt.Sprintf("boot-%d", i), Name: "Client.exe"}
	}
	if _, err = r.Report(clientHB(id, procs...), "127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	v, err := r.SetClientMaxProxyPIDs(id, 4, 0)
	if err != nil {
		t.Fatal(err)
	}
	b := []ClientBinding{{PID: 100, StartTime: "boot-0", NodeID: "node-b"}}
	v, err = r.UpdateClientBindingsCAS(id, b, map[string]int{"node-a": 2, "node-b": 2}, v.Revision)
	if err != nil {
		t.Fatal(err)
	}
	v, added, err := r.AutoAssignClient(id, map[string]int{"node-a": 2, "node-b": 2}, v.Revision)
	if err != nil || added != 3 || len(v.ClientBindings) != 4 {
		t.Fatalf("unexpected auto plan %+v, added=%d err=%v", v.ClientBindings, added, err)
	}
	if v.ClientBindings[0].NodeID != "node-b" || v.ClientBindings[0].PID != 100 {
		t.Fatal("existing binding was moved")
	}
	counter := map[string]int{}
	for _, x := range v.ClientBindings {
		counter[x.NodeID]++
	}
	if counter["node-a"] != 2 || counter["node-b"] != 2 {
		t.Fatalf("not balanced: %+v", counter)
	}
	same, added, err := r.AutoAssignClient(id, map[string]int{"node-a": 2, "node-b": 2}, v.Revision)
	if err != nil || added != 0 || same.Revision != v.Revision {
		t.Fatalf("repeat changed state: %+v added=%d err=%v", same, added, err)
	}
	if _, _, err = r.AutoAssignClient(id, map[string]int{"node-a": 2}, v.Revision-1); err == nil {
		t.Fatal("stale revision accepted")
	}
	restored, err := NewRegistry(r.path[:len(r.path)-len("/agents.json")])
	if err != nil {
		t.Fatal(err)
	}
	fresh, _ := restored.Find(id)
	if len(fresh.ClientBindings) != 4 {
		t.Fatal("auto bindings not persisted")
	}
}
func TestAutoAssignConcurrentCapacityCannotOverbook(t *testing.T) {
	r, err := NewRegistry(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{}
	for i := 0; i < 8; i++ {
		id, _, err := r.Enroll(EnrollInput{Kind: "client", Name: fmt.Sprint(i)}, "127.0.0.1")
		if err != nil {
			t.Fatal(err)
		}
		if _, err = r.Report(clientHB(id, ClientProcess{PID: 100 + i, StartTime: "start", Name: "Client.exe"}), "127.0.0.1"); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	var wg sync.WaitGroup
	for _, id := range ids {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			if _, _, err := r.AutoAssignClient(id, map[string]int{"one": 2}, -1); err != nil {
				t.Error(err)
			}
		}(id)
	}
	wg.Wait()
	total := 0
	for _, v := range r.List() {
		total += len(v.ClientBindings)
	}
	if total != 2 {
		t.Fatalf("capacity overbooked or underassigned: %d", total)
	}
}
func TestAutoAssignRejectsStaleInventoryAndHonorsLocalLimit(t *testing.T) {
	r, err := NewRegistry(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	id, _, _ := r.Enroll(EnrollInput{Kind: "client", Name: "desk"}, "")
	p := []ClientProcess{{PID: 5, Name: "Client.exe", StartTime: "5"}, {PID: 6, Name: "Client.exe", StartTime: "6"}}
	if _, err = r.Report(Heartbeat{ID: id, Client: &ClientSnapshot{Processes: p, MaxProxyPIDs: 1, ObservedAt: time.Now().UTC().Format(time.RFC3339)}}, ""); err != nil {
		t.Fatal(err)
	}
	v, added, err := r.AutoAssignClient(id, map[string]int{"A": 6}, 0)
	if err != nil || added != 1 || len(v.ClientBindings) != 1 {
		t.Fatalf("hard cap not observed: %+v %d %v", v.ClientBindings, added, err)
	}
	r.mu.Lock()
	r.agents[0].LastSeen = time.Now().Add(-2 * time.Minute).UTC().Format(time.RFC3339)
	r.mu.Unlock()
	if _, _, err = r.AutoAssignClient(id, map[string]int{"A": 6}, -1); err == nil {
		t.Fatal("offline auto assignment accepted")
	}
}

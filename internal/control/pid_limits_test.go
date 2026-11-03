package control

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPerMachinePIDLimitEnforcedAndPersisted(t *testing.T) {
	dir := t.TempDir()
	r, err := NewRegistry(dir)
	if err != nil {
		t.Fatal(err)
	}
	id, _, err := r.Enroll(EnrollInput{Kind: "client", Name: "gaming-pc"}, "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	other, _, err := r.Enroll(EnrollInput{Kind: "client", Name: "second-pc"}, "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	ps := []ClientProcess{
		{PID: 101, StartTime: "start-101", Name: "Client.exe"},
		{PID: 102, StartTime: "start-102", Name: "Client.exe"},
		{PID: 103, StartTime: "start-103", Name: "Client.exe"},
	}
	for _, clientID := range []string{id, other} {
		if _, err = r.Report(clientHB(clientID, ps...), "127.0.0.1"); err != nil {
			t.Fatal(err)
		}
	}
	if view, ok := r.Find(id); !ok || view.MaxProxyPIDs != DefaultMaxProxyPIDs {
		t.Fatalf("expected default quota: %+v", view)
	}
	if _, err = r.SetClientMaxProxyPIDs(id, 0, 0); err == nil {
		t.Fatal("zero quota unexpectedly accepted")
	}
	if _, err = r.SetClientMaxProxyPIDs(id, 129, 0); err == nil {
		t.Fatal("over hard max unexpectedly accepted")
	}
	view, err := r.SetClientMaxProxyPIDs(id, 2, 0)
	if err != nil || view.MaxProxyPIDs != 2 {
		t.Fatalf("set quota failed: %+v %v", view, err)
	}
	if _, err = r.SetClientMaxProxyPIDs(id, 3, view.Revision-1); err == nil {
		t.Fatal("stale CAS accepted")
	}
	bindings := []ClientBinding{{PID: 101, StartTime: "start-101", NodeID: "node-a"}, {PID: 102, StartTime: "start-102", NodeID: "node-a"}, {PID: 103, StartTime: "start-103", NodeID: "node-a"}}
	caps := map[string]int{"node-a": 10}
	if _, err = r.UpdateClientBindingsCAS(id, bindings, caps, view.Revision); err == nil || !strings.Contains(err.Error(), "最多允许 2") {
		t.Fatalf("over-quota write accepted: %v", err)
	}
	view, err = r.UpdateClientBindingsCAS(id, bindings[:2], caps, view.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.UpdateClientBindingsCAS(id, bindings[:1], caps, view.Revision-1); err == nil {
		t.Fatal("stale binding revision accepted")
	}
	if _, err = r.SetClientMaxProxyPIDs(id, 1, view.Revision); err == nil {
		t.Fatal("quota was reduced with live assignments")
	}
	before := view.Revision
	if view, err = r.SetClientMaxProxyPIDs(id, 2, view.Revision); err != nil || view.Revision != before {
		t.Fatalf("idempotent quota update changed revision: %+v %v", view, err)
	}
	view, err = r.UpdateClientBindingsCAS(id, bindings[:1], caps, view.Revision)
	if err != nil {
		t.Fatal(err)
	}
	view, err = r.SetClientMaxProxyPIDs(id, 1, view.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.UpdateClientBindings(id, bindings[:2], caps); err == nil {
		t.Fatal("binding quota exceeded after shrink")
	}
	fresh, err := NewRegistry(dir)
	if err != nil {
		t.Fatal(err)
	}
	restored, ok := fresh.Find(id)
	if !ok || restored.MaxProxyPIDs != 1 || len(restored.ClientBindings) != 1 {
		t.Fatalf("quota not persisted: %+v", restored)
	}
	otherv, ok := fresh.Find(other)
	if !ok || otherv.MaxProxyPIDs != DefaultMaxProxyPIDs {
		t.Fatalf("quota leaked across machines: %+v", otherv)
	}
}

func TestClientPIDQuotaBlocksEvenWithSufficientNodeCapacity(t *testing.T) {
	r, err := NewRegistry(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	id, _, err := r.Enroll(EnrollInput{Kind: "client", Name: "game-pc"}, "")
	if err != nil {
		t.Fatal(err)
	}
	ps := []ClientProcess{{PID: 101, StartTime: "t101", Name: "game.exe"}, {PID: 102, StartTime: "t102", Name: "game.exe"}}
	if _, err = r.Report(Heartbeat{ID: id, Client: &ClientSnapshot{ObservedAt: time.Now().UTC().Format(time.RFC3339), Processes: ps}}, ""); err != nil {
		t.Fatal(err)
	}
	view, err := r.SetClientMaxProxyPIDs(id, 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	bs := []ClientBinding{{PID: 101, StartTime: "t101", NodeID: "n"}, {PID: 102, StartTime: "t102", NodeID: "n"}}
	if _, err = r.UpdateClientBindings(id, bs, map[string]int{"n": 128}); err == nil {
		t.Fatal("node capacity bypassed workstation quota")
	}
	if len(view.ClientBindings) != 0 {
		t.Fatal("transaction partially applied")
	}
}

// v1.2 records had no maxProxyPids. Existing 9+ legitimate reservations are
// preserved during upgrade instead of silently discarding the excess.
func TestMigrationFromV12PreservesOldClientReservations(t *testing.T) {
	dir := t.TempDir()
	r, err := NewRegistry(dir)
	if err != nil {
		t.Fatal(err)
	}
	id, _, err := r.Enroll(EnrollInput{Name: "legacy-machine", Kind: "client"}, "")
	if err != nil {
		t.Fatal(err)
	}
	ps := make([]ClientProcess, 9)
	bs := make([]ClientBinding, 9)
	for i := 0; i < 9; i++ {
		ps[i] = ClientProcess{PID: 100 + i, StartTime: fmt.Sprint(i), Name: "Client.exe"}
		bs[i] = ClientBinding{PID: 100 + i, StartTime: fmt.Sprint(i), NodeID: "node"}
	}
	if _, err = r.Report(clientHB(id, ps...), ""); err != nil {
		t.Fatal(err)
	}
	if _, err = r.SetClientMaxProxyPIDs(id, 12, -1); err != nil {
		t.Fatal(err)
	}
	if _, err = r.UpdateClientBindings(id, bs, map[string]int{"node": 12}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "agents.json")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var data map[string]any
	if err = json.Unmarshal(b, &data); err != nil {
		t.Fatal(err)
	}
	a := data["agents"].([]any)[0].(map[string]any)
	delete(a, "maxProxyPids")
	b, err = json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, b, 0600); err != nil {
		t.Fatal(err)
	}
	migrated, err := NewRegistry(dir)
	if err != nil {
		t.Fatal(err)
	}
	restored, ok := migrated.Find(id)
	if !ok || restored.MaxProxyPIDs != 9 || len(restored.ClientBindings) != 9 {
		t.Fatalf("v1.2 reservations lost: %+v", restored)
	}
}

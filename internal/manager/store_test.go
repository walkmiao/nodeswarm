package manager

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStoreEncryptAndPlan(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	node, err := s.Create(NodeInput{Name: "test-1", Host: "127.0.0.1", Port: 1080, Username: "bob", Password: "super-secret", Capacity: 1})
	if err != nil {
		t.Fatal(err)
	}
	if node.Password != "" || !node.HasPassword {
		t.Fatal("API must not expose secret")
	}
	raw, err := os.ReadFile(filepath.Join(dir, "nodes.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "super-secret") {
		t.Fatal("secret in plain text")
	}
	s2, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	internal, ok := s2.NodeByID(node.ID)
	if !ok || internal.Password != "super-secret" {
		t.Fatal("failed to decrypt saved credential")
	}
	a, err := s2.Assign(Process{PID: 100, StartTime: "start1", Name: "Client.exe"}, node.ID, false)
	if err != nil || a.Status != "planned_only" {
		t.Fatal("invalid planning", err)
	}
	if _, err = s2.Assign(Process{PID: 101, StartTime: "start2", Name: "Client.exe"}, node.ID, false); err == nil {
		t.Fatal("should fail at capacity 1")
	}
	_, assigned, _ := s2.Snapshot()
	if len(assigned) != 1 {
		t.Fatal("not assigned")
	}
	s2.PruneProcesses([]Process{{PID: 100, StartTime: "new-start", Name: "Client.exe"}})
	_, assigned, _ = s2.Snapshot()
	if len(assigned) != 0 {
		t.Fatal("PID reuse must invalidate old assignment")
	}
	s3, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	_, assigned, _ = s3.Snapshot()
	if len(assigned) != 0 {
		t.Fatal("ephemeral assignments should not persist")
	}
}
func TestImportAtomic(t *testing.T) {
	s, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	inp := NodeInput{Name: "one", Host: "localhost", Port: 1080, Capacity: 2}
	count, err := s.Import([]NodeInput{inp, inp})
	if err != nil || count != 1 {
		t.Fatalf("import dedup: %d %v", count, err)
	}
	count, err = s.Import([]NodeInput{{Name: "invalid", Host: "bad host", Port: 0, Capacity: 1}})
	if err == nil || count != 0 {
		t.Fatal("expected validation rejection")
	}
	nodes, _, _ := s.Snapshot()
	if len(nodes) != 1 {
		t.Fatal("invalid import must not mutate")
	}
}

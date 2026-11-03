package manager

import (
	"strings"
	"testing"
	"time"
)

func TestParseImport(t *testing.T) {
	input := "proxy.example.com|1080|alice|secret|2026-12-31 23:59:59\n127.0.0.1|1081|||"
	nodes, err := ParseImport(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 2 || nodes[0].Capacity != 1 || nodes[0].Password != "secret" {
		t.Fatalf("bad parse: %#v", nodes)
	}
	if !strings.HasPrefix(nodes[0].ExpiresAt, "2026-12-31T") {
		t.Fatalf("invalid ISO date: %s", nodes[0].ExpiresAt)
	}
	_, err = ParseImport("127.0.0.1|bad|u|p|")
	if err == nil {
		t.Fatal("expected port validation error")
	}
}
func TestCapacityAndExpiry(t *testing.T) {
	now := time.Now()
	n := Node{ID: "1", Capacity: 2, Enabled: true}
	if !NodeAvailable(n, 1, now) || NodeAvailable(n, 2, now) {
		t.Fatal("bad capacity")
	}
	n.ExpiresAt = now.Add(-time.Second).Format(time.RFC3339)
	if NodeAvailable(n, 0, now) {
		t.Fatal("expired should not be available")
	}
}
func TestChooseNodeBalances(t *testing.T) {
	now := time.Now()
	nodes := []Node{{ID: "a", Capacity: 2, Enabled: true}, {ID: "b", Capacity: 2, Enabled: true}}
	n, ok := ChooseNode(nodes, nil, now)
	if !ok || n.ID != "a" {
		t.Fatal("expected deterministic first node")
	}
	assignments := []Assignment{{NodeID: "a"}}
	n, ok = ChooseNode(nodes, assignments, now)
	if !ok || n.ID != "b" {
		t.Fatal("expected lesser used node")
	}
}

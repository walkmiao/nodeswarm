package client

import (
	"strings"
	"testing"

	"github.com/walkmiao/nodeswarm/internal/control"
	"github.com/walkmiao/nodeswarm/internal/manager"
)

func TestWindowsClientLocalHardQuotaRejectsOverLimit(t *testing.T) {
	c, err := New(Config{DataDir: t.TempDir(), Engine: "observe", Targets: "Client.exe", PIDHardLimit: 1})
	if err != nil {
		t.Fatal(err)
	}
	c.processList = func() ([]manager.Process, error) {
		return []manager.Process{{PID: 41, Name: "Client.exe", StartTime: "a"}, {PID: 42, Name: "Client.exe", StartTime: "b"}}, nil
	}
	p := &control.ClientPolicy{Revision: 5, MaxProxyPIDs: 8, Bindings: []control.ClientBinding{{PID: 41, StartTime: "a", NodeID: "node-a"}, {PID: 42, StartTime: "b", NodeID: "node-a"}}}
	if err := c.Apply(p); err != nil {
		t.Fatal(err)
	}
	snap := c.Observe()
	if snap.MaxProxyPIDs != 1 || !strings.Contains(snap.Error, "pid_limit_exceeded") || len(snap.Routes) != 2 || snap.Routes[0].Status != "pid_limit_exceeded" {
		t.Fatalf("local hard ceiling not enforced: %+v", snap)
	}
	p.Revision++
	p.Bindings = p.Bindings[:1]
	if err := c.Apply(p); err != nil {
		t.Fatal(err)
	}
	snap = c.Observe()
	if snap.MaxProxyPIDs != 1 || snap.Error != "" || len(snap.Routes) != 1 || snap.Routes[0].Status != "planned_only" {
		t.Fatalf("valid smaller policy not recovered: %+v", snap)
	}
	p.MaxProxyPIDs = 0 // old Controller fallback: default 8, but hard cap still wins
	p.Bindings = append(p.Bindings, control.ClientBinding{PID: 42, StartTime: "b", NodeID: "node-a"})
	if err := c.Apply(p); err != nil {
		t.Fatal(err)
	}
	if c.Observe().Routes[0].Status != "pid_limit_exceeded" {
		t.Fatal("legacy policy bypassed local cap")
	}
}
func TestInvalidPIDHardQuotaRejectedAtStartup(t *testing.T) {
	for _, n := range []int{-1, 129} {
		if _, err := New(Config{DataDir: t.TempDir(), PIDHardLimit: n}); err == nil {
			t.Fatalf("accepted invalid cap %d", n)
		}
	}
}

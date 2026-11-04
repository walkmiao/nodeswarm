package client

import (
	"errors"
	"strings"
	"testing"

	"github.com/walkmiao/nodeswarm/internal/control"
	"github.com/walkmiao/nodeswarm/internal/manager"
)

// A heartbeat can observe a PID and then receive a policy several seconds
// later. A newly activated proxy must recheck process identity before it can
// use a cached process-name (or PID) binding.
func TestApplyChangedPolicyRejectsReusedPIDWithoutCacheExpiry(t *testing.T) {
	c, err := New(Config{DataDir: t.TempDir(), Engine: "proxybridge", Targets: "Client.exe"})
	if err != nil {
		t.Fatal(err)
	}
	first := true
	calls := 0
	c.processList = func() ([]manager.Process, error) {
		calls++
		started := "old-start"
		if !first {
			started = "reused-start"
		}
		return []manager.Process{{PID: 42, Name: "Client.exe", StartTime: started, Path: "C:\\Game\\Client.exe"}}, nil
	}
	c.Observe() // Populate a fresh cache with the old process lifetime.
	first = false
	policy := &control.ClientPolicy{Revision: 10, MaxProxyPIDs: 8,
		Bindings:  []control.ClientBinding{{PID: 42, Name: "Client.exe", StartTime: "old-start", NodeID: "node-a"}},
		Endpoints: []control.ClientEndpoint{{NodeID: "node-a", Host: "203.0.113.9", Port: 1080, Username: "u", Password: "pw"}}}
	if err := c.Apply(policy); err != nil {
		t.Fatal(err)
	}
	snap := c.Observe()
	if calls != 2 {
		t.Fatalf("new policy must force inventory refresh: scans=%d", calls)
	}
	if snap.Error == "" || !strings.Contains(snap.Error, "no longer matches") || snap.Routes[0].Status != "cannot_enforce" {
		t.Fatalf("reused PID was allowed using old cache: %+v", snap)
	}
	if c.command != nil || c.fingerprint != "" {
		t.Fatal("unsafe process-name proxy was launched")
	}
}

func TestApplyChangedPolicyFailsClosedOnFreshScanError(t *testing.T) {
	c, err := New(Config{DataDir: t.TempDir(), Engine: "pidtcp"})
	if err != nil {
		t.Fatal(err)
	}
	c.processList = func() ([]manager.Process, error) {
		return []manager.Process{{PID: 42, Name: "Client.exe", StartTime: "old-start", Path: "C:\\Game\\Client.exe"}}, nil
	}
	c.Observe()
	c.processList = func() ([]manager.Process, error) { return nil, errors.New("permission denied") }
	policy := &control.ClientPolicy{Revision: 2, MaxProxyPIDs: 8,
		Bindings: []control.ClientBinding{{PID: 42, Name: "Client.exe", StartTime: "old-start", NodeID: "node-a"}}}
	err = c.Apply(policy)
	if err == nil || !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("expected scan failure: %v", err)
	}
	snap := c.Observe()
	if snap.ScanError == "" || snap.Error == "" || c.tcp != nil {
		t.Fatalf("scanner failed open: %+v", snap)
	}
}

func TestObserveUsesCachedInventoryWhileIdle(t *testing.T) {
	c, err := New(Config{DataDir: t.TempDir(), Engine: "pidtcp"})
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	c.processList = func() ([]manager.Process, error) {
		calls++
		return nil, nil
	}
	c.Observe()
	c.Observe()
	if calls != 1 {
		t.Fatalf("normal monitoring should reuse short-lived cache: scans=%d", calls)
	}
}

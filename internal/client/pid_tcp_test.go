package client

import (
	"strings"
	"testing"

	"github.com/walkmiao/nodeswarm/internal/control"
	"github.com/walkmiao/nodeswarm/internal/manager"
)

func TestPIDTCPNeverClaimsSuccessfulRouteWithoutDriver(t *testing.T) {
	c, err := New(Config{DataDir: t.TempDir(), Engine: "pidtcp", Targets: "Client.exe", PIDHardLimit: 2})
	if err != nil {
		t.Fatal(err)
	}
	c.processList = func() ([]manager.Process, error) {
		return []manager.Process{{PID: 12, Name: "Client.exe", StartTime: "2026-11-03T00:00:01Z", Path: `C:\Game\Client.exe`}, {PID: 13, Name: "Client.exe", StartTime: "2026-11-03T00:00:02Z", Path: `C:\Game\Client.exe`}}, nil
	}
	p := &control.ClientPolicy{Revision: 2, MaxProxyPIDs: 8, Bindings: []control.ClientBinding{{PID: 12, StartTime: "2026-11-03T00:00:01Z", Name: "Client.exe", NodeID: "a"}, {PID: 13, StartTime: "2026-11-03T00:00:02Z", Name: "Client.exe", NodeID: "b"}}, Endpoints: []control.ClientEndpoint{{NodeID: "a", Host: "203.0.113.4", Port: 1080}, {NodeID: "b", Host: "198.51.100.4", Port: 1080}}}
	if err := c.Apply(p); err != nil {
		t.Fatal(err)
	}
	snap := c.Observe()
	if len(snap.Routes) != 2 {
		t.Fatal("missing routes")
	}
	for _, route := range snap.Routes {
		if route.Status == "planned_only" || route.Status == "active" || route.Status == "ok" {
			t.Fatalf("claimed successful routing without verification: %+v", route)
		}
	}
	if !strings.Contains(snap.Error, "pidtcp") && !strings.Contains(snap.Error, "experimental") {
		t.Fatalf("missing experimental limitation: %+v", snap)
	}
	p.Bindings[1].StartTime = "expired"
	if err := c.Apply(p); err != nil {
		t.Fatal(err)
	}
	snap = c.Observe()
	if snap.Routes[0].Status != "cannot_enforce" || !strings.Contains(snap.Error, "stale") {
		t.Fatalf("stale PID not rejected: %+v", snap)
	}
	c.Stop()
}

func TestPIDTCPRejectsQuotaBeforeStartingDriver(t *testing.T) {
	c, err := New(Config{DataDir: t.TempDir(), Engine: "pidtcp", PIDHardLimit: 1})
	if err != nil {
		t.Fatal(err)
	}
	c.processList = func() ([]manager.Process, error) {
		return []manager.Process{{PID: 12, Name: "Client.exe", StartTime: "a"}, {PID: 13, Name: "Client.exe", StartTime: "b"}}, nil
	}
	p := &control.ClientPolicy{MaxProxyPIDs: 16, Bindings: []control.ClientBinding{{PID: 12, StartTime: "a", NodeID: "a"}, {PID: 13, StartTime: "b", NodeID: "b"}}}
	if err := c.Apply(p); err != nil {
		t.Fatal(err)
	}
	if s := c.Observe(); s.Routes[0].Status != "pid_limit_exceeded" || s.MaxProxyPIDs != 1 {
		t.Fatalf("PID cap bypassed: %+v", s)
	}
}

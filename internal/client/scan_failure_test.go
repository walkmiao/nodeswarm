package client

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/walkmiao/nodeswarm/internal/control"
	"github.com/walkmiao/nodeswarm/internal/manager"
)

func TestScanErrorInvalidatesReportedRoutes(t *testing.T) {
	c, err := New(Config{DataDir: t.TempDir(), Engine: "observe"})
	if err != nil {
		t.Fatal(err)
	}
	c.processList = func() ([]manager.Process, error) {
		return []manager.Process{{PID: 123, Name: "Client.exe", StartTime: "stamp"}}, nil
	}
	policy := &control.ClientPolicy{Revision: 5, MaxProxyPIDs: 8, Bindings: []control.ClientBinding{{PID: 123, StartTime: "stamp", NodeID: "a"}}}
	if err := c.Apply(policy); err != nil {
		t.Fatal(err)
	}
	if c.Observe().Routes[0].Status != "planned_only" {
		t.Fatal("expected planned route")
	}
	c.processList = func() ([]manager.Process, error) { return nil, errors.New("access denied") }
	c.lastScan = c.lastScan.Add(-20 * time.Second)
	snap := c.Observe()
	if !strings.Contains(snap.ScanError, "access denied") || snap.Routes[0].Status != "scan_unavailable" || c.fingerprint != "" {
		t.Fatalf("failed scan did not invalidate route: %+v", snap)
	}
}

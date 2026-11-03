package pidtcp

import (
	"strings"
	"testing"

	"github.com/walkmiao/nodeswarm/internal/control"
	"github.com/walkmiao/nodeswarm/internal/manager"
)

func examplePolicy() (control.ClientPolicy, []manager.Process) {
	processes := []manager.Process{{PID: 101, Name: "Client.exe", StartTime: "2026-11-03T00:00:01Z", Path: `C:\Game\Client.exe`}, {PID: 102, Name: "Client.exe", StartTime: "2026-11-03T00:00:02Z", Path: `C:\Game\Client.exe`}}
	return control.ClientPolicy{MaxProxyPIDs: 8, Bindings: []control.ClientBinding{{PID: 101, Name: "Client.exe", StartTime: processes[0].StartTime, NodeID: "a"}, {PID: 102, Name: "Client.exe", StartTime: processes[1].StartTime, NodeID: "b"}}, Endpoints: []control.ClientEndpoint{{NodeID: "a", Host: "203.0.113.9", Port: 1080, Username: "u1", Password: "pass1"}, {NodeID: "b", Host: "198.51.100.4", Port: 1080, Username: "u2", Password: "pass2"}}}, processes
}
func TestCompileDistinctSameNamePIDs(t *testing.T) {
	p, processes := examplePolicy()
	got, err := Compile(p, processes, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].PID != 101 || got[1].PID != 102 || got[0].Endpoint.NodeID == got[1].Endpoint.NodeID {
		t.Fatalf("PID routes not isolated: %+v", got)
	}
}
func TestCompilePIDQuotaAndLifetime(t *testing.T) {
	p, processes := examplePolicy()
	if _, err := Compile(p, processes, 1); err == nil || !strings.Contains(err.Error(), "quota") {
		t.Fatalf("local quota ignored: %v", err)
	}
	p.Bindings[1].StartTime = "reused"
	if _, err := Compile(p, processes, 0); err == nil || !strings.Contains(err.Error(), "stale") {
		t.Fatalf("PID reuse accepted: %v", err)
	}
	p, processes = examplePolicy()
	p.Bindings[1].PID = 101
	if _, err := Compile(p, processes, 0); err == nil {
		t.Fatal("duplicate PID accepted")
	}
	p, processes = examplePolicy()
	p.Bindings[1].NodeID = "nonexistent"
	if _, err := Compile(p, processes, 0); err == nil {
		t.Fatal("missing endpoint accepted")
	}
	p, processes = examplePolicy()
	p.Endpoints[0].Host = "127.0.0.1"
	if _, err := Compile(p, processes, 0); err == nil {
		t.Fatal("loopback endpoint accepted")
	}
}

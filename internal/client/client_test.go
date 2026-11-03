package client

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/walkmiao/nodeswarm/internal/control"
	"github.com/walkmiao/nodeswarm/internal/manager"
)

func testPolicy() control.ClientPolicy {
	return control.ClientPolicy{Revision: 3,
		Bindings:  []control.ClientBinding{{PID: 12, StartTime: "t1", Name: "Client.exe", NodeID: "a"}},
		Endpoints: []control.ClientEndpoint{{NodeID: "a", Host: "203.0.113.7", Port: 1080, Username: "u", Password: "secret-123"}}}
}
func TestBuildProfileSingleNameAllAssigned(t *testing.T) {
	p := testPolicy()
	p.Bindings = append(p.Bindings, control.ClientBinding{PID: 13, Name: "Client.exe", StartTime: "t2", NodeID: "a"})
	all := []manager.Process{{PID: 12, Name: "Client.exe", StartTime: "t1"}, {PID: 13, Name: "Client.exe", StartTime: "t2"}}
	raw, err := BuildProfile(p, all)
	if err != nil {
		t.Fatal(err)
	}
	var profile map[string]any
	if err = json.Unmarshal(raw, &profile); err != nil {
		t.Fatal(err)
	}
	rules := profile["ProxyRules"].([]any)
	if len(rules) != 1 || rules[0].(map[string]any)["ProcessName"] != "client.exe" {
		t.Fatalf("want exactly one by-name rule: %s", raw)
	}
	if !strings.Contains(string(raw), "secret-123") {
		t.Fatal("credential should be in private engine profile")
	}
}
func TestBuildProfileSafetyConflicts(t *testing.T) {
	base := testPolicy()
	t.Run("reject unassigned same-name PID", func(t *testing.T) {
		_, err := BuildProfile(base, []manager.Process{{PID: 12, Name: "Client.exe", StartTime: "t1"}, {PID: 13, Name: "Client.exe", StartTime: "t2"}})
		if err == nil || !strings.Contains(err.Error(), "unsafe_name_collision") {
			t.Fatalf("want collision rejection: %v", err)
		}
	})
	t.Run("reject separate nodes for same name", func(t *testing.T) {
		p := base
		p.Bindings = append(p.Bindings, control.ClientBinding{PID: 13, Name: "Client.exe", StartTime: "t2", NodeID: "b"})
		p.Endpoints = append(p.Endpoints, control.ClientEndpoint{NodeID: "b", Host: "203.0.113.8", Port: 1080})
		_, err := BuildProfile(p, []manager.Process{{PID: 12, Name: "Client.exe", StartTime: "t1"}, {PID: 13, Name: "Client.exe", StartTime: "t2"}})
		if err == nil || !strings.Contains(err.Error(), "unsupported_pid_isolation") {
			t.Fatalf("want PID isolation rejection: %v", err)
		}
	})
	t.Run("reject PID reuse", func(t *testing.T) {
		_, err := BuildProfile(base, []manager.Process{{PID: 12, Name: "Client.exe", StartTime: "new-start"}})
		if err == nil || !strings.Contains(err.Error(), "no longer matches") {
			t.Fatalf("want stale PID rejection: %v", err)
		}
	})
	t.Run("reject missing node", func(t *testing.T) {
		p := base
		p.Endpoints = nil
		if _, err := BuildProfile(p, []manager.Process{{PID: 12, Name: "Client.exe", StartTime: "t1"}}); err == nil {
			t.Fatal("missing node accepted")
		}
	})
}
func TestObserveAndPlanNeverPretendsToRoute(t *testing.T) {
	c, err := New(Config{DataDir: t.TempDir(), Targets: "Client.exe;Other.exe", Engine: "observe"})
	if err != nil {
		t.Fatal(err)
	}
	c.processList = func() ([]manager.Process, error) {
		return []manager.Process{{PID: 5, Name: "Client.exe", StartTime: "t1"}, {PID: 6, Name: "Other.exe", StartTime: "t2"}, {PID: 7, Name: "unrelated.exe", StartTime: "t3"}}, nil
	}
	snapshot := c.Observe()
	if len(snapshot.Processes) != 2 {
		t.Fatalf("only target processes should be reported: %+v", snapshot)
	}
	p := testPolicy()
	p.Bindings[0].PID = 5
	if err = c.Apply(&p); err != nil {
		t.Fatal(err)
	}
	snapshot = c.Observe()
	if len(snapshot.Routes) != 1 || snapshot.Routes[0].Status != "planned_only" {
		t.Fatalf("should not claim enforcement: %+v", snapshot)
	}
	if snapshot.Revision != p.Revision {
		t.Fatalf("policy revision not acknowledged: %+v", snapshot)
	}
}

package ops

import (
	"github.com/walkmiao/nodeswarm/internal/control"
	"github.com/walkmiao/nodeswarm/internal/manager"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestEvaluateAndMutePersistence(t *testing.T) {
	now := time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC)
	agents := []control.AgentView{
		{ID: "offline", Name: "Cloud-A", Status: "offline", LastSeen: now.Add(-5 * time.Minute).Format(time.RFC3339)},
		{ID: "error", Name: "Cloud-B", Status: "online", LastSeen: now.Format(time.RFC3339), Desired: control.ProxyConfig{Enabled: true}, Runtime: control.ProxyRuntime{Status: "error", Error: "port busy"}},
		{ID: "busy", Name: "Cloud-C", Status: "online", LastSeen: now.Format(time.RFC3339), Metrics: control.Metrics{CPU: 95, MemoryUsed: 95, MemoryTotal: 100}},
		{ID: "late", Name: "Cloud-D", Status: "online", LastSeen: now.Format(time.RFC3339), DesiredAt: now.Add(-2 * time.Minute).Format(time.RFC3339), Desired: control.ProxyConfig{Enabled: true}, Revision: 5, Runtime: control.ProxyRuntime{Status: "running", Revision: 4}},
	}
	nodes := []manager.Node{{ID: "expired", Name: "HK", Enabled: true, ExpiresAt: now.Add(-time.Hour).Format(time.RFC3339)}, {ID: "failed", Name: "Tokyo", Enabled: true, LastStatus: "failed", LastError: "connect failed"}}
	alerts := Evaluate(agents, nodes, now)
	if len(alerts) != 7 {
		t.Fatalf("want 7 distinct alerts, got %d: %+v", len(alerts), alerts)
	}
	if alerts[0].Severity != "critical" {
		t.Fatal("critical should sort first")
	}
	for _, a := range alerts {
		if a.ID == "" || a.TargetID == "" || a.Title == "" {
			t.Fatalf("invalid alert %+v", a)
		}
	}
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err = store.Mute(alerts[0].ID, 24); err != nil {
		t.Fatal(err)
	}
	out, active := store.List(Evaluate(agents, nodes, now), time.Now())
	if active != 6 || out[0].MutedUntil == "" {
		t.Fatal("mute not reflected")
	}
	other, err := Open(filepath.Dir(store.path))
	if err != nil {
		t.Fatal(err)
	}
	_, active = other.List(Evaluate(agents, nodes, now), time.Now())
	if active != 6 {
		t.Fatal("mute not durable")
	}
	if err = other.Unmute(alerts[0].ID); err != nil {
		t.Fatal(err)
	}
	_, active = other.List(Evaluate(agents, nodes, now), time.Now())
	if active != 7 {
		t.Fatal("unmute not working")
	}
}

func TestEvaluateReportsRejectedConfigWithoutDriftDelay(t *testing.T) {
	now := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	agent := control.AgentView{ID: "cloud-rollback", Name: "Cloud Rollback", Kind: "cloud", Status: "online", LastSeen: now.Format(time.RFC3339),
		DesiredAt: now.Add(-5 * time.Second).Format(time.RFC3339), Revision: 9, Desired: control.ProxyConfig{Enabled: true},
		Runtime: control.ProxyRuntime{Status: "running", Revision: 8, Error: "configuration rejected: address already in use"}}
	alerts := Evaluate([]control.AgentView{agent}, nil, now)
	if len(alerts) != 1 || alerts[0].ID != "agent:cloud-rollback:config-rejected" || alerts[0].Severity != "warning" {
		t.Fatalf("expected immediate rollback warning: %+v", alerts)
	}
	if alerts[0].Detail == "" || !strings.Contains(alerts[0].Detail, "address already in use") {
		t.Fatalf("error detail lost: %+v", alerts[0])
	}
}

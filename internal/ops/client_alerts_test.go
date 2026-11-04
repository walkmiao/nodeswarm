package ops

import (
	"strings"
	"testing"
	"time"

	"github.com/walkmiao/nodeswarm/internal/control"
)

func liveClientAlertFixture(now time.Time) control.AgentView {
	return control.AgentView{ID: "win-1", Kind: "client", Name: "Windows-A", Status: "online",
		LastSeen: now.Format(time.RFC3339), Revision: 8,
		DesiredAt:      now.Add(-time.Minute).Format(time.RFC3339),
		ClientBindings: []control.ClientBinding{{PID: 41, NodeID: "a"}, {PID: 42, NodeID: "b"}},
		Client:         control.ClientSnapshot{Backend: "pidtcp", Revision: 8}}
}
func TestClientEngineFailuresAreGroupedOnce(t *testing.T) {
	now := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	a := liveClientAlertFixture(now)
	a.Client.Routes = []control.ClientRoute{
		{PID: 41, Status: "engine_error", Detail: "WinDivert not installed"},
		{PID: 42, Status: "engine_exited", Detail: "capture stopped"}}
	alerts := Evaluate([]control.AgentView{a}, nil, now)
	if len(alerts) != 1 || alerts[0].ID != "agent:win-1:client-engine" || alerts[0].Severity != "critical" ||
		!strings.Contains(alerts[0].Detail, "2 / 2") || !strings.Contains(alerts[0].Detail, "WinDivert") {
		t.Fatalf("missing grouped client failure: %+v", alerts)
	}
}
func TestClientPolicyDriftAndInvalidBindingAreActionable(t *testing.T) {
	now := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	a := liveClientAlertFixture(now)
	a.Client.Revision = 7
	a.Client.Routes = []control.ClientRoute{{PID: 41, Status: "stale_pid", Detail: "PID reused"}}
	alerts := Evaluate([]control.AgentView{a}, nil, now)
	if len(alerts) != 2 {
		t.Fatalf("want drift and invalid binding alerts, got %+v", alerts)
	}
	ids := map[string]bool{}
	for _, alert := range alerts {
		ids[alert.ID] = true
	}
	if !ids["agent:win-1:client-policy-drift"] || !ids["agent:win-1:client-bindings"] {
		t.Fatalf("unexpected alerts: %+v", alerts)
	}
	a.DesiredAt = now.Add(-10 * time.Second).Format(time.RFC3339)
	if got := Evaluate([]control.AgentView{a}, nil, now); len(got) != 1 || got[0].ID != "agent:win-1:client-bindings" {
		t.Fatalf("fresh policy should not alert for drift: %+v", got)
	}
}
func TestObserveAndUnverifiedTCPDoNotClaimExecutionFailure(t *testing.T) {
	now := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	a := liveClientAlertFixture(now)
	a.Client.Backend = "observe"
	a.Client.Routes = []control.ClientRoute{{PID: 41, Status: "planned_only"}}
	if got := Evaluate([]control.AgentView{a}, nil, now); len(got) != 0 {
		t.Fatalf("Observe false alarm: %+v", got)
	}
	a.Client.Backend = "pidtcp"
	a.Client.Routes[0].Status = "pid_tcp_unverified"
	if got := Evaluate([]control.AgentView{a}, nil, now); len(got) != 0 {
		t.Fatalf("unverified egress is not an engine failure: %+v", got)
	}
}
func TestClientScanFailureHasSingleAlertAndOfflineRemainsPrimary(t *testing.T) {
	now := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	a := liveClientAlertFixture(now)
	a.Client.ScanError = "access denied"
	a.Client.Routes = []control.ClientRoute{{PID: 41, Status: "scan_unavailable"}}
	got := Evaluate([]control.AgentView{a}, nil, now)
	if len(got) != 1 || got[0].ID != "agent:win-1:client-scan" {
		t.Fatalf("scan alert missing or duplicated: %+v", got)
	}
	a.Status = "offline"
	a.LastSeen = now.Add(-5 * time.Minute).Format(time.RFC3339)
	got = Evaluate([]control.AgentView{a}, nil, now)
	if len(got) != 1 || got[0].ID != "agent:win-1:offline" {
		t.Fatalf("offline state duplicated: %+v", got)
	}
}

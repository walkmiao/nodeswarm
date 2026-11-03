package main

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/walkmiao/nodeswarm/internal/control"
	"github.com/walkmiao/nodeswarm/internal/manager"
	"github.com/walkmiao/nodeswarm/internal/ops"
)

func TestClientScopedDeviceCannotControlAnotherWorkstation(t *testing.T) {
	dir := t.TempDir()
	store, e := manager.OpenStore(dir)
	if e != nil {
		t.Fatal(e)
	}
	reg, e := control.NewRegistry(dir)
	if e != nil {
		t.Fatal(e)
	}
	alerts, e := ops.Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	a := &app{store: store, registry: reg, alerts: alerts, adminToken: "admin-secret", enrollKey: "enroll-key"}
	handler := a.routes()
	request := func(method, path, token string, payload any) (int, string) {
		t.Helper()
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(method, "http://localhost:8088"+path, bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		return w.Code, w.Body.String()
	}
	id1, token1, e := reg.Enroll(control.EnrollInput{Kind: "client", Name: "PC-01"}, "")
	if e != nil {
		t.Fatal(e)
	}
	id2, token2, e := reg.Enroll(control.EnrollInput{Kind: "client", Name: "PC-02"}, "")
	if e != nil {
		t.Fatal(e)
	}
	p1 := control.ClientProcess{PID: 1200, Name: "Client.exe", StartTime: "start-1200"}
	p2 := control.ClientProcess{PID: 1300, Name: "Client.exe", StartTime: "start-1300"}
	for _, entry := range []struct {
		id string
		p  control.ClientProcess
	}{{id1, p1}, {id2, p2}} {
		_, e = reg.Report(control.Heartbeat{ID: entry.id, Client: &control.ClientSnapshot{ObservedAt: time.Now().UTC().Format(time.RFC3339), Processes: []control.ClientProcess{entry.p}}}, "")
		if e != nil {
			t.Fatal(e)
		}
	}
	node, e := store.Create(manager.NodeInput{Name: "Node A", Host: "203.0.113.1", Port: 1080, Username: "proxyuser", Password: "never-show-this-secret", Capacity: 1})
	if e != nil {
		t.Fatal(e)
	}
	code, body := request("GET", "/api/agent/client/context?id="+id1, token1, nil)
	if code != 200 || !strings.Contains(body, "Node A") || strings.Contains(body, "never-show-this-secret") {
		t.Fatalf("scoped context: %d %s", code, body)
	}
	for _, tc := range []struct{ id, token string }{{id2, token1}, {id1, "invalid"}, {id2, ""}} {
		if code, _ := request("GET", "/api/agent/client/context?id="+tc.id, tc.token, nil); code != 401 {
			t.Fatalf("cross-device context accepted: %d", code)
		}
	}
	input := map[string]any{"id": id1, "expectedRevision": 0, "bindings": []control.ClientBinding{{PID: p1.PID, StartTime: p1.StartTime, NodeID: node.ID}}}
	if code, _ := request("PUT", "/api/agent/client/bindings", token2, input); code != 401 {
		t.Fatalf("other client modified device: %d", code)
	}
	if code, _ := request("PUT", "/api/agent/client/bindings", token1, map[string]any{"id": id1, "bindings": input["bindings"]}); code != 400 {
		t.Fatalf("missing CAS revision accepted: %d", code)
	}
	code, body = request("PUT", "/api/agent/client/bindings", token1, input)
	if code != 200 || !strings.Contains(body, "Client.exe") || strings.Contains(body, "never-show-this-secret") {
		t.Fatalf("own bind: %d %s", code, body)
	}
	if code, _ := request("PUT", "/api/agent/client/bindings", token1, input); code != 409 {
		t.Fatalf("stale device CAS accepted: %d", code)
	}
	// A second machine cannot oversell a one-capacity shared node.
	second := map[string]any{"id": id2, "expectedRevision": 0, "bindings": []control.ClientBinding{{PID: p2.PID, StartTime: p2.StartTime, NodeID: node.ID}}}
	if code, _ := request("PUT", "/api/agent/client/bindings", token2, second); code != 409 {
		t.Fatalf("oversold node: %d", code)
	}
	code, body = request("GET", "/api/agent/client/context?id="+id1, "admin-secret", nil)
	if code != 401 {
		t.Fatalf("admin token should not act as a device credential: %d %s", code, body)
	}
}

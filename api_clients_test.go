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

func TestRemoteWindowsClientLifecycle(t *testing.T) {
	dir := t.TempDir()
	store, err := manager.OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	reg, err := control.NewRegistry(dir)
	if err != nil {
		t.Fatal(err)
	}
	alerts, err := ops.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	h := (&app{store: store, registry: reg, alerts: alerts, adminToken: "admin-secret", enrollKey: "enroll-secret"}).routes()
	request := func(method, path, token string, payload any) (int, string) {
		t.Helper()
		b, _ := json.Marshal(payload)
		r := httptest.NewRequest(method, "http://localhost:8088"+path, bytes.NewReader(b))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code, w.Body.String()
	}
	code, _ := request("GET", "/api/clients", "", nil)
	if code != 401 {
		t.Fatal("admin clients endpoint unauthenticated")
	}
	code, body := request("POST", "/api/agent/enroll", "enroll-secret", control.EnrollInput{Kind: "client", Name: "Win-01", OS: "windows"})
	if code != 201 {
		t.Fatalf("register: %d %s", code, body)
	}
	var cred struct{ ID, Token string }
	_ = json.Unmarshal([]byte(body), &cred)
	p := control.ClientProcess{PID: 400, Name: "Client.exe", Path: "C:/game/Client.exe", StartTime: "2026-11-03T01:02:03Z"}
	code, body = request("POST", "/api/agent/heartbeat", cred.Token, control.Heartbeat{ID: cred.ID, Client: &control.ClientSnapshot{Backend: "observe", Processes: []control.ClientProcess{p}, ObservedAt: time.Now().UTC().Format(time.RFC3339)}})
	if code != 200 {
		t.Fatalf("heartbeat: %d %s", code, body)
	}
	code, _ = request("PUT", "/api/clients/"+cred.ID+"/limits", "", map[string]any{"maxProxyPids": 1})
	if code != 401 {
		t.Fatal("unauthenticated quota change accepted")
	}
	code, body = request("PUT", "/api/clients/"+cred.ID+"/limits", "admin-secret", map[string]any{"maxProxyPids": 1, "expectedRevision": 0})
	if code != 200 || !strings.Contains(body, `"maxProxyPids":1`) {
		t.Fatalf("quota update failed: %d %s", code, body)
	}
	code, _ = request("PUT", "/api/clients/"+cred.ID+"/limits", "admin-secret", map[string]any{"maxProxyPids": 2, "expectedRevision": 0})
	if code != 409 {
		t.Fatal("stale limit edit accepted")
	}
	node, err := store.Create(manager.NodeInput{Name: "Tokyo", Host: "203.0.113.8", Port: 1080, Username: "me", Password: "sensitive-9876", Capacity: 1})
	if err != nil {
		t.Fatal(err)
	}
	payload := map[string]any{"bindings": []map[string]any{{"pid": p.PID, "startTime": p.StartTime, "nodeId": node.ID}}}
	code, body = request("PUT", "/api/clients/"+cred.ID+"/bindings", "admin-secret", payload)
	if code != 200 {
		t.Fatalf("bind: %d %s", code, body)
	}
	if strings.Contains(body, "sensitive-9876") {
		t.Fatal("proxy password leaked in admin binding response")
	}
	code, body = request("POST", "/api/agent/heartbeat", cred.Token, control.Heartbeat{ID: cred.ID, Client: &control.ClientSnapshot{Backend: "observe", Processes: []control.ClientProcess{p}, ObservedAt: time.Now().UTC().Format(time.RFC3339)}})
	if code != 200 || !strings.Contains(body, "sensitive-9876") || !strings.Contains(body, `"maxProxyPids":1`) {
		t.Fatalf("client policy not delivered to authenticated agent: %d %s", code, body)
	}
	code, body = request("GET", "/api/dashboard", "admin-secret", nil)
	if code != 200 || !strings.Contains(body, "Win-01") || strings.Contains(body, "sensitive-9876") {
		t.Fatalf("dashboard leak or client missing: %d %s", code, body)
	}
	if !strings.Contains(body, `"clients"`) {
		t.Fatal("missing clients dashboard collection")
	}
	code, _ = request("DELETE", "/api/clients/"+cred.ID, "admin-secret", nil)
	if code != 200 {
		t.Fatal("client revoke rejected")
	}
	code, _ = request("POST", "/api/agent/heartbeat", cred.Token, control.Heartbeat{ID: cred.ID, Client: &control.ClientSnapshot{}})
	if code != 401 {
		t.Fatal("revoked agent still authorized")
	}
}

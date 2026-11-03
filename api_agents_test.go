package main

import (
	"bytes"
	"encoding/json"
	"github.com/walkmiao/nodeswarm/internal/control"
	"github.com/walkmiao/nodeswarm/internal/manager"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAgentEnrollmentHeartbeatAndManagedNode(t *testing.T) {
	dir := t.TempDir()
	s, err := manager.OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	reg, err := control.NewRegistry(dir)
	if err != nil {
		t.Fatal(err)
	}
	app := &app{store: s, registry: reg, adminToken: "admin-secret", enrollKey: "enroll-secret"}
	h := app.routes()
	request := func(method, path, token string, payload any) (int, []byte) {
		t.Helper()
		var b []byte
		if payload != nil {
			b, _ = json.Marshal(payload)
		}
		r := httptest.NewRequest(method, "http://localhost:8088"+path, bytes.NewReader(b))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code, w.Body.Bytes()
	}
	code, _ := request("GET", "/api/dashboard", "", nil)
	if code != 401 {
		t.Fatalf("unauthenticated admin access: %d", code)
	}
	code, _ = request("POST", "/api/agent/enroll", "invalid", control.EnrollInput{Name: "node-1"})
	if code != 401 {
		t.Fatal("invalid enrollment allowed")
	}
	code, b := request("POST", "/api/agent/enroll", "enroll-secret", control.EnrollInput{Name: "node-1", OS: "linux", Arch: "amd64"})
	if code != 201 {
		t.Fatalf("enroll: %d %s", code, b)
	}
	var creds struct{ ID, Token string }
	_ = json.Unmarshal(b, &creds)
	code, _ = request("POST", "/api/agent/heartbeat", "not-valid", control.Heartbeat{ID: creds.ID})
	if code != 401 {
		t.Fatal("unauth heartbeat allowed")
	}
	code, b = request("POST", "/api/agent/heartbeat", creds.Token, control.Heartbeat{ID: creds.ID, Hostname: "host", Metrics: control.Metrics{CPU: 10}, Runtime: control.ProxyRuntime{Status: "stopped"}})
	if code != 200 {
		t.Fatalf("heartbeat: %d %s", code, b)
	}
	cfg := control.ProxyConfig{Enabled: true, Listen: "127.0.0.1", PublicHost: "203.0.113.15", Port: 11080, Username: "node", Password: "super_secret_123", MaxConnections: 200, Capacity: 8, UDP: true}
	code, b = request("PUT", "/api/agents/"+creds.ID+"/config", "admin-secret", control.ConfigureInput{Name: "node-1", Proxy: cfg})
	if code != 200 {
		t.Fatalf("configure: %d %s", code, b)
	}
	if strings.Contains(string(b), "super_secret_123") {
		t.Fatal("password exposed to admin")
	}
	code, b = request("GET", "/api/dashboard", "admin-secret", nil)
	if code != 200 || !strings.Contains(string(b), "203.0.113.15") {
		t.Fatalf("managed proxy missing: %d %s", code, b)
	}
	code, b = request("GET", "/api/agents/"+creds.ID+"/profile?process=Client.exe", "admin-secret", nil)
	if code != 200 || !strings.Contains(string(b), "Client.exe") || !strings.Contains(string(b), "super_secret_123") {
		t.Fatalf("Profile failure: %d", code)
	}
	code, _ = request("DELETE", "/api/agents/"+creds.ID, "admin-secret", nil)
	if code != 409 {
		t.Fatalf("deleted running desired config: %d", code)
	}
	code, _ = request("POST", "/api/agents/"+creds.ID+"/action", "admin-secret", map[string]any{"action": "stop"})
	if code != 200 {
		t.Fatalf("stop command failed: %d", code)
	}
	code, _ = request("DELETE", "/api/agents/"+creds.ID, "admin-secret", nil)
	if code != 200 {
		t.Fatalf("revoke should work after stopped reported: %d", code)
	}
}

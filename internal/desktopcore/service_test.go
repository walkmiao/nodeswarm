package desktopcore

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/walkmiao/nodeswarm/internal/control"
	"github.com/walkmiao/nodeswarm/internal/manager"
)

func TestSettingsPersistenceNoEnrollmentKey(t *testing.T) {
	dir := t.TempDir()
	s, e := New(dir)
	if e != nil {
		t.Fatal(e)
	}
	v := Settings{Controller: "https://console.example.com", Name: "Windows 01", Targets: "Client.exe", Engine: "observe", PIDHardLimit: 4, AutoConnect: true}
	if e = s.SaveSettings(v); e != nil {
		t.Fatal(e)
	}
	b, e := os.ReadFile(filepath.Join(dir, "settings.json"))
	if e != nil {
		t.Fatal(e)
	}
	if strings.Contains(string(b), "enrollment") || strings.Contains(string(b), "password") {
		t.Fatal("settings contains secret field")
	}
	next, e := New(dir)
	if e != nil || next.GetState().Settings.PIDHardLimit != 4 {
		t.Fatalf("restore: %v", e)
	}
	if e = next.SaveSettings(Settings{Controller: "http://example.com", Name: "x", Targets: "Client.exe", Engine: "observe"}); e == nil {
		t.Fatal("insecure public controller accepted")
	}
	if e = next.Start(""); e == nil {
		t.Fatal("startup without settings or enrollment must be rejected")
	}
}

func TestDesktopDeviceBindingAndHardLimit(t *testing.T) {
	dir := t.TempDir()
	var putCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer device-secret" {
			w.WriteHeader(401)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/agent/client/context":
			_ = json.NewEncoder(w).Encode(map[string]any{"client": control.AgentView{Kind: "client", ID: "device-1", Revision: 0, MaxProxyPIDs: 8, Client: control.ClientSnapshot{Processes: []control.ClientProcess{{PID: 900, StartTime: "start9", Name: "Client.exe"}, {PID: 901, StartTime: "start10", Name: "Client.exe"}}}}, "nodes": []manager.Node{{ID: "node-1", Name: "Node", Capacity: 8, Enabled: true, Host: "127.0.0.1", Port: 1080}}})
		case "/api/agent/client/bindings":
			putCount.Add(1)
			var req struct {
				Bindings []control.ClientBinding `json:"bindings"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			_ = json.NewEncoder(w).Encode(map[string]any{"client": control.AgentView{ID: "device-1", Revision: 1, MaxProxyPIDs: 8, ClientBindings: req.Bindings}})
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	s, e := New(dir)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.SaveSettings(Settings{Controller: server.URL, Name: "Win", Targets: "Client.exe", Engine: "observe", PIDHardLimit: 1}); e != nil {
		t.Fatal(e)
	}
	if e = os.MkdirAll(s.agentDir(), 0700); e != nil {
		t.Fatal(e)
	}
	data, _ := json.Marshal(credentials{ID: "device-1", Token: "device-secret", Role: "client", Controller: server.URL})
	if e = os.WriteFile(filepath.Join(s.agentDir(), "agent.json"), data, 0600); e != nil {
		t.Fatal(e)
	}
	if e = s.Refresh(); e != nil {
		t.Fatal(e)
	}
	if !s.GetState().CredentialReady || len(s.GetState().Nodes) != 1 {
		t.Fatal("device context missing")
	}
	_, e = s.AutoFill()
	if e != nil {
		t.Fatal(e)
	}
	if putCount.Load() != 1 {
		t.Fatalf("expected one safe update, got %d", putCount.Load())
	}
	s.mu.Lock()
	s.status.Remote.ClientBindings = []control.ClientBinding{{PID: 900, StartTime: "start9", NodeID: "node-1"}}
	s.mu.Unlock()
	e = s.SendBindings([]control.ClientBinding{{PID: 900, StartTime: "start9", NodeID: "node-1"}, {PID: 901, StartTime: "start10", NodeID: "node-1"}})
	if e == nil || !strings.Contains(e.Error(), "上限") {
		t.Fatalf("local hard quota was bypassed: %v", e)
	}
}

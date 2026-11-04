package agent

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/walkmiao/nodeswarm/internal/control"
)

func TestHeartbeatImmediatelyReportsRejectedProxyUpdate(t *testing.T) {
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	active := control.ProxyConfig{Enabled: true, Listen: "127.0.0.1", Port: port, Username: "u", Password: "old-password", Capacity: 8, MaxConnections: 8}
	var mu sync.Mutex
	var reports []control.ProxyRuntime
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/agent/heartbeat" || r.Header.Get("Authorization") != "Bearer device-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		var hb control.Heartbeat
		if err := json.NewDecoder(r.Body).Decode(&hb); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		mu.Lock()
		reports = append(reports, hb.Runtime)
		mu.Unlock()
		invalid := active
		invalid.Password = "" // remote desired revision cannot start
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(control.HeartbeatResponse{Revision: 2, Proxy: invalid})
	}))
	defer server.Close()
	d := &daemon{cfg: Config{Controller: server.URL}, creds: credential{ID: "cloud-a", Token: "device-token"},
		client: &http.Client{Timeout: 3 * time.Second}, started: time.Now(), lastContact: time.Now()}
	if err := d.service.Apply(1, active); err != nil {
		t.Fatal(err)
	}
	defer d.service.Stop()
	if err := d.heartbeat(context.Background()); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	observed := append([]control.ProxyRuntime(nil), reports...)
	mu.Unlock()
	if len(observed) != 2 {
		t.Fatalf("want initial heartbeat + immediate failure ack, got %d", len(observed))
	}
	after := observed[1]
	if after.Revision != 1 || after.Status != "running" || !strings.Contains(after.Error, "configuration rejected") || after.Address == "" {
		t.Fatalf("rollback state not acknowledged immediately: %+v", after)
	}
	if got := d.service.Snapshot(); got.Revision != 1 || got.Status != "running" {
		t.Fatalf("old listener lost: %+v", got)
	}
}

package main

import (
	"encoding/json"
	"github.com/walkmiao/nodeswarm/internal/manager"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAPIStateAndNodeCredentials(t *testing.T) {
	s, err := manager.OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	handler := (&app{store: s, adminToken: "test_admin_token"}).routes()
	body := `{"name":"test","host":"127.0.0.1","port":1080,"username":"user","password":"secret","capacity":2}`
	req := httptest.NewRequest("POST", "http://127.0.0.1:8088/api/nodes", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test_admin_token")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != 201 {
		t.Fatalf("create: %d %s", res.Code, res.Body.String())
	}
	if strings.Contains(res.Body.String(), "secret") {
		t.Fatal("password exposed")
	}
	req = httptest.NewRequest("GET", "http://127.0.0.1:8088/api/state", nil)
	req.Header.Set("Authorization", "Bearer test_admin_token")
	res = httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	var state struct {
		Nodes       []manager.Node `json:"nodes"`
		ProxyEngine string         `json:"proxyEngine"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &state); err != nil {
		t.Fatal(err)
	}
	if len(state.Nodes) != 1 || state.Nodes[0].HasPassword != true || state.ProxyEngine != "client_transparent_proxy_not_integrated" {
		t.Fatal("bad state")
	}
	req = httptest.NewRequest("POST", "http://127.0.0.1:8088/api/nodes", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test_admin_token")
	req.Header.Set("Origin", "http://malicious.example")
	req.Header.Set("Authorization", "Bearer test_admin_token")
	res = httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusForbidden {
		t.Fatalf("cross-origin write allowed: %d", res.Code)
	}
}

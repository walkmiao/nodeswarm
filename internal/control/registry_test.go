package control

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRegistryCredentialEncryptionAndCommands(t *testing.T) {
	dir := t.TempDir()
	s, err := NewRegistry(dir)
	if err != nil {
		t.Fatal(err)
	}
	id, token, err := s.Enroll(EnrollInput{Name: "Cloud-1", OS: "linux"}, "203.0.113.7")
	if err != nil {
		t.Fatal(err)
	}
	if !s.Authorize(id, token) || s.Authorize(id, "incorrect") {
		t.Fatal("agent authorization failed")
	}
	secret := "very_private_node_password_2026"
	c := ProxyConfig{Enabled: true, Listen: "127.0.0.1", PublicHost: "203.0.113.9", Port: 1080, Username: "demo", Password: secret, Capacity: 8, MaxConnections: 256, UDP: true}
	v, err := s.Configure(id, ConfigureInput{Name: "Tokyo", Proxy: c})
	if err != nil {
		t.Fatal(err)
	}
	if !v.Desired.HasPassword || v.Desired.Password != "" || v.Revision != 1 {
		t.Fatal("credential leaked or invalid revision")
	}
	raw, err := os.ReadFile(filepath.Join(dir, "agents.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), secret) || strings.Contains(string(raw), token) {
		t.Fatal("secret written to disk in plaintext")
	}
	s2, err := NewRegistry(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !s2.Authorize(id, token) {
		t.Fatal("token lost on restart")
	}
	got, ok := s2.Credentials(id)
	if !ok || got.Password != secret {
		t.Fatal("encrypted password not restored")
	}
	_, err = s2.Report(Heartbeat{ID: id, Metrics: Metrics{CPU: 15.3, At: time.Now().UTC().Format(time.RFC3339)}, Runtime: ProxyRuntime{Status: "running", Revision: 1}}, "203.0.113.7")
	if err != nil {
		t.Fatal(err)
	}
	if len(s2.List()[0].History) == 0 || s2.List()[0].Status != "online" {
		t.Fatal("heartbeat not stored")
	}
	_, err = s2.Action(id, "stop")
	if err != nil {
		t.Fatal(err)
	}
	v, _ = s2.Find(id)
	if v.Desired.Enabled || v.Revision != 2 {
		t.Fatal("stop command incorrect")
	}
	_, err = s2.Action(id, "start")
	if err != nil {
		t.Fatal(err)
	}
	v, _ = s2.Find(id)
	if !v.Desired.Enabled || v.Revision != 3 {
		t.Fatal("start command incorrect")
	}
	b, _ := json.Marshal(v)
	if strings.Contains(string(b), secret) {
		t.Fatal("admin view exposes password")
	}
}
func TestValidateConfig(t *testing.T) {
	c := ProxyConfig{Enabled: true, Listen: "0.0.0.0", Port: 1080, Username: "u", Password: "p", Capacity: 1, MaxConnections: 32, AllowedCIDRs: []string{"10.1.0.0/16"}}
	if err := ValidateConfig(c); err != nil {
		t.Fatal(err)
	}
	c.AllowedCIDRs = []string{"bad"}
	if ValidateConfig(c) == nil {
		t.Fatal("invalid CIDR allowed")
	}
	c.AllowedCIDRs = nil
	c.Username = ""
	if ValidateConfig(c) == nil {
		t.Fatal("unprotected SOCKS service allowed")
	}
}

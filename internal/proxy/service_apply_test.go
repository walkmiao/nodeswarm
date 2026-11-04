package proxy

import (
	"errors"
	"net"
	"strings"
	"testing"

	"github.com/walkmiao/nodeswarm/internal/control"
)

func availablePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()
	return port
}

func validProxyConfig(port int) control.ProxyConfig {
	return control.ProxyConfig{Enabled: true, Listen: "127.0.0.1", Port: port,
		Username: "tester", Password: "password", Capacity: 8, MaxConnections: 16}
}

func assertListening(t *testing.T, s *Service, revision int64) {
	t.Helper()
	snap := s.Snapshot()
	if snap.Status != "running" || snap.Revision != revision || snap.Address == "" {
		t.Fatalf("unexpected live runtime: %+v", snap)
	}
	c, err := net.Dial("tcp", snap.Address)
	if err != nil {
		t.Fatalf("old proxy stopped accepting connections: %v", err)
	}
	_ = c.Close()
}

func TestApplyRejectsInvalidConfigAndPreservesLiveRevision(t *testing.T) {
	old := validProxyConfig(availablePort(t))
	var service Service
	t.Cleanup(service.Stop)
	if err := service.Apply(1, old); err != nil {
		t.Fatal(err)
	}
	invalid := old
	invalid.Password = ""
	if err := service.Apply(2, invalid); err == nil {
		t.Fatal("invalid config accepted")
	}
	assertListening(t, &service, 1)
	if service.Snapshot().Error == "" {
		t.Fatal("rejection should be reported")
	}
}

func TestApplyNewPortOccupiedKeepsOldProxy(t *testing.T) {
	old := validProxyConfig(availablePort(t))
	var service Service
	t.Cleanup(service.Stop)
	if err := service.Apply(11, old); err != nil {
		t.Fatal(err)
	}
	busy, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer busy.Close()
	newer := old
	newer.Port = busy.Addr().(*net.TCPAddr).Port
	if err := service.Apply(12, newer); err == nil {
		t.Fatal("port conflict not reported")
	}
	assertListening(t, &service, 11)
	busy.Close()
	if err := service.Apply(12, newer); err != nil {
		t.Fatal(err)
	}
	assertListening(t, &service, 12)
}

func TestApplySamePortRollsBackOnStartFailure(t *testing.T) {
	cfg := validProxyConfig(availablePort(t))
	var service Service
	t.Cleanup(service.Stop)
	if err := service.Apply(5, cfg); err != nil {
		t.Fatal(err)
	}
	next := cfg
	next.Password = "updated-password"
	service.starter = func(c control.ProxyConfig) (*Server, error) {
		if c.Password == next.Password {
			return nil, errors.New("simulated initialization failure")
		}
		return Start(c)
	}
	if err := service.Apply(6, next); err == nil {
		t.Fatal("expected simulated failure")
	}
	assertListening(t, &service, 5)
	if !strings.Contains(service.Snapshot().Error, "restored") {
		t.Fatal("missing rollback status")
	}
	service.starter = nil
	if err := service.Apply(6, next); err != nil {
		t.Fatal(err)
	}
	assertListening(t, &service, 6)
}

func TestApplySamePortRollbackAlsoFails(t *testing.T) {
	cfg := validProxyConfig(availablePort(t))
	var service Service
	t.Cleanup(service.Stop)
	if err := service.Apply(5, cfg); err != nil {
		t.Fatal(err)
	}
	service.starter = func(control.ProxyConfig) (*Server, error) { return nil, errors.New("bind failure") }
	if err := service.Apply(6, cfg); err == nil {
		t.Fatal("expected failure")
	}
	snap := service.Snapshot()
	if snap.Revision != 5 || snap.Status != "error" || snap.Address != "" || !strings.Contains(snap.Error, "rollback failed") {
		t.Fatalf("unexpected failed rollback: %+v", snap)
	}
}

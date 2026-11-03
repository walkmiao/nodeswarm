package client

import (
	"github.com/walkmiao/nodeswarm/internal/control"
	"os/exec"
	"runtime"
	"strings"
	"testing"
)

func TestSuspendReportsDirectConnectRiskAndClearsPlan(t *testing.T) {
	m, err := New(Config{DataDir: t.TempDir(), Engine: "observe"})
	if err != nil {
		t.Fatal(err)
	}
	m.state = control.ClientSnapshot{Routes: []control.ClientRoute{{PID: 123, Status: "process_name_unverified"}}}
	m.Suspend("heartbeat timeout")
	got := m.Observe()
	if !strings.Contains(got.Error, "controller_unreachable") || len(got.Routes) != 1 || got.Routes[0].Status != "controller_unreachable" || !strings.Contains(got.Routes[0].Detail, "直连") {
		t.Fatalf("unsafe suspend status: %+v", got)
	}
}

func TestSuspendTerminatesOwnedChild(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test uses POSIX test child; Windows CI checks native compilation")
	}
	m, err := New(Config{DataDir: t.TempDir(), Engine: "observe"})
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	ch := make(chan error, 1)
	go func() { ch <- cmd.Wait() }()
	m.command = cmd
	m.exited = ch
	m.fingerprint = "active"
	m.Suspend("network lost")
	if m.command != nil || m.fingerprint != "" {
		t.Fatal("suspend left child managed")
	}

}

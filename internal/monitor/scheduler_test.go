package monitor

import (
	"context"
	"github.com/walkmiao/nodeswarm/internal/manager"
	"testing"
	"time"
)

func TestScanOnceSkipsDisabledAndExpired(t *testing.T) {
	s, err := manager.OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	disabled := false
	_, err = s.Create(manager.NodeInput{Name: "disabled", Host: "127.0.0.1", Port: 29999, Capacity: 1, Enabled: &disabled})
	if err != nil {
		t.Fatal(err)
	}
	enabled := true
	_, err = s.Create(manager.NodeInput{Name: "expired", Host: "127.0.0.1", Port: 29999, Capacity: 1, Enabled: &enabled, ExpiresAt: time.Now().Add(-time.Hour).Format(time.RFC3339)})
	if err != nil {
		t.Fatal(err)
	}
	checked, ok := ScanOnce(context.Background(), s, 10, 2)
	if checked != 0 || ok != 0 {
		t.Fatalf("unexpected scans: %d/%d", checked, ok)
	}
}

func TestScanOnceCancelled(t *testing.T) {
	s, err := manager.OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	enabled := true
	_, err = s.Create(manager.NodeInput{Name: "will-not-probe", Host: "127.0.0.1", Port: 29999, Capacity: 1, Enabled: &enabled})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	checked, ok := ScanOnce(ctx, s, 10, 2)
	if checked != 0 || ok != 0 {
		t.Fatalf("cancelled scan did work: %d/%d", checked, ok)
	}
}

package control

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestEnrollmentSingleUsePersistAndRevoke(t *testing.T) {
	dir := t.TempDir()
	reg, err := NewRegistry(dir)
	if err != nil {
		t.Fatal(err)
	}
	grant, token, err := reg.CreateEnrollment("Tokyo-01", 60)
	if err != nil {
		t.Fatal(err)
	}
	if len(token) < 48 || len(reg.ListEnrollments()) != 1 {
		t.Fatal("missing issued grant")
	}
	raw, err := os.ReadFile(filepath.Join(dir, "agents.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), token) {
		t.Fatal("plaintext grant on disk")
	}
	reg, err = NewRegistry(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(reg.ListEnrollments()) != 1 || reg.ListEnrollments()[0].ID != grant.ID {
		t.Fatal("grant not restored")
	}
	if _, _, err = reg.EnrollWithGrant("wrong", EnrollInput{Name: "node1"}, ""); err == nil {
		t.Fatal("wrong grant accepted")
	}
	var successful atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id, secret, e := reg.EnrollWithGrant(token, EnrollInput{Name: "node1"}, "127.0.0.1")
			if e == nil {
				if !reg.Authorize(id, secret) {
					t.Error("Agent token not usable")
				}
				successful.Add(1)
			}
		}()
	}
	wg.Wait()
	if successful.Load() != 1 {
		t.Fatalf("single-use grant allowed %d enrollments", successful.Load())
	}
	if len(reg.ListEnrollments()) != 0 {
		t.Fatal("consumed grant not removed")
	}
	reg, err = NewRegistry(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = reg.EnrollWithGrant(token, EnrollInput{Name: "node2"}, ""); err == nil {
		t.Fatal("grant replay succeeded after restart")
	}
	grant2, _, err := reg.CreateEnrollment("Windows", 10)
	if err != nil {
		t.Fatal(err)
	}
	if err = reg.RevokeEnrollment(grant2.ID); err != nil {
		t.Fatal(err)
	}
	if len(reg.ListEnrollments()) != 0 {
		t.Fatal("revoke failed")
	}
}
func TestGrantExpiryAndValidation(t *testing.T) {
	reg, _ := NewRegistry(t.TempDir())
	if _, _, err := reg.CreateEnrollment("", 60); err == nil {
		t.Fatal("empty label accepted")
	}
	if _, _, err := reg.CreateEnrollment("node", 0); err == nil {
		t.Fatal("invalid TTL accepted")
	}
	_, token, err := reg.CreateEnrollment("expired", 1)
	if err != nil {
		t.Fatal(err)
	}
	reg.mu.Lock()
	reg.enrollments[0].ExpiresAt = time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
	reg.mu.Unlock()
	if _, _, err = reg.EnrollWithGrant(token, EnrollInput{Name: "test"}, ""); err == nil {
		t.Fatal("expired grant accepted")
	}
}

package identity

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCredentialsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	original := Credentials{ID: "id-123", Role: "client", Token: "very-secret-token", Controller: "https://controller.test"}
	if err := Save(dir, original); err != nil {
		t.Fatal(err)
	}
	stored, err := os.ReadFile(filepath.Join(dir, "agent.json"))
	if err != nil {
		t.Fatal(err)
	}
	if dpapiAvailable() && bytes.Contains(stored, []byte(original.Token)) {
		t.Fatal("Windows persisted a plaintext device token")
	}
	got, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got != original {
		t.Fatalf("roundtrip mismatch: %#v", got)
	}
}

func TestLegacyCredentialUpgrade(t *testing.T) {
	dir := t.TempDir()
	legacy := Credentials{ID: "legacy", Role: "client", Token: "old-secret", Controller: "https://test.example"}
	raw, _ := json.Marshal(legacy)
	if err := os.WriteFile(filepath.Join(dir, "agent.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	got, err := Load(dir)
	if err != nil || got != legacy {
		t.Fatalf("legacy read: %+v %v", got, err)
	}
	disk, err := os.ReadFile(filepath.Join(dir, "agent.json"))
	if err != nil {
		t.Fatal(err)
	}
	if dpapiAvailable() && bytes.Contains(disk, []byte("old-secret")) {
		t.Fatal("Windows plaintext credential was not migrated")
	}
}

func TestCorruptedCredentialRejected(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "agent.json"), []byte(`{"id":"x","tokenDpapi":"not!base64","controller":"https://test"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(dir); err == nil {
		t.Fatal("corrupt token accepted")
	}
}

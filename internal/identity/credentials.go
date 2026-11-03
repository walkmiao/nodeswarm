// Package identity persists per-device credentials. On Windows the bearer
// token is protected with user-scoped DPAPI, never saved as plain JSON.
package identity

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type Credentials struct {
	Role       string `json:"role,omitempty"`
	ID         string `json:"id"`
	Token      string `json:"token,omitempty"`
	Controller string `json:"controller"`
}

type savedCredentials struct {
	Role       string `json:"role,omitempty"`
	ID         string `json:"id"`
	Token      string `json:"token,omitempty"`
	TokenDPAPI string `json:"tokenDpapi,omitempty"`
	Controller string `json:"controller"`
}

func Load(dir string) (Credentials, error) {
	var out Credentials
	contents, err := os.ReadFile(filepath.Join(dir, "agent.json"))
	if err != nil {
		return out, err
	}
	var raw savedCredentials
	if err := json.Unmarshal(contents, &raw); err != nil {
		return out, fmt.Errorf("decode device credentials: %w", err)
	}
	out = Credentials{ID: raw.ID, Role: raw.Role, Controller: raw.Controller}
	if raw.TokenDPAPI != "" {
		encrypted, e := base64.StdEncoding.DecodeString(raw.TokenDPAPI)
		if e != nil {
			return Credentials{}, fmt.Errorf("decode device token: %w", e)
		}
		plain, e := unprotect(encrypted)
		if e != nil {
			return Credentials{}, fmt.Errorf("device token unavailable to current Windows user: %w", e)
		}
		out.Token = string(plain)
		for i := range plain {
			plain[i] = 0
		}
	} else {
		out.Token = raw.Token
	}
	if out.ID == "" || out.Token == "" || out.Controller == "" {
		return Credentials{}, errors.New("invalid device credential")
	}
	// Migrate legacy plaintext files automatically when running on Windows.
	// The new file is written atomically, and the caller can continue with the
	// already parsed token even if the disk is briefly read-only.
	if raw.TokenDPAPI == "" && dpapiAvailable() {
		if err := Save(dir, out); err != nil {
			return Credentials{}, fmt.Errorf("migrate device credential: %w", err)
		}
	}
	return out, nil
}

func Save(dir string, c Credentials) error {
	if c.ID == "" || c.Token == "" || c.Controller == "" {
		return errors.New("incomplete device credential")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	raw := savedCredentials{Role: c.Role, ID: c.ID, Controller: c.Controller}
	if dpapiAvailable() {
		encrypted, err := protect([]byte(c.Token))
		if err != nil {
			return fmt.Errorf("protect device token: %w", err)
		}
		raw.TokenDPAPI = base64.StdEncoding.EncodeToString(encrypted)
	} else {
		raw.Token = c.Token
	}
	b, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".agent-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(b)
	}
	if err == nil {
		err = f.Sync()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(f.Name(), filepath.Join(dir, "agent.json"))
}

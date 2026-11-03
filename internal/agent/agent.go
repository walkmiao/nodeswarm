package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/walkmiao/nodeswarm/internal/client"
	"github.com/walkmiao/nodeswarm/internal/control"
	"github.com/walkmiao/nodeswarm/internal/proxy"
)

type Config struct {
	Role              string // agent/cloud or client
	ClientEngine      string
	ProxyBridgeCLI    string
	ClientTargets     string
	Controller        string
	Name              string
	EnrollKey         string
	DataDir           string
	AllowInsecureHTTP bool
}
type credential struct {
	Role       string `json:"role,omitempty"`
	ID         string `json:"id"`
	Token      string `json:"token"`
	Controller string `json:"controller"`
}
type daemon struct {
	cfg          Config
	creds        credential
	service      proxy.Service
	client       *http.Client
	logs         []control.AgentLog
	mu           sync.Mutex
	seq          int
	started      time.Time
	lastContact  time.Time
	disconnected bool
	cpu          cpuSampler
	workstation  *client.Manager
}

func (d *daemon) note(level, message string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.seq++
	d.logs = append(d.logs, control.AgentLog{ID: fmt.Sprintf("%d-%d", time.Now().UnixNano(), d.seq), At: time.Now().UTC().Format(time.RFC3339), Level: level, Message: message})
	if len(d.logs) > 24 {
		d.logs = d.logs[len(d.logs)-24:]
	}
	log.Printf("[%s] %s", level, message)
}
func (d *daemon) recentLogs() []control.AgentLog {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make([]control.AgentLog, len(d.logs))
	copy(out, d.logs)
	return out
}
func (d *daemon) request(ctx context.Context, path, token string, body any, ret any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(d.cfg.Controller, "/")+path, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := d.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, 256*1024))
	if err != nil {
		return err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("controller %s: HTTP %d: %s", path, res.StatusCode, strings.TrimSpace(string(data)))
	}
	if ret != nil {
		return json.Unmarshal(data, ret)
	}
	return nil
}
func loadCredential(dir string) (credential, error) {
	var c credential
	b, e := os.ReadFile(filepath.Join(dir, "agent.json"))
	if e != nil {
		return c, e
	}
	e = json.Unmarshal(b, &c)
	if e != nil {
		return c, e
	}
	if c.ID == "" || c.Token == "" {
		return c, errors.New("invalid agent credential")
	}
	return c, nil
}
func saveCredential(dir string, c credential) error {
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".agent-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = f.Chmod(0600); err != nil {
		f.Close()
		return err
	}
	if _, err = f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), filepath.Join(dir, "agent.json"))
}
func validateController(c Config) error {
	u, e := url.Parse(c.Controller)
	if e != nil || u.Hostname() == "" || u.User != nil {
		return errors.New("invalid controller URL")
	}
	if u.Scheme != "https" {
		ip := net.ParseIP(u.Hostname())
		isLocal := u.Hostname() == "localhost" || (ip != nil && ip.IsLoopback())
		if u.Scheme != "http" || (!isLocal && !c.AllowInsecureHTTP) {
			return errors.New("remote controller must use HTTPS; only local loopback HTTP is allowed without explicit insecure flag")
		}
	}
	if c.DataDir == "" {
		return errors.New("agent data directory required")
	}
	return nil
}
func Run(ctx context.Context, c Config) error {
	if err := validateController(c); err != nil {
		return err
	}
	if err := os.MkdirAll(c.DataDir, 0700); err != nil {
		return err
	}
	if c.Role == "" {
		c.Role = "agent"
	}
	if c.Role != "agent" && c.Role != "client" {
		return errors.New("unsupported agent role")
	}
	d := &daemon{cfg: c, client: &http.Client{Timeout: 12 * time.Second}, started: time.Now(), lastContact: time.Now()}
	if c.Role == "client" {
		wm, err := client.New(client.Config{DataDir: c.DataDir, Engine: c.ClientEngine, ProxyBridgeCLI: c.ProxyBridgeCLI, Targets: c.ClientTargets})
		if err != nil {
			return err
		}
		d.workstation = wm
		defer d.workstation.Stop()
	}
	cr, err := loadCredential(c.DataDir)
	if errors.Is(err, os.ErrNotExist) {
		if c.EnrollKey == "" {
			return errors.New("first run requires NODE_MANAGER_ENROLL_KEY to register")
		}
		hostname, _ := os.Hostname()
		var registered struct {
			ID    string `json:"id"`
			Token string `json:"token"`
		}
		kind := "cloud"
		if c.Role == "client" {
			kind = "client"
		}
		enroll := control.EnrollInput{Kind: kind, Name: c.Name, Hostname: hostname, OS: runtime.GOOS, Arch: runtime.GOARCH, Version: control.AgentVersion}
		ctxReq, cancel := context.WithTimeout(ctx, 15*time.Second)
		err = d.request(ctxReq, "/api/agent/enroll", c.EnrollKey, enroll, &registered)
		cancel()
		if err != nil {
			return fmt.Errorf("agent enrollment failed: %w", err)
		}
		cr = credential{ID: registered.ID, Token: registered.Token, Controller: c.Controller, Role: c.Role}
		if err = saveCredential(c.DataDir, cr); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	if cr.Controller != c.Controller {
		return errors.New("agent credential belongs to a different controller; use a different data directory")
	}
	if cr.Role == "" {
		cr.Role = "agent"
	} // pre-v1.2 cloud credentials
	if cr.Role != c.Role {
		return errors.New("credential was registered for a different role; use a separate data directory")
	}
	// The credential file must not be reused by a different agent role.
	// Controller stores the role at enrollment and rejects role-specific actions.
	d.creds = cr
	d.note("success", "Agent connected: "+c.Name)
	ticker := time.NewTicker(7 * time.Second)
	defer ticker.Stop()
	defer d.service.Stop()
	for {
		if err := d.heartbeat(ctx); err != nil {
			if strings.Contains(err.Error(), "HTTP 401") {
				d.service.Stop()
				return fmt.Errorf("Agent credential revoked or invalid: %w", err)
			}
			d.note("warning", fmt.Sprintf("heartbeat failed: %.250s", err.Error()))
			if time.Since(d.lastContact) > 10*time.Minute && !d.disconnected {
				d.service.Stop()
				if d.workstation != nil {
					d.workstation.Stop()
				}
				d.disconnected = true
				d.note("warning", "Controller unavailable for over 10 minutes; SOCKS5 service stopped (fail-closed)")
			}
		} else {
			d.lastContact = time.Now()
			d.disconnected = false
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}
func (d *daemon) heartbeat(ctx context.Context) error {
	hostname, _ := os.Hostname()
	metrics := d.cpu.readMetrics()
	metrics.At = time.Now().UTC().Format(time.RFC3339)
	metrics.UptimeSeconds = uint64(time.Since(d.started).Seconds())
	hb := control.Heartbeat{ID: d.creds.ID, Hostname: hostname, OS: runtime.GOOS, Arch: runtime.GOARCH, Version: control.AgentVersion,
		Metrics: metrics, Runtime: d.service.Snapshot(), Logs: d.recentLogs()}
	if d.workstation != nil {
		current := d.workstation.Observe()
		hb.Client = &current
	}
	var desired control.HeartbeatResponse
	reqCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	if err := d.request(reqCtx, "/api/agent/heartbeat", d.creds.Token, hb, &desired); err != nil {
		return err
	}
	if d.workstation != nil {
		if err := d.workstation.Apply(desired.Client); err != nil {
			d.note("error", "Client policy apply failed: "+err.Error())
		}
		// Immediate acknowledgement carries the observed engine status.
		updated := hb
		current := d.workstation.Observe()
		updated.Client = &current
		nextCtx, nextCancel := context.WithTimeout(ctx, 12*time.Second)
		err := d.request(nextCtx, "/api/agent/heartbeat", d.creds.Token, updated, nil)
		nextCancel()
		return err
	}
	prev := d.service.Snapshot()
	if desired.Revision != prev.Revision || prev.Status == "error" || (desired.Proxy.Enabled && prev.Status != "running") || (!desired.Proxy.Enabled && prev.Status != "stopped") {
		err := d.service.Apply(desired.Revision, desired.Proxy)
		if err != nil {
			d.note("error", "Proxy configuration failed: "+err.Error())
			return nil
		}
		cur := d.service.Snapshot()
		d.note("info", fmt.Sprintf("Proxy desired revision %d applied: %s", desired.Revision, cur.Status))
		// Acknowledge immediately instead of waiting another heartbeat interval.
		updated := hb
		updated.Runtime = cur
		updated.Logs = d.recentLogs()
		nextCtx, nextCancel := context.WithTimeout(ctx, 12*time.Second)
		err = d.request(nextCtx, "/api/agent/heartbeat", d.creds.Token, updated, nil)
		nextCancel()
		if err != nil {
			return err
		}
	}
	return nil
}

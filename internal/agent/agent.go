package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/walkmiao/nodeswarm/internal/identity"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/walkmiao/nodeswarm/internal/client"
	"github.com/walkmiao/nodeswarm/internal/control"
	"github.com/walkmiao/nodeswarm/internal/proxy"
)

type Config struct {
	Role               string // agent/cloud or client
	ClientEngine       string
	ProxyBridgeCLI     string
	ClientWinDivertDir string
	ClientTargets      string
	ClientPIDHardLimit int
	Controller         string
	Name               string
	EnrollKey          string
	DataDir            string
	AllowInsecureHTTP  bool
	OnClientReport     func(control.ClientSnapshot) // native desktop status; never contains proxy passwords
	OnConnection       func(bool, string)           // heartbeat and connection state for native desktop
}
type credential = identity.Credentials
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
func loadCredential(dir string) (credential, error) { return identity.Load(dir) }
func saveCredential(dir string, c credential) error { return identity.Save(dir, c) }
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
		wm, err := client.New(client.Config{DataDir: c.DataDir, Engine: c.ClientEngine, ProxyBridgeCLI: c.ProxyBridgeCLI, WinDivertDir: c.ClientWinDivertDir, Targets: c.ClientTargets, PIDHardLimit: c.ClientPIDHardLimit})
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
			// The workstation must not keep a stale process-name proxy profile
			// running while its lease to Controller is lost. This does not
			// block direct OS networking; the UI reports that limitation.
			if d.workstation != nil {
				d.workstation.Suspend(err.Error())
				if c.OnClientReport != nil {
					c.OnClientReport(d.workstation.Observe())
				}
			}
			if c.OnConnection != nil {
				c.OnConnection(false, err.Error())
			}
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
			if c.OnConnection != nil {
				c.OnConnection(true, "")
			}
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
		if d.cfg.OnClientReport != nil {
			d.cfg.OnClientReport(current)
		}
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
		if d.cfg.OnClientReport != nil {
			d.cfg.OnClientReport(current)
		}
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

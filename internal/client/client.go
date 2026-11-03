// Package client tracks application instances on a Windows workstation and
// adapts explicit per-name routing to an installed ProxyBridge CLI. It never
// claims to enforce distinct SOCKS5 routes for multiple PIDs of the same exe.
package client

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/walkmiao/nodeswarm/internal/control"
	"github.com/walkmiao/nodeswarm/internal/manager"
)

type Config struct {
	DataDir        string
	Engine         string // observe (default) or proxybridge
	ProxyBridgeCLI string // full path to an administrator-approved executable
	Targets        string // semicolon-separated basenames, e.g. Client.exe
	PIDHardLimit   int    // optional device-side upper bound, 0 = use Controller limit
}

type Manager struct {
	mu          sync.Mutex
	cfg         Config
	targets     map[string]bool
	processList func() ([]manager.Process, error)
	lastScan    time.Time
	all         []manager.Process
	scanError   string
	state       control.ClientSnapshot
	command     *exec.Cmd
	exited      chan error
	profile     string
	fingerprint string
}

func New(c Config) (*Manager, error) {
	if c.Engine == "" {
		c.Engine = "observe"
	}
	if c.PIDHardLimit < 0 || c.PIDHardLimit > control.AbsoluteMaxProxyPIDs {
		return nil, fmt.Errorf("PIDHardLimit must be 0-%d", control.AbsoluteMaxProxyPIDs)
	}
	if c.Engine != "observe" && c.Engine != "proxybridge" {
		return nil, fmt.Errorf("unsupported client engine %q", c.Engine)
	}
	if c.DataDir == "" {
		return nil, errors.New("client data directory required")
	}
	if err := os.MkdirAll(c.DataDir, 0700); err != nil {
		return nil, err
	}
	if c.Targets == "" {
		c.Targets = "Client.exe"
	}
	targets := map[string]bool{}
	for _, raw := range strings.Split(c.Targets, ";") {
		name := strings.ToLower(strings.TrimSpace(raw))
		if name == "" {
			continue
		}
		if name == "*" || strings.ContainsAny(name, "/\\\r\n*?") || len(name) > 100 {
			return nil, fmt.Errorf("invalid target executable name %q", raw)
		}
		targets[name] = true
	}
	if len(targets) == 0 || len(targets) > 12 {
		return nil, errors.New("must select 1-12 target process names")
	}
	m := &Manager{cfg: c, targets: targets, processList: manager.ListProcesses, state: control.ClientSnapshot{Backend: c.Engine, Processes: []control.ClientProcess{}, Routes: []control.ClientRoute{}}}
	return m, nil
}

// Observe is called once per heartbeat. Scans are cached to avoid starting a
// PowerShell enumeration on every seven-second network poll.
func (m *Manager) Observe() control.ClientSnapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.scanLocked()
	if m.exited != nil {
		select {
		case err := <-m.exited:
			m.command = nil
			m.exited = nil
			m.fingerprint = ""
			if m.profile != "" {
				_ = os.Remove(m.profile)
				m.profile = ""
			}
			m.state.Error = "ProxyBridge exited: " + fmt.Sprint(err)
			for i := range m.state.Routes {
				if m.state.Routes[i].Status == "process_name_unverified" {
					m.state.Routes[i].Status = "engine_exited"
				}
			}
		default:
		}
	}
	snapshot := m.state
	snapshot.Processes = slices.Clone(m.state.Processes)
	snapshot.Routes = slices.Clone(m.state.Routes)
	return snapshot
}
func (m *Manager) scanLocked() {
	if !m.lastScan.IsZero() && time.Since(m.lastScan) < 12*time.Second {
		return
	}
	m.lastScan = time.Now()
	all, err := m.processList()
	if err != nil {
		m.scanError = err.Error()
		m.state.ScanError = "进程扫描失败: " + err.Error()
		// A process-name proxy has no way to know which newly started PID
		// owns a flow when inventory is unavailable. Stop it instead of
		// silently retaining an unsafe profile from an earlier scan.
		m.stopLocked()
		m.state.Error = "scan_unavailable: " + err.Error()
		for i := range m.state.Routes {
			m.state.Routes[i].Status = "scan_unavailable"
			m.state.Routes[i].Detail = "进程身份不可验证；已停止托管代理程序"
		}
		return
	}
	m.scanError = ""
	m.all = all
	current := make([]control.ClientProcess, 0, 32)
	for _, p := range all {
		if m.targets[strings.ToLower(p.Name)] && p.PID > 0 && p.StartTime != "" {
			current = append(current, control.ClientProcess{PID: p.PID, StartTime: p.StartTime, Name: p.Name, Path: p.Path})
			if len(current) >= 128 {
				break
			}
		}
	}
	slices.SortFunc(current, func(a, b control.ClientProcess) int { return a.PID - b.PID })
	m.state.Processes = current
	m.state.ObservedAt = time.Now().UTC().Format(time.RFC3339)
	m.state.ScanError = ""
}

// BuildProfile checks every running process with the target name. ProxyBridge
// routes by ProcessName, not PID. If an unmatched same-name instance exists or
// two instances require different exits, activating it would leak routing to
// the wrong process; reject the entire profile instead.
func BuildProfile(policy control.ClientPolicy, all []manager.Process) ([]byte, error) {
	byIdentity := make(map[string]manager.Process, len(all))
	for _, p := range all {
		byIdentity[fmt.Sprintf("%d|%s", p.PID, p.StartTime)] = p
	}
	endpoints := map[string]control.ClientEndpoint{}
	for _, ep := range policy.Endpoints {
		if err := manager.ValidateHost(ep.Host); err != nil {
			return nil, err
		}
		if ep.Port <= 0 || ep.Port > 65535 || len(ep.Username) > 255 || len(ep.Password) > 255 {
			return nil, errors.New("invalid proxy endpoint")
		}
		endpoints[ep.NodeID] = ep
	}
	byName := map[string]string{}
	matched := map[string]bool{}
	for _, b := range policy.Bindings {
		p, found := byIdentity[fmt.Sprintf("%d|%s", b.PID, b.StartTime)]
		if !found || p.Name != b.Name {
			return nil, fmt.Errorf("PID %d no longer matches observed identity", b.PID)
		}
		ep, ok := endpoints[b.NodeID]
		if !ok {
			return nil, fmt.Errorf("proxy node %s is not available", b.NodeID)
		}
		_ = ep
		name := strings.ToLower(p.Name)
		if current, ok := byName[name]; ok && current != b.NodeID {
			return nil, fmt.Errorf("unsupported_pid_isolation: %s requires multiple nodes", p.Name)
		}
		byName[name] = b.NodeID
		matched[fmt.Sprintf("%d|%s", p.PID, p.StartTime)] = true
	}
	for _, p := range all {
		if _, ok := byName[strings.ToLower(p.Name)]; ok && !matched[fmt.Sprintf("%d|%s", p.PID, p.StartTime)] {
			return nil, fmt.Errorf("unsafe_name_collision: unmanaged %s PID %d would also be routed", p.Name, p.PID)
		}
	}
	names := make([]string, 0, len(byName))
	for name := range byName {
		names = append(names, name)
	}
	slices.Sort(names)
	configs := []map[string]any{}
	rules := []map[string]any{}
	for i, name := range names {
		ep := endpoints[byName[name]]
		id := i + 1
		configs = append(configs, map[string]any{"Id": id, "Type": "socks5", "Host": ep.Host, "Port": fmt.Sprintf("%d", ep.Port), "Username": ep.Username, "Password": ep.Password})
		rules = append(rules, map[string]any{"ProcessName": name, "TargetHosts": "*", "TargetPorts": "*", "Protocol": "BOTH", "Action": "PROXY", "IsEnabled": true, "LocalhostViaProxy": false, "ProxyConfigId": id})
	}
	return json.MarshalIndent(map[string]any{"Version": "1.0", "IsTrafficLoggingEnabled": true, "ProxyConfigs": configs, "ProxyRules": rules}, "", "  ")
}

func (m *Manager) Apply(policy *control.ClientPolicy) error {
	if policy == nil {
		return errors.New("client policy missing")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.scanLocked()
	// A failed scan must never be mistaken for an empty process inventory.
	if m.scanError != "" {
		m.state.Revision = policy.Revision
		return fmt.Errorf("process scan unavailable: %s", m.scanError)
	}
	routes := make([]control.ClientRoute, 0, len(policy.Bindings))
	for _, b := range policy.Bindings {
		routes = append(routes, control.ClientRoute{PID: b.PID, StartTime: b.StartTime, NodeID: b.NodeID, Status: "planned_only", Detail: "仅分配策略；尚未验证游戏流量出口"})
	}
	m.state.Routes = routes
	m.state.Revision = policy.Revision
	// A compromised/stale Controller or an old API cannot raise the local
	// machine's hard ceiling. Never launch a by-name engine when over quota.
	limit := policy.MaxProxyPIDs
	if limit <= 0 || limit > control.AbsoluteMaxProxyPIDs {
		limit = control.DefaultMaxProxyPIDs
	}
	if m.cfg.PIDHardLimit > 0 && m.cfg.PIDHardLimit < limit {
		limit = m.cfg.PIDHardLimit
	}
	m.state.MaxProxyPIDs = limit
	distinct := make(map[int]bool, len(policy.Bindings))
	for _, b := range policy.Bindings {
		distinct[b.PID] = true
	}
	if len(policy.Bindings) > limit || len(distinct) != len(policy.Bindings) {
		m.stopLocked()
		m.state.Error = fmt.Sprintf("pid_limit_exceeded: request %d PID(s), machine limit %d", len(policy.Bindings), limit)
		for i := range m.state.Routes {
			m.state.Routes[i].Status = "pid_limit_exceeded"
			m.state.Routes[i].Detail = "当前机器代理 PID 配额超限，已拒绝启动代理引擎"
		}
		return nil
	}
	if len(policy.Bindings) == 0 {
		m.stopLocked()
		m.state.Error = ""
		return nil
	}
	if m.cfg.Engine == "observe" {
		m.stopLocked()
		m.state.Error = ""
		// Observe mode deliberately performs no transparent proxy or routing.
		live := map[string]bool{}
		for _, p := range m.all {
			live[fmt.Sprintf("%d|%s", p.PID, p.StartTime)] = true
		}
		for i, b := range policy.Bindings {
			if !live[fmt.Sprintf("%d|%s", b.PID, b.StartTime)] {
				m.state.Routes[i].Status = "stale_pid"
				m.state.Routes[i].Detail = "进程已经退出或 PID 已重用"
			}
		}
		return nil
	}
	raw, err := BuildProfile(*policy, m.all)
	if err != nil {
		m.stopLocked()
		m.state.Error = err.Error()
		for i := range m.state.Routes {
			m.state.Routes[i].Status = "cannot_enforce"
			m.state.Routes[i].Detail = err.Error()
		}
		return nil // Report applied intent but never claim enforcement.
	}
	if runtime.GOOS != "windows" {
		m.stopLocked()
		m.state.Error = "ProxyBridge client engine supports Windows only"
		return nil
	}
	if m.cfg.ProxyBridgeCLI == "" {
		m.stopLocked()
		m.state.Error = "NODE_MANAGER_PROXYBRIDGE_CLI not configured"
		return nil
	}
	hash := sha256.Sum256(raw)
	signature := hex.EncodeToString(hash[:])
	if m.command != nil && m.fingerprint == signature {
		for i := range m.state.Routes {
			m.state.Routes[i].Status = "process_name_unverified"
			m.state.Routes[i].Detail = "ProxyBridge 进程名规则已启动；实际游戏流量出口未确认"
		}
		return nil
	}
	// Change policy only after the complete profile is validated. A PID-only
	// dispatch is never silently downgraded into unsafe by-name routing.
	m.stopLocked()
	if err := m.startLocked(raw, signature); err != nil {
		m.state.Error = err.Error()
		for i := range m.state.Routes {
			m.state.Routes[i].Status = "engine_error"
			m.state.Routes[i].Detail = err.Error()
		}
		return nil
	}
	m.state.Error = ""
	for i := range m.state.Routes {
		m.state.Routes[i].Status = "process_name_unverified"
		m.state.Routes[i].Detail = "ProxyBridge 进程名规则已启动；实际游戏流量出口未确认"
	}
	return nil
}
func (m *Manager) startLocked(profile []byte, fingerprint string) error {
	exe, err := filepath.Abs(m.cfg.ProxyBridgeCLI)
	if err != nil {
		return err
	}
	if filepath.Ext(exe) != ".exe" {
		return errors.New("ProxyBridge CLI must be an explicitly selected .exe")
	}
	if _, err := os.Stat(exe); err != nil {
		return fmt.Errorf("ProxyBridge CLI unavailable: %w", err)
	}
	f, err := os.CreateTemp(m.cfg.DataDir, ".proxybridge-*.pbprofile")
	if err != nil {
		return err
	}
	filename := f.Name()
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(profile)
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		os.Remove(filename)
		return err
	}
	cmd := exec.Command(exe, "--profile", filename, "--verbose", "0")
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err = cmd.Start(); err != nil {
		os.Remove(filename)
		return err
	}
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	m.command = cmd
	m.exited = exited
	m.profile = filename
	m.fingerprint = fingerprint
	return nil
}
func (m *Manager) stopLocked() {
	if m.command != nil {
		_ = m.command.Process.Kill()
		if m.exited != nil {
			select {
			case <-m.exited:
			case <-time.After(3 * time.Second):
			}
		}
	}
	m.command = nil
	m.exited = nil
	m.fingerprint = ""
	if m.profile != "" {
		_ = os.Remove(m.profile)
		m.profile = ""
	}
}

// Suspend stops the managed proxy executable immediately when Controller
// authorization or heartbeat is unavailable. This is NOT a network firewall:
// unmanaged applications may still connect directly. We report this fact
// rather than claiming fail-closed packet isolation.
func (m *Manager) Suspend(reason string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopLocked()
	m.state.Error = "controller_unreachable: " + reason
	for i := range m.state.Routes {
		m.state.Routes[i].Status = "controller_unreachable"
		m.state.Routes[i].Detail = "已停止 NodeSwarm 托管代理；Windows 未阻止应用直连，请检查网络"
	}
}

func (m *Manager) Stop() { m.mu.Lock(); defer m.mu.Unlock(); m.stopLocked() }

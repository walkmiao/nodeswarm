// Package desktopcore is the local, window-independent application service.
// It has no HTTP listener or admin token: Wails binds these methods directly
// into an installed Windows desktop app using the native WebView2 bridge.
package desktopcore

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/walkmiao/nodeswarm/internal/agent"
	"github.com/walkmiao/nodeswarm/internal/control"
	"github.com/walkmiao/nodeswarm/internal/identity"
	"github.com/walkmiao/nodeswarm/internal/manager"
)

type Settings struct {
	Controller     string `json:"controller"`
	Name           string `json:"name"`
	Targets        string `json:"targets"`
	Engine         string `json:"engine"`
	ProxyBridgeCLI string `json:"proxyBridgeCli"`
	WinDivertDir   string `json:"winDivertDir"`
	PIDHardLimit   int    `json:"pidHardLimit"`
	AutoConnect    bool   `json:"autoConnect"`
}

type State struct {
	Settings        Settings               `json:"settings"`
	Connected       bool                   `json:"connected"`
	Running         bool                   `json:"running"`
	Status          string                 `json:"status"`
	LastError       string                 `json:"lastError"`
	LastSync        string                 `json:"lastSync"`
	CredentialReady bool                   `json:"credentialReady"`
	Client          control.ClientSnapshot `json:"client"`
	Remote          control.AgentView      `json:"remote"`
	Nodes           []manager.Node         `json:"nodes"`
	Logs            []string               `json:"logs"`
}

type credentials = identity.Credentials

type Service struct {
	mu              sync.Mutex
	dir             string
	settings        Settings
	status          State
	cancel          context.CancelFunc
	stopped         chan struct{}
	httpClient      *http.Client
	remoteRefreshed time.Time
}

func Defaults() Settings {
	return Settings{Targets: "Client.exe", Engine: "observe", AutoConnect: true}
}

func ValidateSettings(v Settings) error {
	u, err := url.Parse(strings.TrimSpace(v.Controller))
	if err != nil || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" {
		return errors.New("请输入有效的主控地址，例如 https://console.example.com")
	}
	if u.Scheme != "https" {
		ip := net.ParseIP(u.Hostname())
		if u.Scheme != "http" || !(u.Hostname() == "localhost" || (ip != nil && ip.IsLoopback())) {
			return errors.New("远程主控必须使用 HTTPS（仅本机环回地址允许 HTTP）")
		}
	}
	if strings.TrimSpace(v.Name) == "" || len(v.Name) > 80 {
		return errors.New("本机名称不能为空，最长 80 字符")
	}
	if v.PIDHardLimit < 0 || v.PIDHardLimit > control.AbsoluteMaxProxyPIDs {
		return fmt.Errorf("PID 本机上限需为 0-%d（0 表示跟随主控）", control.AbsoluteMaxProxyPIDs)
	}
	if _, err := managerClientConfig(v); err != nil {
		return err
	}
	return nil
}

func managerClientConfig(v Settings) (*managerClientConfigValue, error) {
	// Validation without needing the actual proxy executable or a running agent.
	if v.Engine != "observe" && v.Engine != "proxybridge" && v.Engine != "pidtcp" {
		return nil, errors.New("运行模式仅支持 observe、proxybridge 或 pidtcp（实验）")
	}
	targets := map[string]bool{}
	for _, raw := range strings.Split(v.Targets, ";") {
		s := strings.ToLower(strings.TrimSpace(raw))
		if s == "" {
			continue
		}
		if s == "*" || strings.ContainsAny(s, "/\\\r\n*?") || len(s) > 100 {
			return nil, errors.New("无效的目标进程名")
		}
		targets[s] = true
	}
	if len(targets) == 0 || len(targets) > 12 {
		return nil, errors.New("请输入 1-12 个 EXE 进程名（以分号分隔）")
	}
	return &managerClientConfigValue{targets: targets}, nil
}

type managerClientConfigValue struct{ targets map[string]bool }

func New(dir string) (*Service, error) {
	if dir == "" {
		return nil, errors.New("desktop data directory required")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	s := &Service{dir: dir, settings: Defaults(), httpClient: &http.Client{Timeout: 8 * time.Second}}
	buf, err := os.ReadFile(filepath.Join(dir, "settings.json"))
	if err == nil {
		if err := json.Unmarshal(buf, &s.settings); err != nil {
			return nil, fmt.Errorf("invalid desktop settings: %w", err)
		}
		if err := ValidateSettings(s.settings); err != nil {
			return nil, fmt.Errorf("invalid saved settings: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	s.status = State{Settings: s.settings, Status: "disconnected", Nodes: []manager.Node{}, Logs: []string{}, Client: control.ClientSnapshot{Processes: []control.ClientProcess{}, Routes: []control.ClientRoute{}}}
	s.status.CredentialReady = s.credentialReadyLocked()
	return s, nil
}

func (s *Service) agentDir() string { return filepath.Join(s.dir, "agent") }

func (s *Service) readCredentials() (credentials, error) {
	c, err := identity.Load(s.agentDir())
	if err != nil {
		return c, err
	}
	if c.Role != "client" && c.Role != "" {
		return credentials{}, errors.New("当前设备凭据的角色无效")
	}
	return c, nil
}
func (s *Service) credentialReadyLocked() bool {
	c, err := s.readCredentials()
	return err == nil && (s.settings.Controller == "" || c.Controller == s.settings.Controller)
}
func (s *Service) logLocked(msg string) {
	stamp := time.Now().Format("15:04:05")
	s.status.Logs = append(s.status.Logs, stamp+"  "+msg)
	if len(s.status.Logs) > 80 {
		s.status.Logs = s.status.Logs[len(s.status.Logs)-80:]
	}
}
func (s *Service) ClearLogs() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status.Logs = []string{}
}

func (s *Service) GetState() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.status
	out.Settings = s.settings
	out.Client.Processes = slices.Clone(s.status.Client.Processes)
	out.Client.Routes = slices.Clone(s.status.Client.Routes)
	out.Nodes = slices.Clone(s.status.Nodes)
	out.Logs = slices.Clone(s.status.Logs)
	out.Remote.ClientBindings = slices.Clone(s.status.Remote.ClientBindings)
	out.CredentialReady = s.credentialReadyLocked()
	return out
}

func (s *Service) SaveSettings(v Settings) error {
	v.Controller = strings.TrimRight(strings.TrimSpace(v.Controller), "/")
	v.Name = strings.TrimSpace(v.Name)
	v.Targets = strings.TrimSpace(v.Targets)
	if err := ValidateSettings(v); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		return errors.New("请先断开当前连接，再修改配置")
	}
	c, err := s.readCredentials()
	if err == nil && c.Controller != v.Controller {
		return errors.New("已注册的设备不能直接更换主控地址，请使用独立数据目录重新注册")
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(s.dir, ".settings-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err = tmp.Chmod(0600); err == nil {
		_, err = tmp.Write(data)
	}
	if e := tmp.Close(); err == nil {
		err = e
	}
	if err != nil {
		return err
	}
	if err = os.Rename(tmp.Name(), filepath.Join(s.dir, "settings.json")); err != nil {
		return err
	}
	s.settings = v
	s.status.Settings = v
	s.logLocked("本机设置已保存")
	return nil
}
func (s *Service) Start(enrollKey string) error {
	s.mu.Lock()
	if s.cancel != nil {
		s.mu.Unlock()
		return errors.New("客户端已启动")
	}
	cfg := s.settings
	if err := ValidateSettings(cfg); err != nil {
		s.mu.Unlock()
		return err
	}
	if !s.credentialReadyLocked() && strings.TrimSpace(enrollKey) == "" {
		s.mu.Unlock()
		return errors.New("首次连接请输入由管理员签发的一次性注册令牌")
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	s.cancel = cancel
	s.stopped = done
	s.status.Running = true
	s.status.Connected = false
	s.status.Status = "connecting"
	s.status.LastError = ""
	s.logLocked("正在连接主控并启动本机进程监测")
	s.mu.Unlock()
	go func() {
		defer close(done)
		runErr := agent.Run(ctx, agent.Config{
			Role: "client", Controller: cfg.Controller, Name: cfg.Name, DataDir: s.agentDir(), EnrollKey: strings.TrimSpace(enrollKey),
			ClientEngine: cfg.Engine, ClientTargets: cfg.Targets, ProxyBridgeCLI: cfg.ProxyBridgeCLI, ClientWinDivertDir: cfg.WinDivertDir, ClientPIDHardLimit: cfg.PIDHardLimit,
			OnConnection: func(ok bool, message string) {
				s.mu.Lock()
				defer s.mu.Unlock()
				if !s.status.Running {
					return
				}
				wasConnected := s.status.Connected
				s.status.Connected = ok
				if ok {
					s.status.Status = "online"
					s.status.LastError = ""
					s.status.LastSync = time.Now().UTC().Format(time.RFC3339)
					if !wasConnected {
						s.logLocked("主控连接已建立")
					}
				}
				if !ok {
					s.status.Status = "reconnecting"
					s.status.LastError = message
					if wasConnected {
						s.logLocked("主控暂时不可达，等待恢复")
					}
				}
			},
			OnClientReport: func(snapshot control.ClientSnapshot) {
				s.mu.Lock()
				s.status.Client = snapshot
				s.mu.Unlock()
			},
		})
		s.mu.Lock()
		defer s.mu.Unlock()
		s.cancel = nil
		s.stopped = nil
		s.status.Running = false
		s.status.Connected = false
		if runErr != nil && ctx.Err() == nil {
			s.status.Status = "error"
			s.status.LastError = runErr.Error()
			s.logLocked("客户端停止: " + runErr.Error())
		} else {
			s.status.Status = "disconnected"
			s.logLocked("客户端已停止")
		}
	}()
	return nil
}
func (s *Service) Stop() error {
	s.mu.Lock()
	cancel := s.cancel
	done := s.stopped
	s.mu.Unlock()
	if cancel == nil {
		return nil
	}
	cancel()
	if done != nil {
		select {
		case <-done:
		case <-time.After(9 * time.Second):
			return errors.New("本机代理正在退出，请稍后重试")
		}
	}
	return nil
}

func (s *Service) deviceRequest(ctx context.Context, method, path string, payload any, target any) error {
	c, err := s.readCredentials()
	if err != nil {
		return errors.New("设备尚未注册，请先连接主控")
	}
	s.mu.Lock()
	controller := s.settings.Controller
	s.mu.Unlock()
	if c.Controller != controller {
		return errors.New("设备凭据与当前主控地址不匹配")
	}
	var rdr io.Reader
	if payload != nil {
		b, e := json.Marshal(payload)
		if e != nil {
			return e
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, controller+path, rdr)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var body struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(b, &body) == nil && body.Error != "" {
			return fmt.Errorf("HTTP %d: %s", resp.StatusCode, body.Error)
		}
		return fmt.Errorf("Controller returned HTTP %d", resp.StatusCode)
	}
	if target != nil {
		return json.Unmarshal(b, target)
	}
	return nil
}
func (s *Service) Refresh() error {
	c, err := s.readCredentials()
	if err != nil {
		return errors.New("尚未完成首次注册")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	var data struct {
		Client control.AgentView `json:"client"`
		Nodes  []manager.Node    `json:"nodes"`
	}
	if err = s.deviceRequest(ctx, "GET", "/api/agent/client/context?id="+url.QueryEscape(c.ID), nil, &data); err != nil {
		s.mu.Lock()
		s.status.LastError = err.Error()
		s.mu.Unlock()
		return err
	}
	s.mu.Lock()
	s.status.Remote = data.Client
	s.status.Nodes = data.Nodes
	if s.status.Nodes == nil {
		s.status.Nodes = []manager.Node{}
	}
	s.remoteRefreshed = time.Now()
	s.mu.Unlock()
	return nil
}
func (s *Service) SendBindings(requested []control.ClientBinding) error {
	if len(requested) > control.AbsoluteMaxProxyPIDs {
		return errors.New("超出最大 PID 数量")
	}
	s.mu.Lock()
	last := s.remoteRefreshed
	s.mu.Unlock()
	if time.Since(last) > 5*time.Second {
		if err := s.Refresh(); err != nil {
			return err
		}
	}
	s.mu.Lock()
	remote := s.status.Remote
	cfg := s.settings
	s.mu.Unlock()
	effective := remote.MaxProxyPIDs
	if effective == 0 {
		effective = control.DefaultMaxProxyPIDs
	}
	if cfg.PIDHardLimit > 0 && cfg.PIDHardLimit < effective {
		effective = cfg.PIDHardLimit
	}
	if len(requested) > effective {
		return fmt.Errorf("超过当前机器 PID 上限 %d", effective)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	c, err := s.readCredentials()
	if err != nil {
		return err
	}
	var resp struct {
		Client control.AgentView `json:"client"`
	}
	payload := map[string]any{"id": c.ID, "expectedRevision": remote.Revision, "bindings": requested}
	if err = s.deviceRequest(ctx, "PUT", "/api/agent/client/bindings", payload, &resp); err != nil {
		return err
	}
	s.mu.Lock()
	s.status.Remote = resp.Client
	s.remoteRefreshed = time.Now()
	s.logLocked(fmt.Sprintf("绑定策略更新成功：%d 个 PID，修订 %d", len(resp.Client.ClientBindings), resp.Client.Revision))
	s.mu.Unlock()
	return nil
}
func (s *Service) Assign(pid int, startTime, nodeID string) error {
	if err := s.Refresh(); err != nil {
		return err
	}
	s.mu.Lock()
	remote := s.status.Remote
	nodes := slices.Clone(s.status.Nodes)
	s.mu.Unlock()
	found := false
	for _, n := range nodes {
		if n.ID == nodeID && manager.NodeAvailable(n, 0, time.Now()) {
			found = true
			break
		}
	}
	if !found {
		return errors.New("代理节点未启用、已过期或不可用")
	}
	var proc control.ClientProcess
	for _, p := range remote.Client.Processes {
		if p.PID == pid && p.StartTime == startTime {
			proc = p
			break
		}
	}
	if proc.PID == 0 {
		return errors.New("PID 已退出、重启或不属于当前设备，请刷新")
	}
	bindings := slices.Clone(remote.ClientBindings)
	replaced := false
	for i, b := range bindings {
		if b.PID == pid {
			bindings[i] = control.ClientBinding{PID: pid, Name: proc.Name, StartTime: startTime, NodeID: nodeID}
			replaced = true
		}
	}
	if !replaced {
		bindings = append(bindings, control.ClientBinding{PID: pid, Name: proc.Name, StartTime: startTime, NodeID: nodeID})
	}
	return s.SendBindings(bindings)
}
func (s *Service) Unassign(pid int, startTime string) error {
	if err := s.Refresh(); err != nil {
		return err
	}
	s.mu.Lock()
	bindings := slices.Clone(s.status.Remote.ClientBindings)
	s.mu.Unlock()
	bindings = slices.DeleteFunc(bindings, func(b control.ClientBinding) bool { return b.PID == pid && b.StartTime == startTime })
	return s.SendBindings(bindings)
}
func (s *Service) AutoFill() (int, error) {
	if err := s.Refresh(); err != nil {
		return 0, err
	}
	s.mu.Lock()
	remote := s.status.Remote
	nodes := slices.Clone(s.status.Nodes)
	cfg := s.settings
	s.mu.Unlock()
	maxPIDs := remote.MaxProxyPIDs
	if maxPIDs == 0 {
		maxPIDs = control.DefaultMaxProxyPIDs
	}
	if cfg.PIDHardLimit > 0 && cfg.PIDHardLimit < maxPIDs {
		maxPIDs = cfg.PIDHardLimit
	}
	bindings := slices.Clone(remote.ClientBindings)
	used := map[string]int{}
	bound := map[int]bool{}
	for _, b := range bindings {
		used[b.NodeID]++
		bound[b.PID] = true
	}
	count := 0
	for _, p := range remote.Client.Processes {
		if len(bindings) >= maxPIDs {
			break
		}
		if bound[p.PID] {
			continue
		}
		var best manager.Node
		selected := false
		for _, n := range nodes {
			if !manager.NodeAvailable(n, used[n.ID], time.Now()) || n.LastStatus == "failed" || n.LastStatus == "error" {
				continue
			}
			if !selected || used[n.ID]*best.Capacity < used[best.ID]*n.Capacity {
				best = n
				selected = true
			}
		}
		if !selected {
			break
		}
		bindings = append(bindings, control.ClientBinding{PID: p.PID, StartTime: p.StartTime, Name: p.Name, NodeID: best.ID})
		bound[p.PID] = true
		used[best.ID]++
		count++
	}
	if count == 0 {
		return 0, nil
	}
	if err := s.SendBindings(bindings); err != nil {
		return 0, err
	}
	return count, nil
}

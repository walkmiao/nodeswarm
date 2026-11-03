package control

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
)

const AgentVersion = "1.1.0"

type ProxyConfig struct {
	Enabled        bool     `json:"enabled"`
	Listen         string   `json:"listen"`
	PublicHost     string   `json:"publicHost"`
	Port           int      `json:"port"`
	Username       string   `json:"username"`
	Password       string   `json:"password,omitempty"`
	HasPassword    bool     `json:"hasPassword"`
	UDP            bool     `json:"udp"`
	UDPPortStart   int      `json:"udpPortStart"`
	UDPPortEnd     int      `json:"udpPortEnd"`
	MaxConnections int      `json:"maxConnections"`
	Capacity       int      `json:"capacity"`
	Provider       string   `json:"provider"`
	Region         string   `json:"region"`
	AllowedCIDRs   []string `json:"allowedCidrs"`
}
type Metrics struct {
	CPU           float64 `json:"cpu"`
	MemoryUsed    uint64  `json:"memoryUsed"`
	MemoryTotal   uint64  `json:"memoryTotal"`
	Cores         int     `json:"cores"`
	UptimeSeconds uint64  `json:"uptimeSeconds"`
	At            string  `json:"at"`
}
type ProxyRuntime struct {
	Status           string `json:"status"`
	Error            string `json:"error,omitempty"`
	Address          string `json:"address,omitempty"`
	Connections      int64  `json:"connections"`
	TotalConnections uint64 `json:"totalConnections"`
	BytesUp          uint64 `json:"bytesUp"`
	BytesDown        uint64 `json:"bytesDown"`
	StartedAt        string `json:"startedAt,omitempty"`
	Revision         int64  `json:"revision"`
}
type AgentLog struct {
	ID      string `json:"id"`
	At      string `json:"at"`
	Level   string `json:"level"`
	Message string `json:"message"`
}
type AgentView struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	Hostname     string       `json:"hostname"`
	OS           string       `json:"os"`
	Arch         string       `json:"arch"`
	Version      string       `json:"version"`
	ObservedIP   string       `json:"observedIp"`
	Status       string       `json:"status"`
	RegisteredAt string       `json:"registeredAt"`
	DesiredAt    string       `json:"desiredAt,omitempty"`
	LastSeen     string       `json:"lastSeen"`
	LastError    string       `json:"lastError"`
	Desired      ProxyConfig  `json:"desired"`
	Revision     int64        `json:"revision"`
	Runtime      ProxyRuntime `json:"runtime"`
	Metrics      Metrics      `json:"metrics"`
	History      []Metrics    `json:"history"`
	Logs         []AgentLog   `json:"logs"`
}
type ConfigureInput struct {
	Name  string      `json:"name"`
	Proxy ProxyConfig `json:"proxy"`
}
type EnrollInput struct {
	Name     string `json:"name"`
	Hostname string `json:"hostname"`
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	Version  string `json:"version"`
}
type Heartbeat struct {
	ID       string       `json:"id"`
	Hostname string       `json:"hostname"`
	OS       string       `json:"os"`
	Arch     string       `json:"arch"`
	Version  string       `json:"version"`
	Metrics  Metrics      `json:"metrics"`
	Runtime  ProxyRuntime `json:"runtime"`
	Logs     []AgentLog   `json:"logs"`
}
type HeartbeatResponse struct {
	Revision   int64       `json:"revision"`
	Proxy      ProxyConfig `json:"proxy"`
	ServerTime string      `json:"serverTime"`
}

type agentDisk struct {
	AgentView
	TokenHash string `json:"tokenHash"`
	Secret    string `json:"secret,omitempty"`
}
type EnrollmentGrant struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	ExpiresAt string `json:"expiresAt"`
	CreatedAt string `json:"createdAt"`
}
type enrollmentDisk struct {
	EnrollmentGrant
	TokenHash string `json:"tokenHash"`
}
type disk struct {
	Agents      []agentDisk      `json:"agents"`
	Enrollments []enrollmentDisk `json:"enrollments,omitempty"`
}
type Registry struct {
	mu          sync.Mutex
	path        string
	key         []byte
	agents      []agentDisk
	enrollments []enrollmentDisk
	lastSaved   time.Time
}

func NewRegistry(dir string) (*Registry, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	keyFile := filepath.Join(dir, "registry.key")
	key, err := os.ReadFile(keyFile)
	if errors.Is(err, os.ErrNotExist) {
		key = make([]byte, 32)
		if _, err = rand.Read(key); err != nil {
			return nil, err
		}
		if err = os.WriteFile(keyFile, key, 0600); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}
	if len(key) != 32 {
		return nil, errors.New("invalid registry encryption key")
	}
	s := &Registry{path: filepath.Join(dir, "agents.json"), key: key, agents: []agentDisk{}}
	b, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	var d disk
	if err = json.Unmarshal(b, &d); err != nil {
		return nil, err
	}
	s.agents = d.Agents
	s.enrollments = d.Enrollments
	for i := range s.agents {
		if s.agents[i].Secret != "" {
			p, e := unseal(key, s.agents[i].Secret)
			if e != nil {
				return nil, fmt.Errorf("agent %s credential: %w", s.agents[i].ID, e)
			}
			s.agents[i].Desired.Password = p
			s.agents[i].Desired.HasPassword = p != ""
		}
	}
	return s, nil
}
func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
func SHA(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }
func seal(key []byte, plaintext string) (string, error) {
	b, e := aes.NewCipher(key)
	if e != nil {
		return "", e
	}
	g, e := cipher.NewGCM(b)
	if e != nil {
		return "", e
	}
	nonce := make([]byte, g.NonceSize())
	if _, e = rand.Read(nonce); e != nil {
		return "", e
	}
	return hex.EncodeToString(g.Seal(nonce, nonce, []byte(plaintext), nil)), nil
}
func unseal(key []byte, coded string) (string, error) {
	raw, e := hex.DecodeString(coded)
	if e != nil {
		return "", e
	}
	b, e := aes.NewCipher(key)
	if e != nil {
		return "", e
	}
	g, e := cipher.NewGCM(b)
	if e != nil {
		return "", e
	}
	if len(raw) < g.NonceSize() {
		return "", errors.New("encrypted credential is truncated")
	}
	v, e := g.Open(nil, raw[:g.NonceSize()], raw[g.NonceSize():], nil)
	return string(v), e
}
func (s *Registry) saveLocked() error {
	d := disk{Agents: make([]agentDisk, 0, len(s.agents)), Enrollments: slices.Clone(s.enrollments)}
	for _, a := range s.agents {
		v := a
		v.Desired.Password = ""
		v.Secret = ""
		if a.Desired.Password != "" {
			ciphertext, e := seal(s.key, a.Desired.Password)
			if e != nil {
				return e
			}
			v.Secret = ciphertext
		}
		d.Agents = append(d.Agents, v)
	}
	b, e := json.MarshalIndent(d, "", "  ")
	if e != nil {
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(s.path), ".agents-*.tmp")
	if e != nil {
		return e
	}
	defer os.Remove(f.Name())
	if e = f.Chmod(0600); e != nil {
		f.Close()
		return e
	}
	if _, e = f.Write(b); e != nil {
		f.Close()
		return e
	}
	if e = f.Sync(); e != nil {
		f.Close()
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	if e = os.Rename(f.Name(), s.path); e == nil {
		s.lastSaved = time.Now()
	}
	return e
}
func safeView(a agentDisk) AgentView {
	v := a.AgentView
	v.Desired.Password = ""
	v.Desired.HasPassword = a.Desired.Password != ""
	v.History = slices.Clone(v.History)
	v.Logs = slices.Clone(v.Logs)
	v.Desired.AllowedCIDRs = slices.Clone(v.Desired.AllowedCIDRs)
	if v.LastSeen == "" {
		v.Status = "pending"
	} else if t, e := time.Parse(time.RFC3339, v.LastSeen); e == nil && time.Since(t) < 32*time.Second {
		v.Status = "online"
	} else {
		v.Status = "offline"
	}
	return v
}
func (s *Registry) List() []AgentView {
	s.mu.Lock()
	defer s.mu.Unlock()
	v := make([]AgentView, 0, len(s.agents))
	for _, a := range s.agents {
		v = append(v, safeView(a))
	}
	return v
}
func (s *Registry) Find(id string) (AgentView, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, a := range s.agents {
		if a.ID == id {
			return safeView(a), true
		}
	}
	return AgentView{}, false
}
func (s *Registry) Credentials(id string) (ProxyConfig, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, a := range s.agents {
		if a.ID == id {
			return a.Desired, true
		}
	}
	return ProxyConfig{}, false
}
func clean(v string, n int) string {
	v = strings.TrimSpace(v)
	if len(v) > n {
		v = v[:n]
	}
	return v
}
func (s *Registry) enrollLocked(in EnrollInput, observed string) (string, string, error) {
	if strings.TrimSpace(in.Name) == "" || len(in.Name) > 80 {
		return "", "", errors.New("agent name required (max 80 chars)")
	}
	id := randomHex(12)
	token := randomHex(32)
	now := time.Now().UTC().Format(time.RFC3339)
	a := agentDisk{AgentView: AgentView{ID: id, Name: clean(in.Name, 80), Hostname: clean(in.Hostname, 160), OS: clean(in.OS, 24), Arch: clean(in.Arch, 24), Version: clean(in.Version, 30), ObservedIP: observed, RegisteredAt: now, Status: "pending", Desired: ProxyConfig{Listen: "0.0.0.0", Port: 1080, MaxConnections: 200, Capacity: 8, UDP: true}, History: []Metrics{}, Logs: []AgentLog{}}, TokenHash: SHA(token)}
	s.agents = append(s.agents, a)
	if err := s.saveLocked(); err != nil {
		s.agents = s.agents[:len(s.agents)-1]
		return "", "", err
	}
	return id, token, nil
}

// Enroll is retained for administrators using the original shared enrollment secret.
func (s *Registry) Enroll(in EnrollInput, observed string) (string, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.enrollLocked(in, observed)
}

// CreateEnrollment issues a limited-life, single-use agent bootstrap key. The
// raw secret is returned once, while only its SHA-256 hash is stored.
func (s *Registry) CreateEnrollment(label string, ttlMinutes int) (EnrollmentGrant, string, error) {
	label = strings.TrimSpace(label)
	if label == "" || len(label) > 80 || ttlMinutes < 1 || ttlMinutes > 1440 {
		return EnrollmentGrant{}, "", errors.New("label required and ttlMinutes must be 1-1440")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	previous := slices.Clone(s.enrollments)
	now := time.Now().UTC()
	s.enrollments = slices.DeleteFunc(s.enrollments, func(g enrollmentDisk) bool {
		exp, err := time.Parse(time.RFC3339, g.ExpiresAt)
		return err != nil || !now.Before(exp)
	})
	if len(s.enrollments) >= 100 {
		s.enrollments = previous
		return EnrollmentGrant{}, "", errors.New("enrollment grant limit reached; revoke unused grants")
	}
	grant := EnrollmentGrant{ID: randomHex(10), Label: label, CreatedAt: now.Format(time.RFC3339), ExpiresAt: now.Add(time.Duration(ttlMinutes) * time.Minute).Format(time.RFC3339)}
	token := randomHex(32)
	s.enrollments = append(s.enrollments, enrollmentDisk{EnrollmentGrant: grant, TokenHash: SHA(token)})
	if err := s.saveLocked(); err != nil {
		s.enrollments = previous
		return EnrollmentGrant{}, "", err
	}
	return grant, token, nil
}
func (s *Registry) ListEnrollments() []EnrollmentGrant {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]EnrollmentGrant, 0, len(s.enrollments))
	for _, g := range s.enrollments {
		out = append(out, g.EnrollmentGrant)
	}
	return out
}
func (s *Registry) RevokeEnrollment(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, g := range s.enrollments {
		if g.ID == id {
			before := slices.Clone(s.enrollments)
			s.enrollments = slices.Delete(s.enrollments, i, i+1)
			if err := s.saveLocked(); err != nil {
				s.enrollments = before
				return err
			}
			return nil
		}
	}
	return errors.New("enrollment grant not found")
}
func (s *Registry) EnrollWithGrant(token string, in EnrollInput, observed string) (string, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if token == "" || strings.TrimSpace(in.Name) == "" || len(in.Name) > 80 {
		return "", "", errors.New("invalid enrollment key or agent name")
	}
	hash := SHA(token)
	now := time.Now()
	for i, g := range s.enrollments {
		expires, e := time.Parse(time.RFC3339, g.ExpiresAt)
		if e != nil || now.After(expires) {
			continue
		}
		if subtle.ConstantTimeCompare([]byte(hash), []byte(g.TokenHash)) != 1 {
			continue
		}
		old := slices.Clone(s.enrollments)
		s.enrollments = slices.Delete(s.enrollments, i, i+1)
		id, agentToken, err := s.enrollLocked(in, observed)
		if err != nil {
			s.enrollments = old
		}
		return id, agentToken, err
	}
	return "", "", errors.New("enrollment key invalid, expired or already used")
}

func (s *Registry) Authorize(id, token string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	hash := SHA(token)
	for _, a := range s.agents {
		if a.ID == id {
			return subtle.ConstantTimeCompare([]byte(a.TokenHash), []byte(hash)) == 1
		}
	}
	return false
}
func (s *Registry) Report(in Heartbeat, observed string) (HeartbeatResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.agents {
		a := &s.agents[i]
		if a.ID != in.ID {
			continue
		}
		a.LastSeen = time.Now().UTC().Format(time.RFC3339)
		a.ObservedIP = observed
		a.Hostname = clean(in.Hostname, 160)
		a.OS = clean(in.OS, 24)
		a.Arch = clean(in.Arch, 24)
		a.Version = clean(in.Version, 30)
		a.Metrics = in.Metrics
		a.Runtime = in.Runtime
		a.LastError = clean(in.Runtime.Error, 512)
		if in.Metrics.At != "" {
			a.History = append(a.History, in.Metrics)
			if len(a.History) > 48 {
				a.History = a.History[len(a.History)-48:]
			}
		}
		seen := make(map[string]bool)
		for _, l := range a.Logs {
			seen[l.ID] = true
		}
		for _, l := range in.Logs {
			if len(l.ID) > 0 && !seen[l.ID] {
				l.Message = clean(l.Message, 360)
				a.Logs = append(a.Logs, l)
				seen[l.ID] = true
			}
		}
		if len(a.Logs) > 80 {
			a.Logs = a.Logs[len(a.Logs)-80:]
		}
		if time.Since(s.lastSaved) > 22*time.Second {
			_ = s.saveLocked()
		}
		return HeartbeatResponse{Revision: a.Revision, Proxy: a.Desired, ServerTime: time.Now().UTC().Format(time.RFC3339)}, nil
	}
	return HeartbeatResponse{}, errors.New("agent not found")
}

var hostnameRE = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9.:-]{0,253}$`)

func ValidateConfig(c ProxyConfig) error {
	if c.Listen != "" && net.ParseIP(c.Listen) == nil {
		return errors.New("监听地址必须是 IP，通常为 0.0.0.0")
	}
	if c.Port < 1 || c.Port > 65535 {
		return errors.New("SOCKS5 port must be 1-65535")
	}
	if c.PublicHost != "" && net.ParseIP(c.PublicHost) == nil && !hostnameRE.MatchString(c.PublicHost) {
		return errors.New("invalid public host")
	}
	if (c.UDPPortStart == 0) != (c.UDPPortEnd == 0) {
		return errors.New("UDP 端口池需同时设置起始和结束端口")
	}
	if c.UDPPortStart != 0 && (c.UDPPortStart < 1024 || c.UDPPortEnd > 65535 || c.UDPPortEnd < c.UDPPortStart || c.UDPPortEnd-c.UDPPortStart+1 > 128) {
		return errors.New("UDP 端口池须在 1024-65535 范围内，且最多 128 个端口")
	}
	if c.MaxConnections < 1 || c.MaxConnections > 4000 {
		return errors.New("maxConnections must be 1-4000")
	}
	if c.Capacity < 1 || c.Capacity > 1000 {
		return errors.New("capacity must be 1-1000")
	}
	if len(c.Username) > 255 || len(c.Password) > 255 || len(c.Username) == 0 {
		return errors.New("SOCKS5 requires a username (1-255 chars) and password (1-255 chars)")
	}
	if len(c.Password) == 0 {
		return errors.New("SOCKS5 requires nonempty password")
	}
	if len(c.Provider) > 80 || len(c.Region) > 80 {
		return errors.New("provider/region too long")
	}
	if len(c.AllowedCIDRs) > 24 {
		return errors.New("too many allowed CIDRs")
	}
	for _, cidr := range c.AllowedCIDRs {
		if _, _, err := net.ParseCIDR(cidr); err != nil {
			return fmt.Errorf("invalid allow CIDR %q", cidr)
		}
	}
	return nil
}
func (s *Registry) Configure(id string, in ConfigureInput) (AgentView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.agents {
		a := &s.agents[i]
		if a.ID != id {
			continue
		}
		c := in.Proxy
		if c.Password == "" && c.Username == a.Desired.Username {
			c.Password = a.Desired.Password
		}
		if c.Username == "" {
			c.Username = "node"
		}
		if c.Password == "" {
			c.Password = randomHex(18)
		}
		if c.Listen == "" {
			c.Listen = "0.0.0.0"
		}
		if c.Port == 0 {
			c.Port = 1080
		}
		if c.MaxConnections == 0 {
			c.MaxConnections = 200
		}
		if c.Capacity == 0 {
			c.Capacity = 8
		}
		if err := ValidateConfig(c); err != nil {
			return AgentView{}, err
		}
		before := *a
		if strings.TrimSpace(in.Name) != "" {
			a.Name = clean(in.Name, 80)
		}
		a.Desired = c
		a.Desired.HasPassword = true
		a.Revision++
		a.DesiredAt = time.Now().UTC().Format(time.RFC3339)
		if err := s.saveLocked(); err != nil {
			*a = before
			return AgentView{}, err
		}
		return safeView(*a), nil
	}
	return AgentView{}, errors.New("agent not found")
}
func (s *Registry) Action(id, action string) (AgentView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.agents {
		a := &s.agents[i]
		if a.ID != id {
			continue
		}
		before := *a
		switch action {
		case "start":
			if a.Desired.Username == "" || a.Desired.Password == "" {
				return AgentView{}, errors.New("请先配置 SOCKS5 用户凭据")
			}
			a.Desired.Enabled = true
		case "stop":
			a.Desired.Enabled = false
		case "restart":
			if !a.Desired.Enabled {
				return AgentView{}, errors.New("代理尚未启动")
			}
		default:
			return AgentView{}, errors.New("unknown action")
		}
		a.Revision++
		a.DesiredAt = time.Now().UTC().Format(time.RFC3339)
		if err := s.saveLocked(); err != nil {
			*a = before
			return AgentView{}, err
		}
		return safeView(*a), nil
	}
	return AgentView{}, errors.New("agent not found")
}
func (s *Registry) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, a := range s.agents {
		if a.ID == id {
			old := s.agents
			s.agents = slices.Delete(s.agents, i, i+1)
			if err := s.saveLocked(); err != nil {
				s.agents = old
				return err
			}
			return nil
		}
	}
	return errors.New("agent not found")
}

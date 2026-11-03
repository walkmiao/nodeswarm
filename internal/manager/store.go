package manager

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"
)

type diskNode struct {
	Node
	Secret string `json:"credential,omitempty"`
}

type diskState struct {
	Nodes  []diskNode `json:"nodes"`
	Events []Event    `json:"events"`
}

type Store struct {
	mu      sync.Mutex
	file    string
	key     []byte
	nodes   []Node
	assigns []Assignment
	events  []Event
}

func OpenStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	keyPath := filepath.Join(dir, "local.key")
	key, err := os.ReadFile(keyPath)
	if errors.Is(err, os.ErrNotExist) {
		key = make([]byte, 32)
		if _, err = rand.Read(key); err != nil {
			return nil, err
		}
		f, e := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if e != nil {
			return nil, e
		}
		if _, e = f.Write(key); e != nil {
			f.Close()
			return nil, e
		}
		if e = f.Close(); e != nil {
			return nil, e
		}
	} else if err != nil {
		return nil, err
	}
	if len(key) != 32 {
		return nil, errors.New("加密密钥长度错误，停止读取节点数据")
	}
	s := &Store{file: filepath.Join(dir, "nodes.json"), key: key, nodes: []Node{}, events: []Event{}, assigns: []Assignment{}}
	raw, err := os.ReadFile(s.file)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	var ds diskState
	if err := json.Unmarshal(raw, &ds); err != nil {
		return nil, fmt.Errorf("读取节点存储失败: %w", err)
	}
	for _, v := range ds.Nodes {
		if v.Secret != "" {
			pass, e := decrypt(key, v.Secret)
			if e != nil {
				return nil, fmt.Errorf("读取节点 %s 的凭据失败: %w", v.ID, e)
			}
			v.Password = pass
		}
		v.HasPassword = v.Password != ""
		s.nodes = append(s.nodes, v.Node)
	}
	s.events = ds.Events
	return s, nil
}

func encrypt(key []byte, pass string) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	g, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, g.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return "", err
	}
	return hex.EncodeToString(g.Seal(nonce, nonce, []byte(pass), nil)), nil
}
func decrypt(key []byte, coded string) (string, error) {
	raw, err := hex.DecodeString(coded)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	g, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < g.NonceSize() {
		return "", errors.New("无效密文")
	}
	plain, err := g.Open(nil, raw[:g.NonceSize()], raw[g.NonceSize():], nil)
	return string(plain), err
}

func (s *Store) saveLocked() error {
	ds := diskState{Events: s.events, Nodes: make([]diskNode, 0, len(s.nodes))}
	for _, node := range s.nodes {
		d := diskNode{Node: node}
		if node.Password != "" {
			secret, err := encrypt(s.key, node.Password)
			if err != nil {
				return err
			}
			d.Secret = secret
		}
		ds.Nodes = append(ds.Nodes, d)
	}
	data, err := json.MarshalIndent(ds, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(s.file), ".nodes-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err := f.Chmod(0600); err != nil {
		f.Close()
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), s.file)
}
func (s *Store) eventLocked(level, msg string) {
	s.events = append(s.events, Event{At: time.Now().Format(time.RFC3339), Level: level, Message: msg})
	if len(s.events) > 200 {
		s.events = s.events[len(s.events)-200:]
	}
}
func randomID() string { b := make([]byte, 8); _, _ = rand.Read(b); return hex.EncodeToString(b) }
func (s *Store) Snapshot() ([]Node, []Assignment, []Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	nodes := slices.Clone(s.nodes)
	for i := range nodes {
		nodes[i].Password = ""
	}
	return nodes, slices.Clone(s.assigns), slices.Clone(s.events)
}
func (s *Store) Create(in NodeInput) (Node, error) {
	if err := ValidateInput(in); err != nil {
		return Node{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	n := Node{ID: randomID(), Name: in.Name, Host: in.Host, Port: in.Port, Username: in.Username, Password: in.Password, HasPassword: in.Password != "", Provider: in.Provider, Region: in.Region, Capacity: in.Capacity, Enabled: enabled, ExpiresAt: in.ExpiresAt, LastStatus: "untested"}
	s.nodes = append(s.nodes, n)
	s.eventLocked("info", "新增代理节点 "+n.Name)
	if err := s.saveLocked(); err != nil {
		s.nodes = s.nodes[:len(s.nodes)-1]
		return Node{}, err
	}
	n.Password = ""
	return n, nil
}
func (s *Store) Update(id string, in NodeInput) (Node, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := -1
	for k, n := range s.nodes {
		if n.ID == id {
			i = k
			break
		}
	}
	if i < 0 {
		return Node{}, errors.New("节点不存在")
	}
	old := s.nodes[i]
	if old.ManagedAgentID != "" {
		return Node{}, errors.New("云端托管节点请在 Agent 配置中修改")
	}
	if in.Password == "" && in.Username == old.Username {
		in.Password = old.Password
	}
	if err := ValidateInput(in); err != nil {
		return Node{}, err
	}
	enabled := old.Enabled
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	s.nodes[i] = Node{ID: id, Name: in.Name, Host: in.Host, Port: in.Port, Username: in.Username, Password: in.Password, HasPassword: in.Password != "", Provider: in.Provider, Region: in.Region, Capacity: in.Capacity, Enabled: enabled, ExpiresAt: in.ExpiresAt, LastStatus: "untested"}
	s.eventLocked("info", "更新代理节点 "+in.Name)
	if err := s.saveLocked(); err != nil {
		s.nodes[i] = old
		return Node{}, err
	}
	n := s.nodes[i]
	n.Password = ""
	return n, nil
}
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := -1
	for k, n := range s.nodes {
		if n.ID == id {
			i = k
			break
		}
	}
	if i < 0 {
		return errors.New("节点不存在")
	}
	if s.nodes[i].ManagedAgentID != "" {
		return errors.New("托管节点必须通过 Agent 管理删除")
	}
	prev := slices.Clone(s.nodes)
	oldAssign := slices.Clone(s.assigns)
	n := s.nodes[i]
	s.nodes = slices.Delete(s.nodes, i, i+1)
	s.assigns = slices.DeleteFunc(s.assigns, func(a Assignment) bool { return a.NodeID == id })
	s.eventLocked("info", "删除节点 "+n.Name+"，同时取消相关规划绑定")
	if err := s.saveLocked(); err != nil {
		s.nodes = prev
		s.assigns = oldAssign
		return err
	}
	return nil
}
func (s *Store) Import(inputs []NodeInput) (int, error) {
	for _, v := range inputs {
		if err := ValidateInput(v); err != nil {
			return 0, err
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	before := slices.Clone(s.nodes)
	count := 0
	for _, v := range inputs {
		duplicate := false
		for _, node := range s.nodes {
			if node.Host == v.Host && node.Port == v.Port && node.Username == v.Username {
				duplicate = true
				break
			}
		}
		if duplicate {
			continue
		}
		n := Node{ID: randomID(), Name: v.Name, Host: v.Host, Port: v.Port, Username: v.Username, Password: v.Password, HasPassword: v.Password != "", Provider: v.Provider, Capacity: v.Capacity, Enabled: true, ExpiresAt: v.ExpiresAt, LastStatus: "untested"}
		s.nodes = append(s.nodes, n)
		count++
	}
	s.eventLocked("info", fmt.Sprintf("批量导入%d个节点，已跳过重复项", count))
	if err := s.saveLocked(); err != nil {
		s.nodes = before
		return 0, err
	}
	return count, nil
}
func (s *Store) NodeByID(id string) (Node, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, n := range s.nodes {
		if n.ID == id {
			return n, true
		}
	}
	return Node{}, false
}
func (s *Store) SaveProbe(id string, p Probe) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.nodes {
		if s.nodes[i].ID == id {
			s.nodes[i].LastStatus = p.Status
			s.nodes[i].LatencyMs = p.LatencyMs
			s.nodes[i].LastCheckedAt = p.CheckedAt
			s.nodes[i].LastError = p.Detail
			if p.OK {
				s.eventLocked("success", "SOCKS5 连接验证通过: "+s.nodes[i].Name)
			} else {
				s.eventLocked("warning", "SOCKS5 检测失败: "+s.nodes[i].Name)
			}
			return s.saveLocked()
		}
	}
	return errors.New("节点不存在")
}
func (s *Store) Assign(p Process, nodeID string, auto bool) (Assignment, error) {
	if p.PID <= 0 || p.StartTime == "" {
		return Assignment{}, errors.New("进程标识不完整，不能绑定")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, a := range s.assigns {
		if a.PID == p.PID && a.StartTime == p.StartTime {
			return Assignment{}, errors.New("进程已存在规划绑定")
		}
	}
	var node Node
	found := false
	if auto {
		node, found = ChooseNode(s.nodes, s.assigns, time.Now())
	} else {
		for _, n := range s.nodes {
			if n.ID == nodeID {
				node = n
				found = true
				break
			}
		}
	}
	if !found {
		return Assignment{}, errors.New("无可用节点，请检查启用状态、容量及有效期")
	}
	used := 0
	for _, a := range s.assigns {
		if a.NodeID == node.ID {
			used++
		}
	}
	if !NodeAvailable(node, used, time.Now()) {
		return Assignment{}, errors.New("节点已禁用、过期或容量不足")
	}
	a := Assignment{PID: p.PID, StartTime: p.StartTime, Name: p.Name, NodeID: node.ID, CreatedAt: time.Now().Format(time.RFC3339), Status: "planned_only"}
	s.assigns = append(s.assigns, a)
	s.eventLocked("info", fmt.Sprintf("计划绑定 PID %d → %s（未激活代理）", p.PID, node.Name))
	_ = s.saveLocked() // assignments intentionally not persisted
	return a, nil
}
func (s *Store) Unassign(pid int, start string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	old := len(s.assigns)
	s.assigns = slices.DeleteFunc(s.assigns, func(a Assignment) bool { return a.PID == pid && a.StartTime == start })
	if len(s.assigns) == old {
		return errors.New("未找到该进程规划绑定")
	}
	s.eventLocked("info", fmt.Sprintf("移除 PID %d 的规划绑定", pid))
	return s.saveLocked()
}
func (s *Store) PruneProcesses(processes []Process) {
	ids := map[string]bool{}
	for _, p := range processes {
		ids[fmt.Sprintf("%d|%s", p.PID, p.StartTime)] = true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	before := len(s.assigns)
	s.assigns = slices.DeleteFunc(s.assigns, func(a Assignment) bool { return !ids[fmt.Sprintf("%d|%s", a.PID, a.StartTime)] })
	if before != len(s.assigns) {
		s.eventLocked("info", fmt.Sprintf("自动清理%d个已退出进程的规划绑定", before-len(s.assigns)))
		_ = s.saveLocked()
	}
}

// UpsertManaged makes a controller-managed cloud SOCKS5 service available in the
// proxy inventory. The node remains distinguishable from imported proxies.
func (s *Store) UpsertManaged(agentID string, in NodeInput) (Node, error) {
	if err := ValidateInput(in); err != nil {
		return Node{}, err
	}
	id := "agent-" + agentID
	s.mu.Lock()
	defer s.mu.Unlock()
	enabled := false
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	n := Node{ID: id, Name: in.Name, Host: in.Host, Port: in.Port, Username: in.Username, Password: in.Password, HasPassword: in.Password != "", Provider: in.Provider, Region: in.Region, Capacity: in.Capacity, Enabled: enabled, LastStatus: "untested", ManagedAgentID: agentID}
	previous := slices.Clone(s.nodes)
	found := false
	for i, v := range s.nodes {
		if v.ID == id {
			n.LastStatus = v.LastStatus
			n.LatencyMs = v.LatencyMs
			n.LastCheckedAt = v.LastCheckedAt
			n.LastError = v.LastError
			if v.Host != n.Host || v.Port != n.Port || v.Username != n.Username {
				n.LastStatus = "untested"
				n.LatencyMs = 0
				n.LastCheckedAt = ""
				n.LastError = ""
			}
			s.nodes[i] = n
			found = true
			break
		}
	}
	if !found {
		s.nodes = append(s.nodes, n)
	}
	if err := s.saveLocked(); err != nil {
		s.nodes = previous
		return Node{}, err
	}
	n.Password = ""
	return n, nil
}
func (s *Store) DeleteManaged(agentID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := "agent-" + agentID
	prev := slices.Clone(s.nodes)
	s.nodes = slices.DeleteFunc(s.nodes, func(n Node) bool { return n.ID == id && n.ManagedAgentID == agentID })
	s.assigns = slices.DeleteFunc(s.assigns, func(a Assignment) bool { return a.NodeID == id })
	if err := s.saveLocked(); err != nil {
		s.nodes = prev
		return err
	}
	return nil
}

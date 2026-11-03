// Package ops implements actionable fleet alerts and operator silences.
package ops

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/walkmiao/nodeswarm/internal/control"
	"github.com/walkmiao/nodeswarm/internal/manager"
)

type Alert struct {
	ID         string `json:"id"`
	Severity   string `json:"severity"`
	Kind       string `json:"kind"`
	Title      string `json:"title"`
	Detail     string `json:"detail"`
	TargetID   string `json:"targetId"`
	TargetName string `json:"targetName"`
	MutedUntil string `json:"mutedUntil,omitempty"`
}

// Evaluate derives live alerts from authoritative Agent and proxy snapshots.
// It never treats desired state as confirmation that the proxy is running.
func Evaluate(agents []control.AgentView, nodes []manager.Node, now time.Time) []Alert {
	out := make([]Alert, 0)
	for _, a := range agents {
		base := "agent:" + a.ID + ":"
		if a.LastSeen == "" {
			if t, err := time.Parse(time.RFC3339, a.RegisteredAt); err == nil && now.Sub(t) > 2*time.Minute {
				out = append(out, Alert{base + "pending", "warning", "agent", "节点尚未完成首次心跳", "检查 Agent 凭据、Controller URL 和服务进程", a.ID, a.Name, ""})
			}
			continue
		}
		seen, _ := time.Parse(time.RFC3339, a.LastSeen)
		if a.Status != "online" {
			sev := "warning"
			if now.Sub(seen) > 3*time.Minute {
				sev = "critical"
			}
			out = append(out, Alert{base + "offline", sev, "agent", "Agent 心跳中断", "最后心跳：" + a.LastSeen + "；检查服务器和出站 HTTPS", a.ID, a.Name, ""})
			continue
		}
		if a.Runtime.Status == "error" {
			out = append(out, Alert{base + "proxy-error", "critical", "agent", "SOCKS5 服务运行异常", a.Runtime.Error, a.ID, a.Name, ""})
		} else if a.Desired.Enabled {
			since, err := time.Parse(time.RFC3339, a.DesiredAt)
			if err == nil && now.Sub(since) > 30*time.Second && (a.Runtime.Status != "running" || a.Runtime.Revision != a.Revision) {
				out = append(out, Alert{base + "out-of-sync", "warning", "agent", "远端配置未确认生效", fmt.Sprintf("期望修订 %d，实际修订 %d，实际状态 %s", a.Revision, a.Runtime.Revision, a.Runtime.Status), a.ID, a.Name, ""})
			}
		}
		if a.Metrics.CPU >= 90 {
			out = append(out, Alert{base + "cpu", "warning", "agent", "CPU 使用率过高", fmt.Sprintf("最近样本 CPU %.1f%%", a.Metrics.CPU), a.ID, a.Name, ""})
		}
		if a.Metrics.MemoryTotal > 0 {
			usage := float64(a.Metrics.MemoryUsed) / float64(a.Metrics.MemoryTotal) * 100
			if usage >= 92 {
				out = append(out, Alert{base + "memory", "warning", "agent", "主机内存使用率过高", fmt.Sprintf("最近样本内存 %.1f%%", usage), a.ID, a.Name, ""})
			}
		}
	}
	for _, n := range nodes {
		base := "node:" + n.ID + ":"
		if !n.Enabled {
			continue
		}
		if n.LastStatus == "failed" {
			out = append(out, Alert{base + "failed", "warning", "node", "代理连接检测失败", n.LastError, n.ID, n.Name, ""})
		}
		if n.ExpiresAt != "" {
			exp, err := time.Parse(time.RFC3339, n.ExpiresAt)
			if err != nil {
				continue
			}
			until := exp.Sub(now)
			if until <= 0 {
				out = append(out, Alert{base + "expired", "critical", "node", "代理节点已过期", "到期时间：" + n.ExpiresAt, n.ID, n.Name, ""})
			} else if until <= 72*time.Hour {
				out = append(out, Alert{base + "expiring", "warning", "node", "代理节点即将到期", "剩余约 " + fmt.Sprintf("%.1f", until.Hours()) + " 小时", n.ID, n.Name, ""})
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		rank := func(v string) int {
			if v == "critical" {
				return 0
			}
			return 1
		}
		if rank(out[i].Severity) != rank(out[j].Severity) {
			return rank(out[i].Severity) < rank(out[j].Severity)
		}
		return out[i].ID < out[j].ID
	})
	return out
}

type Store struct {
	mu    sync.Mutex
	path  string
	muted map[string]time.Time
}

func Open(dir string) (*Store, error) {
	s := &Store{path: filepath.Join(dir, "alert-mutes.json"), muted: map[string]time.Time{}}
	b, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(b, &s.muted); err != nil {
		return nil, err
	}
	return s, nil
}
func (s *Store) List(items []Alert, now time.Time) ([]Alert, int) {
	if s == nil {
		return items, len(items)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	active := 0
	for i := range items {
		until := s.muted[items[i].ID]
		if now.Before(until) {
			items[i].MutedUntil = until.UTC().Format(time.RFC3339)
		} else {
			active++
		}
	}
	return items, active
}
func (s *Store) Mute(id string, hours int) error {
	if s == nil {
		return errors.New("alerts store unavailable")
	}
	if id == "" || len(id) > 180 || hours < 1 || hours > 168 {
		return errors.New("invalid alert or duration")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	old := s.muted[id]
	s.muted[id] = time.Now().Add(time.Duration(hours) * time.Hour).UTC()
	if err := s.saveLocked(); err != nil {
		if old.IsZero() {
			delete(s.muted, id)
		} else {
			s.muted[id] = old
		}
		return err
	}
	return nil
}
func (s *Store) Unmute(id string) error {
	if s == nil {
		return errors.New("alerts store unavailable")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	old, ok := s.muted[id]
	delete(s.muted, id)
	if err := s.saveLocked(); err != nil {
		if ok {
			s.muted[id] = old
		}
		return err
	}
	return nil
}
func (s *Store) saveLocked() error {
	now := time.Now()
	for id, until := range s.muted {
		if now.After(until) {
			delete(s.muted, id)
		}
	}
	b, err := json.MarshalIndent(s.muted, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(s.path), ".alert-*.tmp")
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
	return os.Rename(f.Name(), s.path)
}

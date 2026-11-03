package manager

import (
	"errors"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Node struct {
	ID             string `json:"id"`
	ManagedAgentID string `json:"managedAgentId,omitempty"`
	Name           string `json:"name"`
	Host           string `json:"host"`
	Port           int    `json:"port"`
	Username       string `json:"username"`
	Password       string `json:"-"`
	HasPassword    bool   `json:"hasPassword"`
	Provider       string `json:"provider"`
	Region         string `json:"region"`
	Capacity       int    `json:"capacity"`
	Enabled        bool   `json:"enabled"`
	ExpiresAt      string `json:"expiresAt,omitempty"`
	LastStatus     string `json:"lastStatus"`
	LatencyMs      int64  `json:"latencyMs"`
	LastCheckedAt  string `json:"lastCheckedAt,omitempty"`
	LastError      string `json:"lastError,omitempty"`
}

type NodeInput struct {
	Name      string `json:"name"`
	Host      string `json:"host"`
	Port      int    `json:"port"`
	Username  string `json:"username"`
	Password  string `json:"password"`
	Provider  string `json:"provider"`
	Region    string `json:"region"`
	Capacity  int    `json:"capacity"`
	Enabled   *bool  `json:"enabled"`
	ExpiresAt string `json:"expiresAt"`
}

type Assignment struct {
	PID       int    `json:"pid"`
	StartTime string `json:"startTime"`
	Name      string `json:"name"`
	NodeID    string `json:"nodeId"`
	CreatedAt string `json:"createdAt"`
	Status    string `json:"status"`
}

type Process struct {
	PID       int    `json:"pid"`
	Name      string `json:"name"`
	Path      string `json:"path"`
	StartTime string `json:"startTime"`
}

type Event struct {
	At      string `json:"at"`
	Level   string `json:"level"`
	Message string `json:"message"`
}

type Probe struct {
	OK        bool   `json:"ok"`
	Status    string `json:"status"`
	LatencyMs int64  `json:"latencyMs"`
	Detail    string `json:"detail"`
	CheckedAt string `json:"checkedAt"`
}

func ValidateInput(v NodeInput) error {
	if len(strings.TrimSpace(v.Name)) < 1 || len(v.Name) > 96 {
		return errors.New("节点名称为必填项（最多96字符）")
	}
	if err := ValidateHost(v.Host); err != nil {
		return err
	}
	if v.Port < 1 || v.Port > 65535 {
		return errors.New("端口范围必须是 1-65535")
	}
	if v.Capacity < 1 || v.Capacity > 1000 {
		return errors.New("每节点容量必须是 1-1000")
	}
	if len(v.Username) > 255 || len(v.Password) > 255 {
		return errors.New("SOCKS5 账号或密码不能超过255字节")
	}
	if (v.Username == "") != (v.Password == "") {
		return errors.New("SOCKS5 用户名和密码需同时填写")
	}
	if len(v.Provider) > 96 || len(v.Region) > 96 {
		return errors.New("服务商或地区字段过长")
	}
	if v.ExpiresAt != "" {
		if _, err := time.Parse(time.RFC3339, v.ExpiresAt); err != nil {
			return errors.New("有效期必须为 RFC3339 时间")
		}
	}
	return nil
}

var dnsRegex = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9.-]{0,252}$`)

func ValidateHost(h string) error {
	if net.ParseIP(h) != nil {
		return nil
	}
	if !dnsRegex.MatchString(h) || strings.HasSuffix(h, ".") || strings.Contains(h, "..") {
		return errors.New("请输入合法 IP 或域名，不要包含协议前缀")
	}
	return nil
}

func ParseImport(text string) ([]NodeInput, error) {
	var out []NodeInput
	for i, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) != 5 {
			return nil, fmt.Errorf("第%d行格式错误，需要 IP|端口|账号|密码|到期时间", i+1)
		}
		for k := range parts {
			parts[k] = strings.TrimSpace(parts[k])
		}
		port, err := strconv.Atoi(parts[1])
		if err != nil {
			return nil, fmt.Errorf("第%d行端口无效", i+1)
		}
		in := NodeInput{Name: parts[0] + ":" + parts[1], Host: parts[0], Port: port, Username: parts[2], Password: parts[3], Capacity: 1, Provider: "手工导入"}
		exp := parts[4]
		if exp != "" {
			layouts := []string{"2006-01-02 15:04:05", "2006-01-02", "2006-01-02T15:04:05Z07:00"}
			var parsed time.Time
			var ok bool
			for _, l := range layouts {
				if t, e := time.ParseInLocation(l, exp, time.Local); e == nil {
					parsed = t
					ok = true
					break
				}
			}
			if !ok {
				return nil, fmt.Errorf("第%d行到期时间无效", i+1)
			}
			in.ExpiresAt = parsed.Format(time.RFC3339)
		}
		if err := ValidateInput(in); err != nil {
			return nil, fmt.Errorf("第%d行：%w", i+1, err)
		}
		out = append(out, in)
		if len(out) > 300 {
			return nil, errors.New("一次最多导入300个节点")
		}
	}
	if len(out) == 0 {
		return nil, errors.New("未发现可导入的节点")
	}
	return out, nil
}

func NodeAvailable(n Node, used int, now time.Time) bool {
	if !n.Enabled || n.Capacity <= used {
		return false
	}
	if n.ExpiresAt != "" {
		exp, err := time.Parse(time.RFC3339, n.ExpiresAt)
		if err != nil || !exp.After(now) {
			return false
		}
	}
	return true
}

func ChooseNode(nodes []Node, assignments []Assignment, now time.Time) (Node, bool) {
	used := make(map[string]int)
	for _, a := range assignments {
		used[a.NodeID]++
	}
	var best Node
	found := false
	for _, n := range nodes {
		if !NodeAvailable(n, used[n.ID], now) {
			continue
		}
		if !found || used[n.ID]*best.Capacity < used[best.ID]*n.Capacity ||
			(used[n.ID]*best.Capacity == used[best.ID]*n.Capacity && n.ID < best.ID) {
			best = n
			found = true
		}
	}
	return best, found
}

package control

import (
	"errors"
	"fmt"
	"slices"
	"time"
)

// AutoAssignClient fills currently unused workstation PID quota with eligible
// nodes. It never moves existing bindings: changing a live connection's exit
// requires an explicit operator decision. Capacities are the currently usable
// total slots minus the controller's own local process reservations.
//
// The selection and reservation run under the same registry lock as manual
// binding writes, so concurrent calls cannot overbook the same node.
func (s *Registry) AutoAssignClient(id string, capacities map[string]int, expectedRevision int64) (AgentView, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := -1
	for i := range s.agents {
		if s.agents[i].ID == id && s.agents[i].Kind == "client" {
			idx = i
			break
		}
	}
	if idx < 0 {
		return AgentView{}, 0, errors.New("Windows Client 不存在")
	}
	a := &s.agents[idx]
	if expectedRevision >= 0 && a.Revision != expectedRevision {
		return AgentView{}, 0, errors.New("绑定策略已经变化，请刷新页面重试")
	}
	if a.Client.ScanError != "" || a.Client.ObservedAt == "" {
		return AgentView{}, 0, errors.New("客户端进程清单尚不可用")
	}
	lastSeen, err := time.Parse(time.RFC3339, a.LastSeen)
	if err != nil || time.Since(lastSeen) > time.Minute || lastSeen.After(time.Now().Add(time.Minute)) {
		return AgentView{}, 0, errors.New("Windows Client 已离线，拒绝基于过期进程清单自动分配")
	}
	observedAt, err := time.Parse(time.RFC3339, a.Client.ObservedAt)
	if err != nil || time.Since(observedAt) > 2*time.Minute || observedAt.After(time.Now().Add(time.Minute)) {
		return AgentView{}, 0, errors.New("进程清单已过期，请刷新后重试")
	}
	limit := a.MaxProxyPIDs
	if limit <= 0 || limit > AbsoluteMaxProxyPIDs {
		limit = DefaultMaxProxyPIDs
	}
	// A client may impose a stricter local hard limit. Never plan more than
	// the effective number it reported at the most recent heartbeat.
	if a.Client.MaxProxyPIDs > 0 && a.Client.MaxProxyPIDs < limit {
		limit = a.Client.MaxProxyPIDs
	}
	if len(a.ClientBindings) >= limit {
		return safeView(*a), 0, nil
	}
	used := map[string]int{}
	for _, other := range s.agents {
		if other.Kind != "client" {
			continue
		}
		for _, b := range other.ClientBindings {
			used[b.NodeID]++
		}
	}
	current := make(map[string]bool, len(a.ClientBindings))
	for _, b := range a.ClientBindings {
		current[fmt.Sprintf("%d:%s", b.PID, b.StartTime)] = true
	}
	processes := slices.Clone(a.Client.Processes)
	slices.SortFunc(processes, func(a, b ClientProcess) int {
		if a.PID < b.PID {
			return -1
		}
		if a.PID > b.PID {
			return 1
		}
		return 0
	})
	selected := slices.Clone(a.ClientBindings)
	added := 0
	for _, p := range processes {
		if len(selected) >= limit {
			break
		}
		key := fmt.Sprintf("%d:%s", p.PID, p.StartTime)
		if p.PID <= 0 || p.StartTime == "" || current[key] {
			continue
		}
		// Avoid reassigning another start-time identity of the same PID.
		duplicate := false
		for _, b := range selected {
			if b.PID == p.PID {
				duplicate = true
				break
			}
		}
		if duplicate {
			continue
		}
		best := ""
		for node, capacity := range capacities {
			if capacity <= 0 || used[node] >= capacity {
				continue
			}
			if best == "" || used[node]*capacities[best] < used[best]*capacity ||
				(used[node]*capacities[best] == used[best]*capacity && node < best) {
				best = node
			}
		}
		if best == "" {
			break // no more node capacity; keep already selected bindings
		}
		selected = append(selected, ClientBinding{PID: p.PID, StartTime: p.StartTime, Name: p.Name, NodeID: best})
		current[key] = true
		used[best]++
		added++
	}
	if added == 0 {
		return safeView(*a), 0, nil
	}
	before, beforeRevision, beforeDesiredAt := a.ClientBindings, a.Revision, a.DesiredAt
	a.ClientBindings = selected
	a.Revision++
	a.DesiredAt = time.Now().UTC().Format(time.RFC3339)
	if err := s.saveLocked(); err != nil {
		a.ClientBindings, a.Revision, a.DesiredAt = before, beforeRevision, beforeDesiredAt
		return AgentView{}, 0, err
	}
	return safeView(*a), added, nil
}

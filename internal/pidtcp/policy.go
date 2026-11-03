// Package pidtcp provides the experimental Windows TCP-only, PID-identified
// transparent SOCKS5 relay. It is deliberately not a kill-switch and does not
// claim that UDP, DNS or pre-existing TCP connections are protected.
package pidtcp

import (
	"errors"
	"fmt"
	"net"
	"slices"
	"strings"

	"github.com/walkmiao/nodeswarm/internal/control"
	"github.com/walkmiao/nodeswarm/internal/manager"
)

// Route is tied to one process lifetime, NOT an executable name. StartTime
// must be the exact creation time reported by Windows for this process.
type Route struct {
	PID       int                    `json:"pid"`
	StartTime string                 `json:"startTime"`
	Name      string                 `json:"name"`
	Endpoint  control.ClientEndpoint `json:"endpoint"`
}

func Compile(policy control.ClientPolicy, processes []manager.Process, localLimit int) ([]Route, error) {
	limit := policy.MaxProxyPIDs
	if limit < 1 || limit > control.AbsoluteMaxProxyPIDs {
		limit = control.DefaultMaxProxyPIDs
	}
	if localLimit > 0 && localLimit < limit {
		limit = localLimit
	}
	if localLimit < 0 || localLimit > control.AbsoluteMaxProxyPIDs {
		return nil, errors.New("invalid local PID hard limit")
	}
	if len(policy.Bindings) > limit {
		return nil, fmt.Errorf("PID quota exceeded: %d > %d", len(policy.Bindings), limit)
	}
	lookup := make(map[int]manager.Process, len(processes))
	for _, p := range processes {
		lookup[p.PID] = p
	}
	endpoints := make(map[string]control.ClientEndpoint, len(policy.Endpoints))
	for _, ep := range policy.Endpoints {
		if ep.NodeID == "" || ep.Port < 1 || ep.Port > 65535 || len(ep.Username) > 255 || len(ep.Password) > 255 {
			return nil, errors.New("invalid SOCKS5 endpoint")
		}
		if err := manager.ValidateHost(ep.Host); err != nil {
			return nil, fmt.Errorf("invalid endpoint host: %w", err)
		}
		// The cloud endpoint must not point back to a loopback socket. No
		// implicit direct fallback is permitted when it cannot be reached.
		if ip := net.ParseIP(ep.Host); ip != nil && ip.IsLoopback() {
			return nil, errors.New("cloud endpoint must not be loopback")
		}
		endpoints[ep.NodeID] = ep
	}
	seen := make(map[int]bool, len(policy.Bindings))
	routes := make([]Route, 0, len(policy.Bindings))
	for _, b := range policy.Bindings {
		if b.PID <= 0 || b.StartTime == "" || b.NodeID == "" || seen[b.PID] {
			return nil, errors.New("duplicate or invalid PID binding")
		}
		seen[b.PID] = true
		p, exists := lookup[b.PID]
		if !exists || p.StartTime != b.StartTime || !strings.EqualFold(p.Name, b.Name) || p.Path == "" {
			return nil, fmt.Errorf("stale PID identity: %d", b.PID)
		}
		ep, ok := endpoints[b.NodeID]
		if !ok {
			return nil, fmt.Errorf("unknown proxy node for PID %d", b.PID)
		}
		routes = append(routes, Route{PID: b.PID, StartTime: b.StartTime, Name: p.Name, Endpoint: ep})
	}
	slices.SortFunc(routes, func(a, b Route) int { return a.PID - b.PID })
	return routes, nil
}

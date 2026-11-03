package pidtcp

import (
	"errors"
	"net"
	"net/netip"
	"sync"
	"time"
)

const natRetention = 90 * time.Second

type mapping struct {
	original   flowKey
	reverse    flowKey
	translated uint16
	initialSeq uint32
	route      Route
	accepted   bool
	conn       *net.TCPConn
	expires    time.Time
}
type flowTable struct {
	mu      sync.Mutex
	forward map[flowKey]*mapping
	reverse map[flowKey]*mapping
	ports   map[uint16]bool
	next    uint16
	relay   uint16
	limit   int
	stopped bool
}

func newFlowTable(relay uint16, limit int) *flowTable {
	return &flowTable{forward: map[flowKey]*mapping{}, reverse: map[flowKey]*mapping{}, ports: map[uint16]bool{}, next: 49152, relay: relay, limit: limit}
}
func (t *flowTable) expire(now time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for k, m := range t.forward {
		if !m.expires.IsZero() && !now.Before(m.expires) {
			delete(t.forward, k)
			delete(t.reverse, m.reverse)
			delete(t.ports, m.translated)
		}
	}
}
func (t *flowTable) add(p tcpPacket, route Route) (*mapping, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.stopped {
		return nil, errors.New("redirector stopping")
	}
	if m := t.forward[p.key]; m != nil {
		if m.initialSeq == p.seq {
			return m, nil
		}
		return nil, errors.New("TCP tuple reuse before previous mapping expires")
	}
	if len(t.forward) >= t.limit {
		return nil, errors.New("TCP interception capacity exceeded")
	}
	for k := 0; k < 16384; k++ {
		port := t.next
		t.next++
		if t.next == 0 {
			t.next = 49152
		}
		if t.ports[port] || port == t.relay {
			continue
		}
		rev := flowKey{
			local:  netip.AddrPortFrom(p.key.local.Addr(), t.relay),
			remote: netip.AddrPortFrom(p.key.remote.Addr(), port),
		}
		m := &mapping{original: p.key, reverse: rev, translated: port, initialSeq: p.seq, route: route, expires: time.Now().Add(natRetention)}
		t.forward[p.key] = m
		t.reverse[rev] = m
		t.ports[port] = true
		return m, nil
	}
	return nil, errors.New("TCP NAT translated-port capacity exhausted")
}

// translate keeps route ownership frozen for the lifetime of the TCP mapping.
func (t *flowTable) translate(p tcpPacket) (mapped bool, drop bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if m := t.reverse[p.key]; m != nil {
		p.rewrite(m.original.remote, m.original.local)
		return true, false
	}
	if m := t.forward[p.key]; m != nil {
		if p.syn && !p.ack && p.seq != m.initialSeq {
			return true, true
		}
		p.rewrite(netip.AddrPortFrom(m.original.remote.Addr(), m.translated), netip.AddrPortFrom(m.original.local.Addr(), t.relay))
		return true, false
	}
	return false, false
}
func (t *flowTable) claim(c *net.TCPConn) *mapping {
	local := c.LocalAddr().(*net.TCPAddr).AddrPort()
	remote := c.RemoteAddr().(*net.TCPAddr).AddrPort()
	local = netip.AddrPortFrom(local.Addr().Unmap(), local.Port())
	remote = netip.AddrPortFrom(remote.Addr().Unmap(), remote.Port())
	t.mu.Lock()
	defer t.mu.Unlock()
	m := t.reverse[flowKey{local: local, remote: remote}]
	if t.stopped || m == nil || m.accepted {
		return nil
	}
	m.accepted = true
	m.conn = c
	m.expires = time.Time{}
	return m
}
func (t *flowTable) release(m *mapping) {
	t.mu.Lock()
	defer t.mu.Unlock()
	m.conn = nil
	m.expires = time.Now().Add(natRetention)
}
func (t *flowTable) stop() {
	t.mu.Lock()
	t.stopped = true
	conns := make([]*net.TCPConn, 0)
	for _, m := range t.forward {
		if m.conn != nil {
			conns = append(conns, m.conn)
		}
	}
	t.mu.Unlock()
	for _, c := range conns {
		c.Close()
	}
}

package pidtcp

import (
	"net/netip"
	"sync"
	"time"
)

// PacketIO is the WinDivert send contract; modified packets require a checksum
// recompute and an inbound direction on injection. Pure-Go fake IO makes NAT
// behavior testable without a kernel driver.
type PacketIO interface {
	Send([]byte, packetAddress, bool) error
}
type packetAddress struct{ raw [80]byte }

func (a packetAddress) impostor() bool { return a.flags()&(1<<19) != 0 }
func (a packetAddress) flags() uint32 {
	return uint32(a.raw[8]) | uint32(a.raw[9])<<8 | uint32(a.raw[10])<<16 | uint32(a.raw[11])<<24
}
func (a *packetAddress) clearOutbound() { a.raw[10] &^= 2 }

type ownerFunc func(flowKey) (*Route, error)
type pendingPacket struct {
	p        tcpPacket
	addr     packetAddress
	deadline time.Time
}
type synDecision struct {
	seq     uint32
	expires time.Time
}

// router delays a new SYN for an ownership lookup. Existing NAT flows always
// get precedence, so policy refreshes cannot redirect half a live connection.
type router struct {
	mu          sync.Mutex
	io          PacketIO
	table       *flowTable
	owner       ownerFunc
	pending     map[flowKey]*pendingPacket
	direct      map[flowKey]synDecision
	jobs        chan *pendingPacket
	stopped     bool
	nextSweep   time.Time
	onSendError func(error) // supplied by engine; abort on packet injection failure
}

func newRouter(io PacketIO, table *flowTable, owner ownerFunc) *router {
	return &router{io: io, table: table, owner: owner, pending: map[flowKey]*pendingPacket{}, direct: map[flowKey]synDecision{}, jobs: make(chan *pendingPacket, 256)}
}
func (r *router) send(raw []byte, addr packetAddress, modified bool) {
	if err := r.io.Send(raw, addr, modified); err != nil && r.onSendError != nil {
		r.onSendError(err)
	}
}
func (r *router) route(raw []byte, addr packetAddress) {
	p, err := decodeTCP(raw)
	if err != nil {
		r.send(raw, addr, false)
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if mapped, drop := r.table.translate(p); mapped {
		if !drop {
			r.send(raw, addr, true)
		}
		return
	}
	if r.stopped {
		r.send(raw, addr, false)
		return
	}
	now := time.Now()
	if d, ok := r.direct[p.key]; ok {
		if now.Before(d.expires) && (!p.syn || d.seq == p.seq) {
			r.send(raw, addr, false)
			return
		}
		delete(r.direct, p.key)
	}
	if job, ok := r.pending[p.key]; ok {
		if p.syn && !p.ack && job.p.seq == p.seq {
			return
		}
		// If a non-SYN arrived before the owner decision, release pending
		// SYN first and pin direct. Never splice a TCP handshake midstream.
		delete(r.pending, p.key)
		r.pinDirectLocked(job.p)
		r.send(job.p.data, job.addr, false)
		r.send(raw, addr, false)
		return
	}
	if !p.syn || p.ack || !eligibleDestination(p.key.remote.Addr()) || addr.impostor() {
		r.send(raw, addr, false)
		return
	}
	if len(r.pending) >= 128 || len(r.direct) >= 65536 {
		// Without owner attribution we cannot safely discriminate unrelated
		// apps. Unknown packets pass unchanged. This engine is NOT a kill-switch.
		r.pinDirectLocked(p)
		r.send(raw, addr, false)
		return
	}
	buf := append([]byte(nil), raw...)
	cp, _ := decodeTCP(buf)
	job := &pendingPacket{p: cp, addr: addr, deadline: now.Add(80 * time.Millisecond)}
	r.pending[p.key] = job
	select {
	case r.jobs <- job:
	default:
		delete(r.pending, p.key)
		r.pinDirectLocked(cp)
		r.send(buf, addr, false)
	}
}
func eligibleDestination(ip netip.Addr) bool {
	return ip.IsValid() && ip.IsGlobalUnicast() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast()
}
func (r *router) pinDirectLocked(p tcpPacket) {
	r.direct[p.key] = synDecision{seq: p.seq, expires: time.Now().Add(5 * time.Minute)}
}
func (r *router) resolve(job *pendingPacket) {
	r.mu.Lock()
	active := r.pending[job.p.key] == job
	r.mu.Unlock()
	if !active {
		return
	}
	route, err := r.owner(job.p.key)
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.pending[job.p.key] != job {
		return
	}
	delete(r.pending, job.p.key)
	// Never redirect if no owner or a lookup exceeded its bounded deadline.
	// The bound PID may have escaped in this case; report capability as
	// best-effort, not as strict isolation.
	if err != nil || route == nil || time.Now().After(job.deadline) {
		r.pinDirectLocked(job.p)
		r.send(job.p.data, job.addr, false)
		return
	}
	if _, err = r.table.add(job.p, *route); err != nil {
		// A positive match must not fall back direct when the relay is full.
		return
	}
	if mapped, drop := r.table.translate(job.p); mapped && !drop {
		r.send(job.p.data, job.addr, true)
	}
}
func (r *router) expire() {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	for k, job := range r.pending {
		if !now.Before(job.deadline) {
			delete(r.pending, k)
			r.pinDirectLocked(job.p)
			r.send(job.p.data, job.addr, false)
		}
	}
	// The receive loop ticks every 10 ms to expire pending SYNs. Walking
	// 65K pinned-direct flows 100 times/s would consume unnecessary CPU.
	if !now.Before(r.nextSweep) {
		for k, d := range r.direct {
			if !now.Before(d.expires) {
				delete(r.direct, k)
			}
		}
		r.nextSweep = now.Add(time.Second)
	}
}
func (r *router) stop() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.stopped {
		return
	}
	r.stopped = true
	for k, job := range r.pending {
		delete(r.pending, k)
		r.send(job.p.data, job.addr, false)
	}
	close(r.jobs)
}

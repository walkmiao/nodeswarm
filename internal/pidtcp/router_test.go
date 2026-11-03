package pidtcp

import (
	"net/netip"
	"sync"
	"testing"
	"time"
)

type outputPacket struct {
	p        []byte
	modified bool
}
type testIO struct {
	mu      sync.Mutex
	outputs []outputPacket
}

func (o *testIO) Send(data []byte, _ packetAddress, modified bool) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.outputs = append(o.outputs, outputPacket{append([]byte(nil), data...), modified})
	return nil
}
func (o *testIO) last() (outputPacket, bool) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if len(o.outputs) == 0 {
		return outputPacket{}, false
	}
	return o.outputs[len(o.outputs)-1], true
}
func TestRouterSeparatesMappedPIDAndUnknown(t *testing.T) {
	io := &testIO{}
	table := newFlowTable(34010, 16)
	route := Route{PID: 101, StartTime: "t1"}
	calls := 0
	r := newRouter(io, table, func(k flowKey) (*Route, error) {
		calls++
		if k.local.Port() == 54321 {
			return &route, nil
		}
		return nil, nil
	})
	b := makePacket(false, 2)
	r.route(b, packetAddress{})
	select {
	case j := <-r.jobs:
		r.resolve(j)
	case <-time.After(time.Second):
		t.Fatal("no owner task")
	}
	if calls != 1 {
		t.Fatal("owner not resolved")
	}
	out, ok := io.last()
	if !ok || !out.modified {
		t.Fatal("captured PID was not redirected")
	}
	p, _ := decodeTCP(out.p)
	if p.key.remote.Port() != 34010 {
		t.Fatal("TCP NAT did not target relay")
	}
	// Unknown PID receives direct original connection unchanged.
	x := makePacket(false, 3)
	binaryPortAt(x, 54322)
	r.route(x, packetAddress{})
	select {
	case j := <-r.jobs:
		r.resolve(j)
	case <-time.After(time.Second):
		t.Fatal("no other owner task")
	}
	out, _ = io.last()
	if out.modified {
		t.Fatal("untracked process redirected")
	}
	q, _ := decodeTCP(out.p)
	if q.key.remote != netip.MustParseAddrPort("203.0.113.2:443") {
		t.Fatal("untracked target modified")
	}
	r.stop()
}
func binaryPortAt(b []byte, n uint16) { b[20] = byte(n >> 8); b[21] = byte(n) }
func TestRouterRejectsConflictingSourceReuse(t *testing.T) {
	io := &testIO{}
	table := newFlowTable(34010, 1)
	r := newRouter(io, table, func(key flowKey) (*Route, error) { return &Route{PID: 101}, nil })
	r.route(makePacket(false, 5), packetAddress{})
	r.resolve(<-r.jobs)
	reuse := makePacket(false, 6)
	r.route(reuse, packetAddress{})
	if out, ok := io.last(); !ok || !out.modified {
		t.Fatal("expected prior NAT first write")
	}
	// conflicting SYN is suppressed instead of joining an existing flow.
	io.mu.Lock()
	count := len(io.outputs)
	io.mu.Unlock()
	if count != 1 {
		t.Fatalf("conflicting SYN injected: %d", count)
	}
	r.stop()
}

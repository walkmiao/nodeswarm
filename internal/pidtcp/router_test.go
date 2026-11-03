package pidtcp

import (
	"fmt"
	"net/netip"
	"strings"
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

type brokenPacketIO struct{}

func (brokenPacketIO) Send(_ []byte, _ packetAddress, _ bool) error {
	return fmt.Errorf("injection denied")
}

func TestPacketInjectionFailureIsReported(t *testing.T) {
	table := newFlowTable(34010, 10)
	r := newRouter(brokenPacketIO{}, table, func(flowKey) (*Route, error) { return nil, nil })
	var errorsSeen int
	r.onFailure = func(err error) {
		if strings.Contains(err.Error(), "injection denied") {
			errorsSeen++
		}
	}
	// ACK cannot open a new mapping; it must be retransmitted unchanged.
	b := makePacket(false, 999)
	b[33] = 0x10
	r.route(b, packetAddress{})
	if errorsSeen != 1 {
		t.Fatalf("driver injection failure was not surfaced: %d", errorsSeen)
	}
	r.stop()
}
func TestPinnedDirectDecisionsSweepOnInterval(t *testing.T) {
	r := newRouter(&testIO{}, newFlowTable(34010, 4), func(flowKey) (*Route, error) { return nil, nil })
	key := flowKey{local: netip.MustParseAddrPort("10.0.0.1:1001"), remote: netip.MustParseAddrPort("203.0.113.2:443")}
	r.direct[key] = synDecision{seq: 1, expires: time.Now().Add(-time.Second)}
	r.nextSweep = time.Now().Add(time.Minute)
	r.expire()
	if len(r.direct) != 1 {
		t.Fatal("full direct cache scanned before scheduled sweep")
	}
	r.nextSweep = time.Now().Add(-time.Second)
	r.expire()
	if len(r.direct) != 0 {
		t.Fatal("direct cache did not expire on scheduled sweep")
	}
	r.stop()
}

func TestRouterDecisionCacheOverloadStopsNewRedirects(t *testing.T) {
	io := &testIO{}
	r := newRouter(io, newFlowTable(34010, 4), func(flowKey) (*Route, error) { return &Route{PID: 9}, nil })
	r.decisionCap = 1
	var errorText string
	r.onFailure = func(err error) { errorText = err.Error() }
	first, _ := decodeTCP(makePacket(false, 1))
	r.pinDirectLocked(first)
	secondBytes := makePacket(false, 2)
	binaryPortAt(secondBytes, 55555)
	second, _ := decodeTCP(secondBytes)
	r.pinDirectLocked(second)
	if !r.bypass || len(r.direct) != 1 || !strings.Contains(errorText, "cache exhausted") {
		t.Fatalf("cache cap must fail engine without unbounded growth: bypass=%v size=%d error=%q", r.bypass, len(r.direct), errorText)
	}
	// Existing NAT takes precedence, but unowned new SYNs cannot be promoted to proxy.
	r.route(secondBytes, packetAddress{})
	if _, ok := io.last(); !ok {
		t.Fatal("bypass should forward unchanged")
	}
	r.stop()
}

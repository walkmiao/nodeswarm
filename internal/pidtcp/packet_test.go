package pidtcp

import (
	"encoding/binary"
	"net/netip"
	"testing"
	"time"
)

func makePacket(v6 bool, seq uint32) []byte {
	off, n := 20, 40
	if v6 {
		off = 40
		n = 60
	}
	b := make([]byte, n)
	if !v6 {
		b[0] = 0x45
		binary.BigEndian.PutUint16(b[2:], uint16(n))
		b[9] = 6
		copy(b[12:16], []byte{10, 0, 0, 2})
		copy(b[16:20], []byte{203, 0, 113, 2})
	} else {
		b[0] = 0x60
		binary.BigEndian.PutUint16(b[4:], 20)
		b[6] = 6
		copy(b[8:24], netip.MustParseAddr("2001:db8::1").AsSlice())
		copy(b[24:40], netip.MustParseAddr("2001:db8::2").AsSlice())
	}
	binary.BigEndian.PutUint16(b[off:], 54321)
	binary.BigEndian.PutUint16(b[off+2:], 443)
	binary.BigEndian.PutUint32(b[off+4:], seq)
	b[off+12] = 0x50
	b[off+13] = 0x02
	return b
}
func TestPacketDecodeRewriteV4V6(t *testing.T) {
	for _, v6 := range []bool{false, true} {
		b := makePacket(v6, 88)
		p, err := decodeTCP(b)
		if err != nil {
			t.Fatal(err)
		}
		if !p.syn || p.ack || p.seq != 88 || p.key.local.Port() != 54321 || p.key.remote.Port() != 443 {
			t.Fatalf("bad packet: %+v", p)
		}
		a := netip.MustParseAddrPort("10.0.0.2:2345")
		z := netip.MustParseAddrPort("203.0.113.2:8888")
		if v6 {
			a = netip.MustParseAddrPort("[2001:db8::1]:2345")
			z = netip.MustParseAddrPort("[2001:db8::2]:8888")
		}
		p.rewrite(a, z)
		q, err := decodeTCP(b)
		if err != nil {
			t.Fatal(err)
		}
		if q.key.local != a || q.key.remote != z {
			t.Fatalf("rewrite incorrect: %+v", q.key)
		}
	}
}
func TestRejectBrokenPackets(t *testing.T) {
	b := makePacket(false, 1)
	if _, err := decodeTCP(b[:18]); err == nil {
		t.Fatal("short accepted")
	}
	b[6] = 0x20 // nonzero fragment offset
	if _, err := decodeTCP(b); err == nil {
		t.Fatal("fragment accepted")
	}
	b = makePacket(true, 1)
	b[6] = 44
	if _, err := decodeTCP(b); err == nil {
		t.Fatal("IPv6 fragmentation accepted")
	}
}
func TestNATRewritesBothDirections(t *testing.T) {
	table := newFlowTable(34012, 4)
	original := makePacket(false, 1234)
	p, _ := decodeTCP(original)
	route := Route{PID: 42}
	m, err := table.add(p, route)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := table.add(p, route); err != nil {
		t.Fatalf("duplicate retransmit rejected: %v", err)
	}
	fwd := makePacket(false, 1234)
	f, _ := decodeTCP(fwd)
	if mapped, drop := table.translate(f); !mapped || drop {
		t.Fatal("forward route not established")
	}
	q, _ := decodeTCP(fwd)
	if q.key.remote.Port() != 34012 || q.key.local.Port() != m.translated || q.key.local.Addr() != p.key.remote.Addr() || q.key.remote.Addr() != p.key.local.Addr() {
		t.Fatalf("forward NAT invalid %+v", q.key)
	}
	// reverse: TCP server is local relay with source L:34012 replying to R:translated
	response := makePacket(false, 987)
	resp, _ := decodeTCP(response)
	resp.rewrite(m.reverse.local, m.reverse.remote)
	resp, _ = decodeTCP(response)
	if mapped, drop := table.translate(resp); !mapped || drop {
		t.Fatal("reverse route missing")
	}
	round, _ := decodeTCP(response)
	if round.key.local != p.key.remote || round.key.remote != p.key.local {
		t.Fatalf("reverse NAT invalid: %+v", round.key)
	}
	reuse := makePacket(false, 500)
	rp, _ := decodeTCP(reuse)
	if mapped, drop := table.translate(rp); !mapped || !drop {
		t.Fatal("PID/port tuple reuse must not splice")
	}
	table.mu.Lock()
	m.expires = time.Now().Add(-time.Second)
	table.mu.Unlock()
	table.expire(time.Now())
	table.mu.Lock()
	count := len(table.forward)
	table.mu.Unlock()
	if count != 0 {
		t.Fatal("expired NAT entry retained")
	}
}

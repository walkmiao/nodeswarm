package proxy

import (
	"bytes"
	"encoding/binary"
	"github.com/walkmiao/nodeswarm/internal/control"
	"io"
	"net"
	"testing"
	"time"
)

func TestPublicIP(t *testing.T) {
	for _, s := range []string{"127.0.0.1", "10.0.1.4", "169.254.169.254", "100.65.1.1", "192.0.2.5", "::1"} {
		if PublicTarget(s) {
			t.Fatalf("accepted reserved %s", s)
		}
	}
	if !PublicTarget("1.1.1.1") {
		t.Fatal("rejected public")
	}
}
func TestSOCKS5Auth(t *testing.T) {
	p, e := Start(control.ProxyConfig{Enabled: true, Listen: "127.0.0.1", Port: 0, Username: "u", Password: "p", MaxConnections: 2, Capacity: 1})
	if e == nil {
		p.Stop()
		t.Fatal("port zero should fail")
	} // use allocated ephemeral port
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	p, e = Start(control.ProxyConfig{Enabled: true, Listen: "127.0.0.1", Port: port, Username: "u", Password: "p", MaxConnections: 2, Capacity: 1})
	if e != nil {
		t.Fatal(e)
	}
	defer p.Stop()
	c, e := net.DialTimeout("tcp", p.ln.Addr().String(), time.Second)
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(3 * time.Second))
	_, _ = c.Write([]byte{5, 1, 2})
	b := make([]byte, 2)
	if _, e = io.ReadFull(c, b); e != nil || b[1] != 2 {
		t.Fatalf("method %v %v", b, e)
	}
	_, _ = c.Write([]byte{1, 1, 'u', 1, 'p'})
	if _, e = io.ReadFull(c, b); e != nil || b[1] != 0 {
		t.Fatalf("auth %v %v", b, e)
	}
	_, _ = c.Write([]byte{5, 1, 0, 1, 127, 0, 0, 1, 0, 80})
	reply := make([]byte, 10)
	_, e = io.ReadFull(c, reply)
	if e != nil || reply[1] != 2 {
		t.Fatalf("block private IP got %v %v", reply, e)
	}
	if binary.BigEndian.Uint16(reply[8:10]) != 0 {
		t.Fatal("unexpected reply port")
	}
}

func TestUDPBoundedPortPoolAndReuse(t *testing.T) {
	probe, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1")})
	if err != nil {
		t.Fatal(err)
	}
	udpPort := probe.LocalAddr().(*net.UDPAddr).Port
	_ = probe.Close()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()
	c := control.ProxyConfig{Enabled: true, Listen: "127.0.0.1", Port: port, Username: "u", Password: "p", MaxConnections: 4, Capacity: 2, UDP: true, UDPPortStart: udpPort, UDPPortEnd: udpPort}
	p, err := Start(c)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Stop()
	first, release, err := p.openUDPRelay()
	if err != nil {
		t.Fatal(err)
	}
	if got := first.LocalAddr().(*net.UDPAddr).Port; got != udpPort {
		t.Fatalf("want UDP %d got %d", udpPort, got)
	}
	if other, cleanup, err := p.openUDPRelay(); err == nil {
		other.Close()
		cleanup()
		t.Fatal("exhausted port allocated twice")
	}
	_ = first.Close()
	release()
	again, cleanup, err := p.openUDPRelay()
	if err != nil {
		t.Fatal("released port not reusable:", err)
	}
	_ = again.Close()
	cleanup()
}

func TestAdvertisePublicUDPAddress(t *testing.T) {
	for _, tc := range []struct {
		host   string
		prefix []byte
	}{
		{"203.0.113.2", []byte{5, 0, 0, 1, 203, 0, 113, 2}},
		{"proxy.example.net", append([]byte{5, 0, 0, 3, 17}, []byte("proxy.example.net")...)},
	} {
		left, right := net.Pipe()
		result := make(chan error, 1)
		go func() { result <- writeUDPAssociateReply(left, tc.host, 11500) }()
		expectedLen := len(tc.prefix) + 2
		got := make([]byte, expectedLen)
		right.SetReadDeadline(time.Now().Add(time.Second))
		if _, err := io.ReadFull(right, got); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got[:len(tc.prefix)], tc.prefix) {
			t.Fatalf("BND.ADDR %q: %v", tc.host, got)
		}
		if got[expectedLen-2] != byte(11500>>8) || got[expectedLen-1] != byte(11500&255) {
			t.Fatalf("wrong UDP port: %v", got)
		}
		if err := <-result; err != nil {
			t.Fatal(err)
		}
		left.Close()
		right.Close()
	}
}

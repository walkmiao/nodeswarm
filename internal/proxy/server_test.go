package proxy

import (
	"encoding/binary"
	"nodeconsole.local/node-console/internal/control"
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

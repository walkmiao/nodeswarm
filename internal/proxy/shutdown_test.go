package proxy

import (
	"io"
	"net"
	"testing"
	"time"

	"github.com/walkmiao/nodeswarm/internal/control"
)

// Regression: an open UDP ASSOCIATE must not block remote stop indefinitely.
func TestStopWithOpenUDPAssociation(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()
	p, err := Start(control.ProxyConfig{Enabled: true, Listen: "127.0.0.1", Port: port, Username: "u", Password: "p", UDP: true, MaxConnections: 8, Capacity: 2})
	if err != nil {
		t.Fatal(err)
	}
	conn, err := net.Dial("tcp", p.ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(3 * time.Second))
	conn.Write([]byte{5, 1, 2})
	buf := make([]byte, 2)
	if _, err = io.ReadFull(conn, buf); err != nil {
		t.Fatal(err)
	}
	conn.Write([]byte{1, 1, 'u', 1, 'p'})
	if _, err = io.ReadFull(conn, buf); err != nil {
		t.Fatal(err)
	}
	conn.Write([]byte{5, 3, 0, 1, 0, 0, 0, 0, 0, 0})
	response := make([]byte, 10)
	if _, err = io.ReadFull(conn, response); err != nil {
		t.Fatal(err)
	}
	if response[1] != 0 {
		t.Fatalf("UDP response %v", response)
	}
	done := make(chan struct{})
	go func() { p.Stop(); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("proxy shutdown hung with an open UDP ASSOCIATE")
	}
}

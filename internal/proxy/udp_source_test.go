package proxy

import (
	"io"
	"net"
	"testing"
	"time"

	"github.com/walkmiao/nodeswarm/internal/control"
)

func TestUDPAssociateSourceAddressValidation(t *testing.T) {
	peer := net.ParseIP("198.51.100.9")
	for _, tc := range []struct {
		host string
		port int
		ok   bool
	}{
		{"0.0.0.0", 0, true}, {"0.0.0.0", 55001, true},
		{"198.51.100.9", 55001, true}, {"198.51.100.10", 55001, false},
		{"127.0.0.1", 0, false}, {"proxy.example.org", 0, false},
		{"0.0.0.0", 65536, false},
	} {
		err := validateUDPAssociateSource(peer, tc.host, tc.port)
		if (err == nil) != tc.ok {
			t.Fatalf("host=%q port=%d: %v", tc.host, tc.port, err)
		}
	}
	if err := validateUDPAssociateSource(net.ParseIP("2001:db8::1"), "::", 0); err != nil {
		t.Fatal(err)
	}
}

func TestUDPAssociatePinsRequestedPortAndFirstSender(t *testing.T) {
	peer := net.ParseIP("198.51.100.9")
	good := &net.UDPAddr{IP: peer, Port: 55321}
	differentPort := &net.UDPAddr{IP: peer, Port: 55322}
	differentIP := &net.UDPAddr{IP: net.ParseIP("198.51.100.10"), Port: 55321}
	if !udpSourceMatches(good, peer, 55321, nil) || !udpSourceMatches(good, peer, 55321, good) {
		t.Fatal("authorized source rejected")
	}
	for _, addr := range []*net.UDPAddr{differentPort, differentIP} {
		if udpSourceMatches(addr, peer, 55321, nil) {
			t.Fatalf("untrusted source accepted: %v", addr)
		}
	}
	if udpSourceMatches(differentPort, peer, 0, good) {
		t.Fatal("first-source pin ignored for unspecified port")
	}
	if !udpSourceMatches(differentPort, peer, 0, nil) {
		t.Fatal("zero-port clients must remain supported")
	}
}

func TestUDPAssociateRejectsWrongClaimWithoutOpeningRelay(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	srv, err := Start(control.ProxyConfig{Enabled: true, Listen: "127.0.0.1", Port: port, Username: "u", Password: "p", MaxConnections: 4, Capacity: 2, UDP: true})
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Stop()
	conn, err := net.DialTimeout("tcp", srv.ln.Addr().String(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err = conn.Write([]byte{5, 1, 2}); err != nil {
		t.Fatal(err)
	}
	reply := make([]byte, 2)
	if _, err = io.ReadFull(conn, reply); err != nil || reply[1] != 2 {
		t.Fatalf("method: %v %v", reply, err)
	}
	if _, err = conn.Write([]byte{1, 1, 'u', 1, 'p'}); err != nil {
		t.Fatal(err)
	}
	if _, err = io.ReadFull(conn, reply); err != nil || reply[1] != 0 {
		t.Fatalf("auth: %v %v", reply, err)
	}
	// UDP ASSOCIATE claims 127.0.0.2 even though TCP peer is 127.0.0.1.
	if _, err = conn.Write([]byte{5, 3, 0, 1, 127, 0, 0, 2, 0, 0}); err != nil {
		t.Fatal(err)
	}
	head := make([]byte, 4)
	if _, err = io.ReadFull(conn, head); err != nil {
		t.Fatal(err)
	}
	if head[0] != 5 || head[1] != 2 {
		t.Fatalf("unexpected UDP source policy response: %v", head)
	}
	srv.udpMu.Lock()
	count := len(srv.udpUsed)
	srv.udpMu.Unlock()
	if count != 0 {
		t.Fatalf("untrusted association allocated UDP relay: %d", count)
	}
}

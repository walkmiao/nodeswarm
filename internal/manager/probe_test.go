package manager

import (
	"io"
	"net"
	"strconv"
	"testing"
	"time"
)

func startFakeSOCKS(t *testing.T, auth bool, allow bool) (string, int) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		c, e := ln.Accept()
		if e != nil {
			return
		}
		defer c.Close()
		b := make([]byte, 3)
		if _, e = io.ReadFull(c, b); e != nil {
			return
		}
		if b[0] != 5 || b[1] != 1 {
			return
		}
		method := byte(0)
		if auth {
			method = 2
		}
		_, _ = c.Write([]byte{5, method})
		if auth {
			v := make([]byte, 2)
			if _, e = io.ReadFull(c, v); e != nil {
				return
			}
			u := make([]byte, int(v[1])+1)
			if _, e = io.ReadFull(c, u); e != nil {
				return
			}
			p := make([]byte, int(u[len(u)-1]))
			if _, e = io.ReadFull(c, p); e != nil {
				return
			}
			_, _ = c.Write([]byte{1, 0})
		}
		req := make([]byte, 10)
		if _, e = io.ReadFull(c, req); e != nil {
			return
		}
		code := byte(0)
		if !allow {
			code = 5
		}
		_, _ = c.Write([]byte{5, code, 0, 1, 127, 0, 0, 1, 0, 0})
	}()
	host, portText, _ := net.SplitHostPort(ln.Addr().String())
	port, _ := strconv.Atoi(portText)
	return host, port
}
func TestProbeSOCKSNoAuth(t *testing.T) {
	host, port := startFakeSOCKS(t, false, true)
	p := ProbeSOCKS5(Node{Host: host, Port: port}, 2*time.Second)
	if !p.OK || p.Status != "tcp_ok" {
		t.Fatalf("expected handshake success: %+v", p)
	}
}
func TestProbeSOCKSAuth(t *testing.T) {
	host, port := startFakeSOCKS(t, true, true)
	p := ProbeSOCKS5(Node{Host: host, Port: port, Username: "bob", Password: "pass"}, 2*time.Second)
	if !p.OK {
		t.Fatalf("expected authenticated handshake success: %+v", p)
	}
}
func TestProbeSOCKSRejected(t *testing.T) {
	host, port := startFakeSOCKS(t, false, false)
	p := ProbeSOCKS5(Node{Host: host, Port: port}, 2*time.Second)
	if p.OK || p.Status != "failed" {
		t.Fatalf("expected CONNECT rejection: %+v", p)
	}
}

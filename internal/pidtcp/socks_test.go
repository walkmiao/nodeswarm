package pidtcp

import (
	"context"
	"encoding/binary"
	"github.com/walkmiao/nodeswarm/internal/control"
	"io"
	"net"
	"net/netip"
	"strconv"
	"testing"
	"time"
)

func TestSOCKS5NoAuthAndAuthenticated(t *testing.T) {
	for _, withPassword := range []bool{false, true} {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		defer ln.Close()
		done := make(chan error, 1)
		go func() {
			c, e := ln.Accept()
			if e != nil {
				done <- e
				return
			}
			defer c.Close()
			c.SetDeadline(time.Now().Add(time.Second * 4))
			greeting := make([]byte, 3)
			if _, e = io.ReadFull(c, greeting); e != nil {
				done <- e
				return
			}
			method := byte(0)
			if withPassword {
				method = 2
			}
			_, e = c.Write([]byte{5, method})
			if e != nil {
				done <- e
				return
			}
			if withPassword {
				head := make([]byte, 2)
				if _, e = io.ReadFull(c, head); e != nil {
					done <- e
					return
				}
				u := make([]byte, int(head[1])+1)
				if _, e = io.ReadFull(c, u); e != nil {
					done <- e
					return
				}
				pass := make([]byte, int(u[len(u)-1]))
				if _, e = io.ReadFull(c, pass); e != nil {
					done <- e
					return
				}
				if string(pass) != "pw" {
					done <- io.ErrUnexpectedEOF
					return
				}
				_, e = c.Write([]byte{1, 0})
				if e != nil {
					done <- e
					return
				}
			}
			req := make([]byte, 4)
			if _, e = io.ReadFull(c, req); e != nil {
				done <- e
				return
			}
			if req[0] != 5 || req[1] != 1 || req[3] != 1 {
				done <- io.ErrUnexpectedEOF
				return
			}
			dst := make([]byte, 6)
			if _, e = io.ReadFull(c, dst); e != nil {
				done <- e
				return
			}
			if binary.BigEndian.Uint16(dst[4:]) != 443 {
				done <- io.ErrUnexpectedEOF
				return
			}
			_, e = c.Write([]byte{5, 0, 0, 1, 127, 0, 0, 1, 0, 0})
			done <- e
		}()
		ep := control.ClientEndpoint{Host: "127.0.0.1", Port: ln.Addr().(*net.TCPAddr).Port}
		if withPassword {
			ep.Username = "u"
			ep.Password = "pw"
		}
		c, err := dialSOCKS5(context.Background(), ep, netip.MustParseAddrPort("198.51.100.2:443"))
		if err != nil {
			t.Fatalf("auth=%v: %v", withPassword, err)
		}
		c.Close()
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
}
func TestSOCKS5CannotFallBackToDirect(t *testing.T) {
	ep := control.ClientEndpoint{Host: "127.0.0.1", Port: 1}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	c, e := dialSOCKS5(ctx, ep, netip.MustParseAddrPort("1.1.1.1:443"))
	if e == nil {
		c.Close()
		t.Fatal("unexpected direct connection")
	}
	if e.Error() == strconv.Itoa(ep.Port) {
		t.Fatal("missing proxy error context")
	}
}

package pidtcp

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"strconv"
	"sync"
	"time"

	"github.com/walkmiao/nodeswarm/internal/control"
)

// dialSOCKS5 only contacts the configured upstream. It never retries a failed
// proxy connection directly against destination. The destination remains a
// numeric IP so local DNS resolution is unnecessary in this TCP relay.
func dialSOCKS5(ctx context.Context, ep control.ClientEndpoint, dst netip.AddrPort) (net.Conn, error) {
	dialCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	dialer := net.Dialer{Timeout: 8 * time.Second}
	addr := net.JoinHostPort(ep.Host, strconv.Itoa(ep.Port))
	c, err := dialer.DialContext(dialCtx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("connect SOCKS5: %w", err)
	}
	success := false
	defer func() {
		if !success {
			c.Close()
		}
	}()
	deadline, ok := dialCtx.Deadline()
	if ok {
		_ = c.SetDeadline(deadline)
	}
	method := byte(0)
	if ep.Username != "" || ep.Password != "" {
		method = 2
	}
	if _, err = c.Write([]byte{5, 1, method}); err != nil {
		return nil, err
	}
	reply := make([]byte, 2)
	if _, err = io.ReadFull(c, reply); err != nil {
		return nil, err
	}
	if reply[0] != 5 || reply[1] != method {
		return nil, errors.New("SOCKS5 rejected requested authentication method")
	}
	if method == 2 {
		if len(ep.Username) > 255 || len(ep.Password) > 255 {
			return nil, errors.New("SOCKS5 credentials exceed 255 bytes")
		}
		auth := make([]byte, 0, 3+len(ep.Username)+len(ep.Password))
		auth = append(auth, 1, byte(len(ep.Username)))
		auth = append(auth, ep.Username...)
		auth = append(auth, byte(len(ep.Password)))
		auth = append(auth, ep.Password...)
		if _, err = c.Write(auth); err != nil {
			return nil, err
		}
		if _, err = io.ReadFull(c, reply); err != nil {
			return nil, err
		}
		if reply[0] != 1 || reply[1] != 0 {
			return nil, errors.New("SOCKS5 credentials rejected")
		}
	}
	if !dst.IsValid() || dst.Port() == 0 {
		return nil, errors.New("invalid SOCKS5 destination")
	}
	target := dst.Addr().Unmap()
	req := []byte{5, 1, 0}
	if target.Is4() {
		a := target.As4()
		req = append(req, 1)
		req = append(req, a[:]...)
	} else if target.Is6() {
		a := target.As16()
		req = append(req, 4)
		req = append(req, a[:]...)
	} else {
		return nil, errors.New("unsupported destination")
	}
	req = binary.BigEndian.AppendUint16(req, dst.Port())
	if _, err = c.Write(req); err != nil {
		return nil, err
	}
	header := make([]byte, 4)
	if _, err = io.ReadFull(c, header); err != nil {
		return nil, err
	}
	if header[0] != 5 || header[1] != 0 || header[2] != 0 {
		return nil, fmt.Errorf("SOCKS5 CONNECT rejected: %d", header[1])
	}
	n := 0
	switch header[3] {
	case 1:
		n = 4
	case 4:
		n = 16
	case 3:
		var l [1]byte
		if _, err = io.ReadFull(c, l[:]); err != nil {
			return nil, err
		}
		n = int(l[0])
	default:
		return nil, errors.New("invalid SOCKS5 address type")
	}
	if _, err = io.CopyN(io.Discard, c, int64(n+2)); err != nil {
		return nil, err
	}
	_ = c.SetDeadline(time.Time{})
	success = true
	return c, nil
}

// relayTCP preserves half-close; an upstream SOCKS failure is an error, not a
// reason to fall back to a direct game-to-server socket.
func relayTCP(ctx context.Context, local *net.TCPConn, ep control.ClientEndpoint, dst netip.AddrPort) error {
	upstream, err := dialSOCKS5(ctx, ep, dst)
	if err != nil {
		return err
	}
	defer upstream.Close()
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	transfer := func(dst net.Conn, src net.Conn) {
		defer wg.Done()
		_, e := io.Copy(dst, src)
		if c, ok := dst.(interface{ CloseWrite() error }); ok {
			_ = c.CloseWrite()
		}
		if e != nil {
			errs <- e
		} else {
			errs <- nil
		}
	}
	wg.Add(2)
	go transfer(upstream, local)
	go transfer(local, upstream)
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-ctx.Done():
		local.Close()
		upstream.Close()
		<-done
		return ctx.Err()
	}
	first, second := <-errs, <-errs
	if first != nil {
		return first
	}
	return second
}

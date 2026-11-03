// Package proxy provides an authenticated SOCKS5 TCP/UDP gateway for owned cloud nodes.
// This is a standalone server and does not intercept other programs' network traffic.
package proxy

import (
	"context"
	"crypto/subtle"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"nodeconsole.local/node-console/internal/control"
)

type Server struct {
	cfg     control.ProxyConfig
	ln      net.Listener
	cancel  context.CancelFunc
	ctx     context.Context
	mu      sync.Mutex
	conns   map[net.Conn]struct{}
	wg      sync.WaitGroup
	active  atomic.Int64
	count   atomic.Uint64
	up      atomic.Uint64
	down    atomic.Uint64
	started string
	sem     chan struct{}
	allowed []*net.IPNet
}

type Service struct {
	mu        sync.Mutex
	server    *Server
	revision  int64
	state     string
	lastError string
}

func (s *Service) Snapshot() control.ProxyRuntime {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := control.ProxyRuntime{Status: s.state, Error: s.lastError, Revision: s.revision}
	if r.Status == "" {
		r.Status = "stopped"
	}
	if s.server == nil {
		return r
	}
	p := s.server
	r.Address = p.ln.Addr().String()
	r.Connections = p.active.Load()
	r.TotalConnections = p.count.Load()
	r.BytesUp = p.up.Load()
	r.BytesDown = p.down.Load()
	r.StartedAt = p.started
	return r
}
func (s *Service) Apply(revision int64, c control.ProxyConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.revision == revision && ((c.Enabled && s.state == "running") || (!c.Enabled && s.state == "stopped")) {
		return nil
	}
	// Preserve the current listener if the new config fails validation.
	if c.Enabled {
		if err := control.ValidateConfig(c); err != nil {
			s.lastError = err.Error()
			s.state = "error"
			return err
		}
	}
	old := s.server
	if old != nil {
		old.Stop()
		s.server = nil
	}
	s.revision = revision
	s.lastError = ""
	s.state = "stopped"
	if !c.Enabled {
		return nil
	}
	p, err := Start(c)
	if err != nil {
		s.state = "error"
		s.lastError = err.Error()
		return err
	}
	s.server = p
	s.state = "running"
	return nil
}
func (s *Service) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.server != nil {
		s.server.Stop()
		s.server = nil
	}
	s.state = "stopped"
}

func Start(c control.ProxyConfig) (*Server, error) {
	if err := control.ValidateConfig(c); err != nil {
		return nil, err
	}
	var nets []*net.IPNet
	for _, v := range c.AllowedCIDRs {
		_, n, _ := net.ParseCIDR(v)
		nets = append(nets, n)
	}
	ln, err := net.Listen("tcp", net.JoinHostPort(c.Listen, strconv.Itoa(c.Port)))
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	p := &Server{cfg: c, ln: ln, cancel: cancel, ctx: ctx, conns: map[net.Conn]struct{}{}, sem: make(chan struct{}, c.MaxConnections), allowed: nets, started: time.Now().UTC().Format(time.RFC3339)}
	p.wg.Add(1)
	go p.accept()
	return p, nil
}
func (p *Server) Stop() {
	p.cancel()
	_ = p.ln.Close()
	p.mu.Lock()
	for c := range p.conns {
		_ = c.Close()
	}
	p.mu.Unlock()
	p.wg.Wait()
}
func (p *Server) accept() {
	defer p.wg.Done()
	for {
		c, err := p.ln.Accept()
		if err != nil {
			if p.ctx.Err() != nil {
				return
			}
			time.Sleep(100 * time.Millisecond)
			continue
		}
		select {
		case p.sem <- struct{}{}:
		default:
			_ = c.Close()
			continue
		}
		p.mu.Lock()
		p.conns[c] = struct{}{}
		p.mu.Unlock()
		p.active.Add(1)
		p.count.Add(1)
		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			defer func() {
				_ = c.Close()
				p.mu.Lock()
				delete(p.conns, c)
				p.mu.Unlock()
				<-p.sem
				p.active.Add(-1)
			}()
			p.serve(c)
		}()
	}
}
func readByte(r io.Reader) (byte, error) {
	var a [1]byte
	_, err := io.ReadFull(r, a[:])
	return a[0], err
}
func (p *Server) clientAllowed(c net.Conn) bool {
	if len(p.allowed) == 0 {
		return true
	}
	host, _, err := net.SplitHostPort(c.RemoteAddr().String())
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	for _, n := range p.allowed {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}
func (p *Server) authenticate(c net.Conn) error {
	_ = c.SetDeadline(time.Now().Add(12 * time.Second))
	var h [2]byte
	if _, e := io.ReadFull(c, h[:]); e != nil {
		return e
	}
	if h[0] != 5 || h[1] == 0 {
		return errors.New("invalid SOCKS version")
	}
	methods := make([]byte, int(h[1]))
	if _, e := io.ReadFull(c, methods); e != nil {
		return e
	}
	found := false
	for _, m := range methods {
		if m == 2 {
			found = true
		}
	}
	if !found {
		_, _ = c.Write([]byte{5, 255})
		return errors.New("password authentication required")
	}
	if _, e := c.Write([]byte{5, 2}); e != nil {
		return e
	}
	ver, e := readByte(c)
	if e != nil {
		return e
	}
	if ver != 1 {
		return errors.New("invalid auth version")
	}
	ulen, e := readByte(c)
	if e != nil {
		return e
	}
	if ulen == 0 {
		return errors.New("empty user")
	}
	username := make([]byte, int(ulen))
	if _, e = io.ReadFull(c, username); e != nil {
		return e
	}
	plen, e := readByte(c)
	if e != nil {
		return e
	}
	if plen == 0 {
		return errors.New("empty password")
	}
	password := make([]byte, int(plen))
	if _, e = io.ReadFull(c, password); e != nil {
		return e
	}
	valid := subtle.ConstantTimeCompare(username, []byte(p.cfg.Username)) == 1 && subtle.ConstantTimeCompare(password, []byte(p.cfg.Password)) == 1
	if !valid {
		_, _ = c.Write([]byte{1, 1})
		return errors.New("unauthorized SOCKS5 login")
	}
	_, e = c.Write([]byte{1, 0})
	return e
}
func readTarget(r io.Reader) (byte, string, int, error) {
	typ, e := readByte(r)
	if e != nil {
		return 0, "", 0, e
	}
	var host string
	switch typ {
	case 1:
		b := make([]byte, 4)
		if _, e = io.ReadFull(r, b); e != nil {
			return 0, "", 0, e
		}
		host = net.IP(b).String()
	case 4:
		b := make([]byte, 16)
		if _, e = io.ReadFull(r, b); e != nil {
			return 0, "", 0, e
		}
		host = net.IP(b).String()
	case 3:
		l, err := readByte(r)
		if err != nil {
			return 0, "", 0, err
		}
		if l == 0 {
			return 0, "", 0, errors.New("invalid domain")
		}
		b := make([]byte, int(l))
		if _, e = io.ReadFull(r, b); e != nil {
			return 0, "", 0, e
		}
		host = string(b)
	default:
		return 0, "", 0, errors.New("unsupported SOCKS address type")
	}
	var port [2]byte
	if _, e = io.ReadFull(r, port[:]); e != nil {
		return 0, "", 0, e
	}
	return typ, host, int(binary.BigEndian.Uint16(port[:])), nil
}
func writeReply(c net.Conn, code byte, addr net.Addr) error {
	ip := net.IPv4zero
	port := 0
	switch a := addr.(type) {
	case *net.TCPAddr:
		ip = a.IP
		port = a.Port
	case *net.UDPAddr:
		ip = a.IP
		port = a.Port
	}
	if ip == nil {
		ip = net.IPv4zero
	}
	var msg []byte
	if v := ip.To4(); v != nil {
		msg = append([]byte{5, code, 0, 1}, v...)
	} else {
		msg = append([]byte{5, code, 0, 4}, ip.To16()...)
	}
	msg = append(msg, byte(port>>8), byte(port))
	_, e := c.Write(msg)
	return e
}
func publicIP(ip net.IP) bool {
	a, ok := netip.AddrFromSlice(ip)
	if !ok {
		return false
	}
	a = a.Unmap()
	if !a.IsGlobalUnicast() || a.IsPrivate() || a.IsLoopback() || a.IsLinkLocalUnicast() {
		return false
	}
	// Explicitly exclude shared, documentation and non-public IPv4 ranges.
	blocked := []string{"0.0.0.0/8", "100.64.0.0/10", "169.254.0.0/16", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "224.0.0.0/4", "240.0.0.0/4", "2001:db8::/32"}
	for _, v := range blocked {
		prefix := netip.MustParsePrefix(v)
		if prefix.Contains(a) {
			return false
		}
	}
	return true
}
func resolvePublic(ctx context.Context, host string) (net.IP, error) {
	if ip := net.ParseIP(host); ip != nil {
		if !publicIP(ip) {
			return nil, errors.New("target address is not public")
		}
		return ip, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	ips, e := net.DefaultResolver.LookupIPAddr(ctx, host)
	if e != nil {
		return nil, e
	}
	for _, ip := range ips {
		if publicIP(ip.IP) {
			return ip.IP, nil
		}
	}
	return nil, errors.New("no public address found for target")
}
func (p *Server) serve(c net.Conn) {
	if !p.clientAllowed(c) {
		return
	}
	if err := p.authenticate(c); err != nil {
		return
	}
	var head [3]byte
	if _, e := io.ReadFull(c, head[:]); e != nil {
		return
	}
	if head[0] != 5 || head[2] != 0 {
		_ = writeReply(c, 1, nil)
		return
	}
	_, host, port, e := readTarget(c)
	if e != nil {
		_ = writeReply(c, 8, nil)
		return
	}
	switch head[1] {
	case 1:
		p.connect(c, host, port)
	case 3:
		if !p.cfg.UDP {
			_ = writeReply(c, 7, nil)
			return
		}
		p.associateUDP(c)
	default:
		_ = writeReply(c, 7, nil)
	}
}

// countWriter updates the byte counters as payload is transmitted instead of
// waiting for a long-lived TCP session to close.
type countWriter struct {
	dst     io.Writer
	counter *atomic.Uint64
}

func (w countWriter) Write(b []byte) (int, error) {
	n, err := w.dst.Write(b)
	if n > 0 {
		w.counter.Add(uint64(n))
	}
	return n, err
}

func (p *Server) connect(c net.Conn, host string, port int) {
	if port == 0 {
		_ = writeReply(c, 8, nil)
		return
	}
	ctx, cancel := context.WithTimeout(p.ctx, 8*time.Second)
	defer cancel()
	ip, err := resolvePublic(ctx, host)
	if err != nil {
		_ = writeReply(c, 2, nil)
		return
	}
	dest := net.JoinHostPort(ip.String(), strconv.Itoa(port))
	out, e := (&net.Dialer{Timeout: 8 * time.Second, KeepAlive: 30 * time.Second}).DialContext(ctx, "tcp", dest)
	if e != nil {
		_ = writeReply(c, 5, nil)
		return
	}
	defer out.Close()
	if err = writeReply(c, 0, out.LocalAddr()); err != nil {
		return
	}
	_ = c.SetDeadline(time.Time{})
	var wg sync.WaitGroup
	stopOut := make(chan struct{})
	go func() {
		select {
		case <-p.ctx.Done():
			_ = out.Close()
		case <-stopOut:
		}
	}()
	defer close(stopOut)
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(countWriter{dst: out, counter: &p.up}, c)
		if t, ok := out.(*net.TCPConn); ok {
			_ = t.CloseWrite()
		}
	}()
	go func() {
		defer wg.Done()
		_, _ = io.Copy(countWriter{dst: c, counter: &p.down}, out)
		if t, ok := c.(*net.TCPConn); ok {
			_ = t.CloseWrite()
		}
	}()
	wg.Wait()
}
func addrHeader(ip net.IP, port int) []byte {
	var b []byte
	if v := ip.To4(); v != nil {
		b = append([]byte{0, 0, 0, 1}, v...)
	} else {
		b = append([]byte{0, 0, 0, 4}, ip.To16()...)
	}
	return append(b, byte(port>>8), byte(port))
}
func parseUDP(dat []byte) (string, int, []byte, error) {
	if len(dat) < 7 || dat[0] != 0 || dat[1] != 0 || dat[2] != 0 {
		return "", 0, nil, errors.New("unsupported fragmented UDP")
	}
	_, host, port, e := readTarget(strings.NewReader(string(dat[3:])))
	if e != nil {
		return "", 0, nil, e
	}
	off := 4
	switch dat[3] {
	case 1:
		off += 4
	case 4:
		off += 16
	case 3:
		off += 1 + int(dat[4])
	default:
		return "", 0, nil, errors.New("address type")
	}
	off += 2
	if off > len(dat) {
		return "", 0, nil, errors.New("invalid UDP header")
	}
	return host, port, dat[off:], nil
}
func (p *Server) associateUDP(c net.Conn) {
	local := c.LocalAddr().(*net.TCPAddr).IP
	if local == nil || local.IsUnspecified() {
		local = net.IPv4zero
	}
	sock, e := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	if e != nil {
		_ = writeReply(c, 1, nil)
		return
	}
	defer sock.Close()
	if e = writeReply(c, 0, &net.UDPAddr{IP: local, Port: sock.LocalAddr().(*net.UDPAddr).Port}); e != nil {
		return
	}
	_ = c.SetDeadline(time.Time{})
	done := make(chan struct{})
	go func() { _, _ = io.Copy(io.Discard, c); close(done); _ = sock.Close() }()
	clientIP := c.RemoteAddr().(*net.TCPAddr).IP
	var clientAddr atomic.Pointer[net.UDPAddr]
	// Each UDP target has its own connected socket so responses cannot be spoofed.
	type peer struct {
		conn *net.UDPConn
		last atomic.Int64
	}
	peers := map[string]*peer{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	defer func() {
		mu.Lock()
		for _, v := range peers {
			_ = v.conn.Close()
		}
		mu.Unlock()
		wg.Wait()
	}()
	buf := make([]byte, 65535)
	for {
		_ = sock.SetReadDeadline(time.Now().Add(time.Second))
		n, src, err := sock.ReadFromUDP(buf)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				select {
				case <-done:
					return
				case <-p.ctx.Done():
					return
				default:
					continue
				}
			}
			return
		}
		if !src.IP.Equal(clientIP) {
			continue
		}
		if clientAddr.Load() == nil {
			clientAddr.CompareAndSwap(nil, src)
		}
		pinned := clientAddr.Load()
		if pinned == nil || !src.IP.Equal(pinned.IP) || src.Port != pinned.Port {
			continue
		}
		host, port, payload, e := parseUDP(buf[:n])
		if e != nil || port == 0 {
			continue
		}
		key := net.JoinHostPort(host, strconv.Itoa(port))
		mu.Lock()
		target := peers[key]
		mu.Unlock()
		if target == nil {
			mu.Lock()
			for k, pr := range peers {
				if time.Now().Unix()-pr.last.Load() > 35 {
					_ = pr.conn.Close()
					delete(peers, k)
				}
			}
			full := len(peers) >= 64
			mu.Unlock()
			if full {
				continue
			}
			ctx, cancel := context.WithTimeout(p.ctx, 4*time.Second)
			ip, e := resolvePublic(ctx, host)
			cancel()
			if e != nil {
				continue
			}
			remote := &net.UDPAddr{IP: ip, Port: port}
			out, e := net.DialUDP("udp", nil, remote)
			if e != nil {
				continue
			}
			target = &peer{conn: out}
			target.last.Store(time.Now().Unix())
			mu.Lock()
			peers[key] = target
			mu.Unlock()
			wg.Add(1)
			go func(pr *peer, remote *net.UDPAddr) {
				defer wg.Done()
				resp := make([]byte, 65535)
				for {
					_ = pr.conn.SetReadDeadline(time.Now().Add(35 * time.Second))
					sz, e := pr.conn.Read(resp)
					if e != nil {
						return
					}
					pinned := clientAddr.Load()
					if pinned == nil {
						return
					}
					msg := append(addrHeader(remote.IP, remote.Port), resp[:sz]...)
					wn, e := sock.WriteToUDP(msg, pinned)
					if e != nil {
						return
					}
					p.down.Add(uint64(wn))
					pr.last.Store(time.Now().Unix())
				}
			}(target, remote)
		}
		wn, e := target.conn.Write(payload)
		if e == nil {
			p.up.Add(uint64(wn))
			target.last.Store(time.Now().Unix())
		}
	}
}
func PublicTarget(addr string) bool { return publicIP(net.ParseIP(addr)) }
func (s *Service) DebugString() string {
	r := s.Snapshot()
	return fmt.Sprintf("%s %s (%d)", r.Status, r.Address, r.Connections)
}

// Package proxy provides an authenticated SOCKS5 TCP/UDP gateway for owned cloud nodes.
// This is a standalone server and does not intercept other programs' network traffic.
package proxy

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/walkmiao/nodeswarm/internal/control"
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
	udpMu   sync.Mutex
	udpUsed map[int]struct{}
}

type Service struct {
	mu        sync.Mutex
	server    *Server
	revision  int64
	state     string
	lastError string
	// starter is injectable by tests to exercise a bind failure after the old
	// listener has been closed. Production always calls Start.
	starter func(control.ProxyConfig) (*Server, error)
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

// Apply advances the acknowledged revision only after the new configuration
// has actually started. A failed update preserves (or restarts) the previous
// listener, so Controller can distinguish the live revision from desired state.
func (s *Service) Apply(revision int64, c control.ProxyConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.revision == revision && ((c.Enabled && s.state == "running") || (!c.Enabled && s.state == "stopped")) {
		return nil
	}
	if c.Enabled {
		if err := control.ValidateConfig(c); err != nil {
			s.lastError = "configuration rejected: " + err.Error()
			if s.server == nil {
				s.state = "error"
			}
			return err
		}
	}
	start := s.starter
	if start == nil {
		start = Start
	}
	old := s.server
	if !c.Enabled {
		if old != nil {
			old.Stop()
		}
		s.server = nil
		s.revision = revision
		s.lastError = ""
		s.state = "stopped"
		return nil
	}

	// A changed socket can be bound and accepted before the existing listener
	// closes. If it is busy, reject the change without disturbing old users.
	if old == nil || !sameListenSocket(old.cfg, c) {
		next, err := start(c)
		if err != nil {
			s.lastError = "configuration rejected: " + err.Error()
			if old == nil {
				s.state = "error"
			}
			return err
		}
		s.server = next
		s.revision = revision
		s.lastError = ""
		s.state = "running"
		if old != nil {
			old.Stop()
		}
		return nil
	}

	// A same-port update cannot open two listeners simultaneously. Close the
	// old listener, attempt the replacement, and recover the exact old config
	// if the new listener fails. Never claim the desired revision on rollback.
	oldConfig := old.cfg
	old.Stop()
	s.server = nil
	next, err := start(c)
	if err == nil {
		s.server = next
		s.revision = revision
		s.lastError = ""
		s.state = "running"
		return nil
	}
	recovered, recoverErr := start(oldConfig)
	if recoverErr != nil {
		s.state = "error"
		s.lastError = fmt.Sprintf("configuration failed: %v; rollback failed: %v", err, recoverErr)
		return fmt.Errorf("%s", s.lastError)
	}
	s.server = recovered
	s.state = "running"
	s.lastError = "configuration rejected; prior listener restored: " + err.Error()
	return err
}

func sameListenSocket(a, b control.ProxyConfig) bool {
	return a.Listen == b.Listen && a.Port == b.Port
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
	p := &Server{cfg: c, ln: ln, cancel: cancel, ctx: ctx, conns: map[net.Conn]struct{}{}, sem: make(chan struct{}, c.MaxConnections), allowed: nets, udpUsed: make(map[int]struct{}), started: time.Now().UTC().Format(time.RFC3339)}
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
		p.associateUDP(c, host, port)
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
	_, host, port, e := readTarget(bytes.NewReader(dat[3:]))
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

// openUDPRelay acquires a dedicated port for one authenticated UDP ASSOCIATE
// session. A bounded pool is useful for cloud security groups / firewalls.
// An empty pool retains ephemeral OS-assigned UDP ports for backward compatibility.
func (p *Server) openUDPRelay() (*net.UDPConn, func(), error) {
	listenIP := net.ParseIP(p.cfg.Listen)
	if listenIP == nil {
		listenIP = net.IPv4zero
	}
	if p.cfg.UDPPortStart == 0 {
		sock, err := net.ListenUDP("udp", &net.UDPAddr{IP: listenIP, Port: 0})
		return sock, func() {}, err
	}
	p.udpMu.Lock()
	defer p.udpMu.Unlock()
	for port := p.cfg.UDPPortStart; port <= p.cfg.UDPPortEnd; port++ {
		if _, exists := p.udpUsed[port]; exists {
			continue
		}
		sock, err := net.ListenUDP("udp", &net.UDPAddr{IP: listenIP, Port: port})
		if err != nil {
			continue
		}
		p.udpUsed[port] = struct{}{}
		return sock, func() {
			p.udpMu.Lock()
			delete(p.udpUsed, port)
			p.udpMu.Unlock()
		}, nil
	}
	return nil, nil, errors.New("UDP relay pool exhausted or ports unavailable")
}

// SOCKS5 UDP BND.ADDR must be reachable by the client. On a cloud VM the
// socket's local IP is often RFC1918 behind NAT, so advertise PublicHost when
// configured. RFC1928 permits an IPv4, IPv6 or a DNS-name reply.
func writeUDPAssociateReply(c net.Conn, publicHost string, port int) error {
	host := publicHost
	if host == "" {
		if local, ok := c.LocalAddr().(*net.TCPAddr); ok {
			host = local.IP.String()
		}
	}
	var reply []byte
	if ip := net.ParseIP(host); ip != nil {
		if v4 := ip.To4(); v4 != nil {
			reply = append([]byte{5, 0, 0, 1}, v4...)
		} else {
			reply = append([]byte{5, 0, 0, 4}, ip.To16()...)
		}
	} else if host != "" && len(host) <= 255 {
		reply = append([]byte{5, 0, 0, 3, byte(len(host))}, []byte(host)...)
	} else {
		reply = []byte{5, 0, 0, 1, 0, 0, 0, 0}
	}
	reply = append(reply, byte(port>>8), byte(port))
	_, err := c.Write(reply)
	return err
}

// validateUDPAssociateSource applies RFC1928's optional client UDP endpoint.
// An unspecified address/port means the server must learn the source from the
// first UDP datagram, but that datagram must still have the TCP peer's IP.
// Domain-name claims are rejected rather than being used for attribution.
func validateUDPAssociateSource(peerIP net.IP, host string, port int) error {
	if peerIP == nil || port < 0 || port > 65535 {
		return errors.New("invalid UDP association peer or port")
	}
	want := net.ParseIP(host)
	if want == nil {
		return errors.New("UDP association source must be a numeric address")
	}
	if !want.IsUnspecified() && !want.Equal(peerIP) {
		return errors.New("UDP association address does not match authenticated TCP peer")
	}
	return nil
}

// A nonzero port in UDP ASSOCIATE pins the authenticated UDP source port.
// When the client leaves the port unspecified, the first valid datagram pins
// the port for the life of this TCP-authenticated association.
func udpSourceMatches(src *net.UDPAddr, tcpIP net.IP, requestedPort int, pinned *net.UDPAddr) bool {
	if src == nil || tcpIP == nil || !src.IP.Equal(tcpIP) || (requestedPort != 0 && src.Port != requestedPort) {
		return false
	}
	return pinned == nil || (src.IP.Equal(pinned.IP) && src.Port == pinned.Port)
}

func (p *Server) associateUDP(c net.Conn, requestedHost string, requestedPort int) {
	tcpPeer, ok := c.RemoteAddr().(*net.TCPAddr)
	if !ok || validateUDPAssociateSource(tcpPeer.IP, requestedHost, requestedPort) != nil {
		_ = writeReply(c, 2, nil)
		return
	}
	sock, release, e := p.openUDPRelay()
	if e != nil {
		_ = writeReply(c, 1, nil)
		return
	}
	defer sock.Close()
	defer release()
	if e = writeUDPAssociateReply(c, p.cfg.PublicHost, sock.LocalAddr().(*net.UDPAddr).Port); e != nil {
		return
	}
	_ = c.SetDeadline(time.Time{})
	done := make(chan struct{})
	go func() { _, _ = io.Copy(io.Discard, c); close(done); _ = sock.Close() }()
	clientIP := tcpPeer.IP
	var clientAddr atomic.Pointer[net.UDPAddr]
	// Each UDP target has its own connected socket so responses cannot be spoofed.
	type peer struct {
		conn *net.UDPConn
		last atomic.Int64
	}
	peers := map[string]*peer{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	// Tear down peer UDP sockets immediately on service cancellation instead
	// of waiting for their 35s receive deadlines during remote restart.
	stopWatch := make(chan struct{})
	go func() {
		select {
		case <-p.ctx.Done():
			mu.Lock()
			for _, peer := range peers {
				_ = peer.conn.Close()
			}
			mu.Unlock()
			_ = sock.Close()
		case <-stopWatch:
		}
	}()
	defer close(stopWatch)
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
		if !udpSourceMatches(src, clientIP, requestedPort, clientAddr.Load()) {
			continue
		}
		if clientAddr.Load() == nil {
			clientAddr.CompareAndSwap(nil, src)
		}
		if !udpSourceMatches(src, clientIP, requestedPort, clientAddr.Load()) {
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

package manager

import (
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"time"
)

// ProbeSOCKS5 verifies an actual SOCKS5 handshake and TCP CONNECT to the
// fixed public endpoint 1.1.1.1:443. It does NOT prove UDP support, a stable
// egress IP, or compatibility with any particular game or application.
func ProbeSOCKS5(n Node, timeout time.Duration) Probe {
	checked := time.Now().Format(time.RFC3339)
	t0 := time.Now()
	result := Probe{Status: "failed", CheckedAt: checked}
	fail := func(err error) Probe {
		result.Detail = err.Error()
		result.LatencyMs = time.Since(t0).Milliseconds()
		return result
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(n.Host, strconv.Itoa(n.Port)), timeout)
	if err != nil {
		return fail(fmt.Errorf("代理服务器 TCP 连接失败: %w", err))
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(timeout))
	method := byte(0x00)
	if n.Username != "" || n.Password != "" {
		method = 0x02
	}
	if _, err = conn.Write([]byte{0x05, 0x01, method}); err != nil {
		return fail(fmt.Errorf("SOCKS5 握手写入失败: %w", err))
	}
	selected := make([]byte, 2)
	if _, err = io.ReadFull(conn, selected); err != nil {
		return fail(fmt.Errorf("SOCKS5 握手读取失败: %w", err))
	}
	if selected[0] != 0x05 || selected[1] != method {
		return fail(errors.New("SOCKS5 服务端不接受当前认证方式"))
	}
	if method == 0x02 {
		auth := []byte{0x01, byte(len(n.Username))}
		auth = append(auth, []byte(n.Username)...)
		auth = append(auth, byte(len(n.Password)))
		auth = append(auth, []byte(n.Password)...)
		if _, err = conn.Write(auth); err != nil {
			return fail(errors.New("SOCKS5 账号认证写入失败"))
		}
		status := make([]byte, 2)
		if _, err = io.ReadFull(conn, status); err != nil {
			return fail(errors.New("SOCKS5 账号认证响应失败"))
		}
		if status[0] != 0x01 || status[1] != 0 {
			return fail(errors.New("SOCKS5 用户名或密码认证失败"))
		}
	}
	// Fixed known endpoint, not an arbitrary API-controlled network scanner.
	// 05(version) 01(CONNECT) 00(reserved) 01(IPv4) 1.1.1.1 443
	req := []byte{0x05, 0x01, 0x00, 0x01, 1, 1, 1, 1, 0x01, 0xBB}
	if _, err = conn.Write(req); err != nil {
		return fail(errors.New("SOCKS5 CONNECT 请求失败"))
	}
	head := make([]byte, 4)
	if _, err = io.ReadFull(conn, head); err != nil {
		return fail(fmt.Errorf("SOCKS5 CONNECT 响应失败: %w", err))
	}
	if head[0] != 0x05 || head[2] != 0x00 {
		return fail(errors.New("SOCKS5 CONNECT 返回了无效响应"))
	}
	if head[1] != 0 {
		return fail(fmt.Errorf("SOCKS5 CONNECT 被代理拒绝，状态码 0x%02x", head[1]))
	}
	var remain int64
	switch head[3] {
	case 0x01:
		remain = 4 + 2
	case 0x04:
		remain = 16 + 2
	case 0x03:
		size := make([]byte, 1)
		if _, err = io.ReadFull(conn, size); err != nil {
			return fail(errors.New("代理返回的域名长度不完整"))
		}
		remain = int64(size[0]) + 2
	default:
		return fail(errors.New("代理返回了不支持的地址类型"))
	}
	if _, err = io.CopyN(io.Discard, conn, remain); err != nil {
		return fail(errors.New("代理返回的 BND 地址不完整"))
	}
	result.OK = true
	result.Status = "tcp_ok"
	result.LatencyMs = time.Since(t0).Milliseconds()
	result.Detail = "SOCKS5 握手、认证及 TCP CONNECT 到 1.1.1.1:443 成功；UDP 和目标应用连接尚未验证"
	return result
}

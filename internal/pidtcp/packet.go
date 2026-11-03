package pidtcp

import (
	"encoding/binary"
	"errors"
	"net/netip"
)

type flowKey struct{ local, remote netip.AddrPort }
type tcpPacket struct {
	data                     []byte
	key                      flowKey
	ipSrc, ipDst, tcp, width int
	syn, ack, rst            bool
	seq                      uint32
}

// decodeTCP rejects fragments and unrecognized IPv6 extension headers instead
// of corrupting packets. Unsupported packets must never be given a NAT mapping.
func decodeTCP(b []byte) (p tcpPacket, err error) {
	invalid := func() (tcpPacket, error) { return tcpPacket{}, errors.New("unsupported TCP packet") }
	if len(b) < 20 {
		return invalid()
	}
	p.data = b
	switch b[0] >> 4 {
	case 4:
		h := int(b[0]&15) * 4
		if h < 20 || h > len(b) || int(binary.BigEndian.Uint16(b[2:4])) != len(b) || b[9] != 6 || binary.BigEndian.Uint16(b[6:8])&0x3fff != 0 {
			return invalid()
		}
		p.ipSrc, p.ipDst, p.tcp, p.width = 12, 16, h, 4
	case 6:
		if len(b) < 40 || int(binary.BigEndian.Uint16(b[4:6]))+40 != len(b) {
			return invalid()
		}
		p.ipSrc, p.ipDst, p.tcp, p.width = 8, 24, 40, 16
		next := b[6]
		for next != 6 {
			if next != 0 && next != 60 {
				return invalid()
			}
			if p.tcp+2 > len(b) {
				return invalid()
			}
			n := (int(b[p.tcp+1]) + 1) * 8
			next = b[p.tcp]
			p.tcp += n
			if p.tcp > len(b) {
				return invalid()
			}
		}
	default:
		return invalid()
	}
	if p.tcp+20 > len(b) {
		return invalid()
	}
	h := int(b[p.tcp+12]>>4) * 4
	if h < 20 || p.tcp+h > len(b) {
		return invalid()
	}
	from, _ := netip.AddrFromSlice(b[p.ipSrc : p.ipSrc+p.width])
	to, _ := netip.AddrFromSlice(b[p.ipDst : p.ipDst+p.width])
	p.key = flowKey{local: netip.AddrPortFrom(from, binary.BigEndian.Uint16(b[p.tcp:])), remote: netip.AddrPortFrom(to, binary.BigEndian.Uint16(b[p.tcp+2:]))}
	p.syn = b[p.tcp+13]&2 != 0
	p.ack = b[p.tcp+13]&16 != 0
	p.rst = b[p.tcp+13]&4 != 0
	p.seq = binary.BigEndian.Uint32(b[p.tcp+4:])
	return p, nil
}
func (p tcpPacket) rewrite(from, to netip.AddrPort) {
	a := from.Addr().AsSlice()
	b := to.Addr().AsSlice()
	copy(p.data[p.ipSrc:p.ipSrc+p.width], a)
	copy(p.data[p.ipDst:p.ipDst+p.width], b)
	binary.BigEndian.PutUint16(p.data[p.tcp:], from.Port())
	binary.BigEndian.PutUint16(p.data[p.tcp+2:], to.Port())
}

//go:build windows && amd64

package pidtcp

import (
	"encoding/binary"
	"fmt"
	"net/netip"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

var (
	netAPI       = syscall.NewLazyDLL("iphlpapi.dll")
	tcpTableProc = netAPI.NewProc("GetExtendedTcpTable")
	winKernel    = syscall.NewLazyDLL("kernel32.dll")
	openProc     = winKernel.NewProc("OpenProcess")
	procTimes    = winKernel.NewProc("GetProcessTimes")
	imageName    = winKernel.NewProc("QueryFullProcessImageNameW")
)

const winEpochTicks = uint64(116444736000000000)

type filetime struct{ Low, High uint32 }

func ownerWindows(key flowKey) (uint32, error) {
	family := uintptr(2) // AF_INET
	rowSize := 24
	if key.local.Addr().Is6() {
		family = 23
		rowSize = 56
	} // AF_INET6
	var size uint32
	ret, _, _ := tcpTableProc.Call(0, uintptr(unsafe.Pointer(&size)), 0, family, 5, 0)
	if ret != uintptr(syscall.ERROR_INSUFFICIENT_BUFFER) && ret != 0 {
		return 0, fmt.Errorf("GetExtendedTcpTable sizing: %d", ret)
	}
	for trial := 0; trial < 4; trial++ {
		if size < 4 || size > 64*1024*1024 {
			return 0, fmt.Errorf("TCP table size out of range: %d", size)
		}
		b := make([]byte, size)
		ret, _, _ = tcpTableProc.Call(uintptr(unsafe.Pointer(&b[0])), uintptr(unsafe.Pointer(&size)), 0, family, 5, 0)
		if ret == uintptr(syscall.ERROR_INSUFFICIENT_BUFFER) {
			continue
		}
		if ret != 0 {
			return 0, fmt.Errorf("GetExtendedTcpTable: %d", ret)
		}
		count := int(binary.LittleEndian.Uint32(b[:4]))
		if count > (len(b)-4)/rowSize {
			return 0, fmt.Errorf("invalid TCP owner table row count")
		}
		for i := 0; i < count; i++ {
			row := b[4+i*rowSize : 4+(i+1)*rowSize]
			var local, remote netip.Addr
			var lport, rport uint16
			var pid uint32
			if rowSize == 24 {
				a := [4]byte{}
				c := [4]byte{}
				copy(a[:], row[4:8])
				copy(c[:], row[12:16])
				local = netip.AddrFrom4(a)
				remote = netip.AddrFrom4(c)
				lport = binary.BigEndian.Uint16(row[8:10])
				rport = binary.BigEndian.Uint16(row[16:18])
				pid = binary.LittleEndian.Uint32(row[20:24])
			} else {
				a := [16]byte{}
				c := [16]byte{}
				copy(a[:], row[0:16])
				copy(c[:], row[24:40])
				local = netip.AddrFrom16(a)
				remote = netip.AddrFrom16(c)
				lport = binary.BigEndian.Uint16(row[20:22])
				rport = binary.BigEndian.Uint16(row[44:46])
				pid = binary.LittleEndian.Uint32(row[52:56])
			}
			if local == key.local.Addr() && remote == key.remote.Addr() && lport == key.local.Port() && rport == key.remote.Port() {
				return pid, nil
			}
		}
		return 0, nil
	}
	return 0, fmt.Errorf("GetExtendedTcpTable constantly resized")
}
func lifetimeMatches(pid int, expectedStart, expectedName string) bool {
	if pid <= 0 {
		return false
	}
	// Query access also prevents PID reuse between initial packet attribution
	// and route selection. No executable-name-only fallback is used.
	h, _, _ := openProc.Call(0x1000 /* PROCESS_QUERY_LIMITED_INFORMATION */, 0, uintptr(pid))
	if h == 0 {
		return false
	}
	defer syscall.CloseHandle(syscall.Handle(h))
	var create, exit, kernel, user filetime
	ok, _, _ := procTimes.Call(h, uintptr(unsafe.Pointer(&create)), uintptr(unsafe.Pointer(&exit)), uintptr(unsafe.Pointer(&kernel)), uintptr(unsafe.Pointer(&user)))
	if ok == 0 {
		return false
	}
	ticks := uint64(create.High)<<32 | uint64(create.Low)
	if ticks <= winEpochTicks {
		return false
	}
	started := time.Unix(0, int64(ticks-winEpochTicks)*100).UTC()
	expected, err := time.Parse(time.RFC3339Nano, expectedStart)
	if err != nil || !expected.Equal(started) {
		return false
	}
	chars := make([]uint16, 32768)
	size := uint32(len(chars))
	ok, _, _ = imageName.Call(h, 0, uintptr(unsafe.Pointer(&chars[0])), uintptr(unsafe.Pointer(&size)))
	if ok == 0 || size == 0 || size > uint32(len(chars)) {
		return false
	}
	name := syscall.UTF16ToString(chars[:size])
	name = strings.ReplaceAll(name, "/", "\\")
	if n := strings.LastIndexByte(name, '\\'); n >= 0 {
		name = name[n+1:]
	}
	return strings.EqualFold(name, expectedName)
}

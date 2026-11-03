//go:build windows && amd64

package pidtcp

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"unsafe"
)

type winDriver struct {
	dll                                   *syscall.DLL
	handle                                uintptr
	recv, send, checksum, shutdown, close *syscall.Proc
	once                                  sync.Once
}

func loadDriver(dir string) (*winDriver, error) {
	if dir == "" {
		executable, err := os.Executable()
		if err != nil {
			return nil, err
		}
		dir = filepath.Dir(executable)
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	dllPath := filepath.Join(abs, "WinDivert.dll")
	driverPath := filepath.Join(abs, "WinDivert64.sys")
	if _, err = os.Stat(driverPath); err != nil {
		return nil, fmt.Errorf("WinDivert64.sys absent (administrator access and official WinDivert 2.2 x64 required): %w", err)
	}
	dll, err := syscall.LoadDLL(dllPath)
	if err != nil {
		return nil, fmt.Errorf("load trusted WinDivert.dll from %s: %w", dllPath, err)
	}
	d := &winDriver{dll: dll}
	defer func() {
		if err != nil {
			dll.Release()
		}
	}()
	for _, entry := range []struct {
		name string
		dest **syscall.Proc
	}{
		{"WinDivertRecv", &d.recv}, {"WinDivertSend", &d.send}, {"WinDivertHelperCalcChecksums", &d.checksum}, {"WinDivertShutdown", &d.shutdown}, {"WinDivertClose", &d.close},
	} {
		*entry.dest, err = dll.FindProc(entry.name)
		if err != nil {
			return nil, err
		}
	}
	open, err := dll.FindProc("WinDivertOpen")
	if err != nil {
		return nil, err
	}
	// Captures outbound TCP (including relay responses). Requires a network
	// driver, admin rights, and all matched packets to be reinjected.
	filter, err := syscall.BytePtrFromString("outbound and tcp and !loopback")
	if err != nil {
		return nil, err
	}
	h, _, callErr := open.Call(uintptr(unsafe.Pointer(filter)), 0 /* NETWORK layer */, 0 /* priority */, 0)
	if h == ^uintptr(0) {
		return nil, fmt.Errorf("WinDivertOpen failed (run as Administrator): %w", callErr)
	}
	d.handle = h
	return d, nil
}
func (d *winDriver) receive(buf []byte) (int, packetAddress, error) {
	var out packetAddress
	var n uint32
	ret, _, err := d.recv.Call(d.handle, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), uintptr(unsafe.Pointer(&n)), uintptr(unsafe.Pointer(&out.raw[0])))
	if ret == 0 {
		return 0, out, err
	}
	return int(n), out, nil
}
func (d *winDriver) Send(buf []byte, addr packetAddress, modified bool) error {
	if len(buf) == 0 {
		return errors.New("empty captured packet")
	}
	if modified {
		addr.clearOutbound()
		ret, _, err := d.checksum.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), uintptr(unsafe.Pointer(&addr.raw[0])), 0)
		if ret == 0 {
			return fmt.Errorf("WinDivert checksum repair: %w", err)
		}
	}
	var n uint32
	ret, _, err := d.send.Call(d.handle, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), uintptr(unsafe.Pointer(&n)), uintptr(unsafe.Pointer(&addr.raw[0])))
	if ret == 0 {
		return fmt.Errorf("WinDivert reinject: %w", err)
	}
	if int(n) != len(buf) {
		return fmt.Errorf("WinDivert reinjected %d of %d bytes", n, len(buf))
	}
	return nil
}
func (d *winDriver) stop() {
	d.shutdown.Call(d.handle, 1)
	d.once.Do(func() { d.close.Call(d.handle) })
}
func (d *winDriver) dispose() { d.stop(); d.dll.Release() }

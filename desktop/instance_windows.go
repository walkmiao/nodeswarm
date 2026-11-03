//go:build windows

package main

import (
	"errors"
	"fmt"
	"syscall"
	"unsafe"
)

var (
	desktopKernel32    = syscall.NewLazyDLL("kernel32.dll")
	desktopCreateMutex = desktopKernel32.NewProc("CreateMutexW")
	desktopCloseHandle = desktopKernel32.NewProc("CloseHandle")
)

// One interactive desktop instance per Windows logon session. A second
// process must never start another heartbeat/ProxyBridge owner using the same
// per-user device credential or it could fight over the active policy.
func acquireAppLock() (func(), error) {
	name, err := syscall.UTF16PtrFromString(`Local\NodeSwarm-Desktop`)
	if err != nil {
		return nil, err
	}
	handle, _, callErr := desktopCreateMutex.Call(0, 0, uintptr(unsafe.Pointer(name)))
	if handle == 0 {
		return nil, fmt.Errorf("cannot create desktop singleton mutex: %w", callErr)
	}
	if callErr == syscall.ERROR_ALREADY_EXISTS {
		desktopCloseHandle.Call(handle)
		return nil, errors.New("NodeSwarm Desktop 已在当前 Windows 会话运行，请切换到现有窗口")
	}
	return func() { desktopCloseHandle.Call(handle) }, nil
}

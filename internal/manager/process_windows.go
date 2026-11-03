//go:build windows

package manager

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"syscall"
	"time"
	"unsafe"
)

// Windows process discovery uses native Toolhelp/Kernel32 APIs. Unlike launching
// PowerShell every 12 seconds, this does not spawn a shell and provides a
// stable creation-time identity for PID reuse detection. The PowerShell path is
// kept as a fallback on restricted Windows installations.
const (
	th32csSnapProcess              = 0x00000002
	processQueryLimitedInformation = 0x1000
	windowsUnixEpochOffset100ns    = uint64(116444736000000000)
)

var (
	kernel32                   = syscall.NewLazyDLL("kernel32.dll")
	createToolhelp32Snapshot   = kernel32.NewProc("CreateToolhelp32Snapshot")
	process32FirstW            = kernel32.NewProc("Process32FirstW")
	process32NextW             = kernel32.NewProc("Process32NextW")
	openProcess                = kernel32.NewProc("OpenProcess")
	getProcessTimes            = kernel32.NewProc("GetProcessTimes")
	queryFullProcessImageNameW = kernel32.NewProc("QueryFullProcessImageNameW")
)

type processEntry32W struct {
	Size            uint32
	Usage           uint32
	ProcessID       uint32
	DefaultHeapID   uintptr
	ModuleID        uint32
	Threads         uint32
	ParentProcessID uint32
	PriClassBase    int32
	Flags           uint32
	ExeFile         [260]uint16
}
type winFiletime struct {
	Low  uint32
	High uint32
}

func createdAt(h syscall.Handle) string {
	var created, exited, kernel, user winFiletime
	ok, _, _ := getProcessTimes.Call(uintptr(h), uintptr(unsafe.Pointer(&created)), uintptr(unsafe.Pointer(&exited)), uintptr(unsafe.Pointer(&kernel)), uintptr(unsafe.Pointer(&user)))
	if ok == 0 {
		return ""
	}
	ticks := uint64(created.High)<<32 | uint64(created.Low)
	if ticks <= windowsUnixEpochOffset100ns {
		return ""
	}
	nano := int64(ticks-windowsUnixEpochOffset100ns) * 100
	return time.Unix(0, nano).UTC().Format(time.RFC3339Nano)
}
func processPath(h syscall.Handle) string {
	buf := make([]uint16, 32768)
	size := uint32(len(buf))
	ok, _, _ := queryFullProcessImageNameW.Call(uintptr(h), 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)))
	if ok == 0 || size == 0 || size > uint32(len(buf)) {
		return ""
	}
	return syscall.UTF16ToString(buf[:size])
}
func listNativeProcesses() ([]Process, error) {
	h, _, err := createToolhelp32Snapshot.Call(th32csSnapProcess, 0)
	if h == ^uintptr(0) {
		return nil, fmt.Errorf("CreateToolhelp32Snapshot: %w", err)
	}
	defer syscall.CloseHandle(syscall.Handle(h))
	var entry processEntry32W
	entry.Size = uint32(unsafe.Sizeof(entry))
	ok, _, _ := process32FirstW.Call(h, uintptr(unsafe.Pointer(&entry)))
	if ok == 0 {
		return nil, errors.New("Process32FirstW failed")
	}
	out := make([]Process, 0, 128)
	for ok != 0 {
		pid := int(entry.ProcessID)
		name := syscall.UTF16ToString(entry.ExeFile[:])
		if pid > 0 && name != "" {
			handle, _, _ := openProcess.Call(processQueryLimitedInformation, 0, uintptr(entry.ProcessID))
			if handle != 0 {
				start := createdAt(syscall.Handle(handle))
				path := processPath(syscall.Handle(handle))
				_ = syscall.CloseHandle(syscall.Handle(handle))
				if start != "" {
					out = append(out, Process{PID: pid, Name: name, Path: path, StartTime: start})
				}
			}
		}
		entry.Size = uint32(unsafe.Sizeof(entry))
		ok, _, _ = process32NextW.Call(h, uintptr(unsafe.Pointer(&entry)))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].PID < out[j].PID })
	if len(out) == 0 {
		return nil, errors.New("native process inventory empty")
	}
	return out, nil
}

func ListProcesses() ([]Process, error) {
	if processes, err := listNativeProcesses(); err == nil {
		return processes, nil
	}
	return listPowershellProcesses()
}

// Fallback for hosts where Toolhelp/process-query APIs are unexpectedly blocked.
func listPowershellProcesses() ([]Process, error) {
	script := `$ErrorActionPreference='SilentlyContinue'; @(Get-Process | ForEach-Object { $st=''; try { $st=$_.StartTime.ToUniversalTime().ToString('o') } catch {}; $p=''; try { $p=$_.Path } catch {}; [pscustomobject]@{pid=$_.Id; name=($_.ProcessName+'.exe'); path=$p; startTime=$st} }) | ConvertTo-Json -Compress -Depth 3`
	b, err := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script).Output()
	if err != nil {
		return nil, fmt.Errorf("进程读取失败: %w", err)
	}
	var out []Process
	if err := json.Unmarshal(b, &out); err != nil {
		var p Process
		if e := json.Unmarshal(b, &p); e != nil {
			return nil, fmt.Errorf("进程数据解析失败: %w", err)
		}
		out = []Process{p}
	}
	return out, nil
}

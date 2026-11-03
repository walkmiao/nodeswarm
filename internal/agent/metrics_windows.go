//go:build windows

package agent

import (
	"github.com/walkmiao/nodeswarm/internal/control"
	"runtime"
	"syscall"
	"unsafe"
)

type filetime struct {
	Lo uint32
	Hi uint32
}

func (f filetime) n() uint64 { return uint64(f.Hi)<<32 | uint64(f.Lo) }

type memoryStatus struct {
	Length               uint32
	MemoryLoad           uint32
	TotalPhys            uint64
	AvailPhys            uint64
	TotalPage            uint64
	AvailPage            uint64
	TotalVirtual         uint64
	AvailVirtual         uint64
	AvailExtendedVirtual uint64
}

var kernel32 = syscall.NewLazyDLL("kernel32.dll")
var getSystemTimes = kernel32.NewProc("GetSystemTimes")
var globalMemoryStatusEx = kernel32.NewProc("GlobalMemoryStatusEx")

type cpuSampler struct {
	prevIdle, prevKernel, prevUser uint64
	ready                          bool
}

func (s *cpuSampler) readMetrics() control.Metrics {
	m := control.Metrics{Cores: runtime.NumCPU()}
	var idle, kernel, user filetime
	r, _, _ := getSystemTimes.Call(uintptr(unsafe.Pointer(&idle)), uintptr(unsafe.Pointer(&kernel)), uintptr(unsafe.Pointer(&user)))
	if r != 0 {
		ti, tk, tu := idle.n(), kernel.n(), user.n()
		if s.ready && tk+tu > s.prevKernel+s.prevUser {
			total := (tk - s.prevKernel) + (tu - s.prevUser)
			busy := total - (ti - s.prevIdle)
			m.CPU = float64(busy) * 100 / float64(total)
		}
		s.prevIdle = ti
		s.prevKernel = tk
		s.prevUser = tu
		s.ready = true
	}
	mem := memoryStatus{Length: uint32(unsafe.Sizeof(memoryStatus{}))}
	r, _, _ = globalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&mem)))
	if r != 0 {
		m.MemoryTotal = mem.TotalPhys
		m.MemoryUsed = mem.TotalPhys - mem.AvailPhys
	}
	return m
}

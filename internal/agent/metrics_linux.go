//go:build linux

package agent

import (
	"bufio"
	"github.com/walkmiao/nodeswarm/internal/control"
	"os"
	"runtime"
	"strconv"
	"strings"
)

type cpuSampler struct {
	idle, total uint64
	ready       bool
}

func (s *cpuSampler) readMetrics() control.Metrics {
	m := control.Metrics{Cores: runtime.NumCPU()}
	if b, e := os.ReadFile("/proc/stat"); e == nil {
		fields := strings.Fields(strings.SplitN(string(b), "\n", 2)[0])
		var total, idle uint64
		for i, v := range fields[1:] {
			n, _ := strconv.ParseUint(v, 10, 64)
			total += n
			if i == 3 || i == 4 {
				idle += n
			}
		}
		if s.ready && total > s.total {
			m.CPU = float64((total-s.total)-(idle-s.idle)) / float64(total-s.total) * 100
		}
		s.total = total
		s.idle = idle
		s.ready = true
	}
	f, e := os.Open("/proc/meminfo")
	if e == nil {
		defer f.Close()
		vars := map[string]uint64{}
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			part := strings.Fields(sc.Text())
			if len(part) > 1 {
				value, _ := strconv.ParseUint(part[1], 10, 64)
				vars[strings.TrimSuffix(part[0], ":")] = value * 1024
			}
		}
		m.MemoryTotal = vars["MemTotal"]
		m.MemoryUsed = m.MemoryTotal - vars["MemAvailable"]
	}
	return m
}

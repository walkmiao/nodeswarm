//go:build !linux && !windows

package agent

import (
	"nodeconsole.local/node-console/internal/control"
	"runtime"
)

type cpuSampler struct{}

func (s *cpuSampler) readMetrics() control.Metrics { return control.Metrics{Cores: runtime.NumCPU()} }

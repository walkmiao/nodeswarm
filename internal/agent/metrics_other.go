//go:build !linux && !windows

package agent

import (
	"github.com/walkmiao/nodeswarm/internal/control"
	"runtime"
)

type cpuSampler struct{}

func (s *cpuSampler) readMetrics() control.Metrics { return control.Metrics{Cores: runtime.NumCPU()} }

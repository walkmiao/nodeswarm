//go:build linux

package manager

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func ListProcesses() ([]Process, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}
	out := make([]Process, 0, 128)
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid <= 0 {
			continue
		}
		base := filepath.Join("/proc", e.Name())
		comm, err := os.ReadFile(filepath.Join(base, "comm"))
		if err != nil {
			continue
		}
		stat, err := os.ReadFile(filepath.Join(base, "stat"))
		if err != nil {
			continue
		}
		line := string(stat)
		right := strings.LastIndex(line, ")")
		if right < 0 {
			continue
		}
		fields := strings.Fields(line[right+1:])
		if len(fields) < 20 {
			continue
		}
		start := fields[19] // field 22 (starttime), after removing pid/comm
		path, _ := os.Readlink(filepath.Join(base, "exe"))
		out = append(out, Process{PID: pid, Name: strings.TrimSpace(string(comm)), Path: path, StartTime: fmt.Sprintf("proc:%s", start)})
	}
	return out, nil
}

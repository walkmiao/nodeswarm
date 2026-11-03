//go:build darwin

package manager

import (
	"bufio"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

func ListProcesses() ([]Process, error) {
	data, err := exec.Command("ps", "-axo", "pid=,lstart=,comm=").Output()
	if err != nil {
		return nil, err
	}
	var out []Process
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		f := strings.Fields(scanner.Text())
		if len(f) < 7 {
			continue
		}
		pid, err := strconv.Atoi(f[0])
		if err != nil {
			continue
		}
		start := strings.Join(f[1:6], " ")
		path := strings.Join(f[6:], " ")
		name := path
		if k := strings.LastIndex(path, "/"); k >= 0 {
			name = path[k+1:]
		}
		out = append(out, Process{PID: pid, Name: name, Path: path, StartTime: fmt.Sprintf("ps:%s", start)})
	}
	return out, scanner.Err()
}

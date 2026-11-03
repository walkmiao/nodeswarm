//go:build windows

package manager

import (
	"encoding/json"
	"fmt"
	"os/exec"
)

// Powershell is only used for best-effort process enumeration. No processes
// are launched, injected into, or terminated by this management prototype.
func ListProcesses() ([]Process, error) {
	script := `$ErrorActionPreference='SilentlyContinue'; @(Get-Process | ForEach-Object { $st=''; try { $st=$_.StartTime.ToUniversalTime().ToString('o') } catch {}; $p=''; try { $p=$_.Path } catch {}; [pscustomobject]@{pid=$_.Id; name=($_.ProcessName+'.exe'); path=$p; startTime=$st} }) | ConvertTo-Json -Compress -Depth 3`
	b, err := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script).Output()
	if err != nil {
		return nil, fmt.Errorf("进程读取失败: %w", err)
	}
	var out []Process
	if err := json.Unmarshal(b, &out); err != nil { // Powershell returns object for single item
		var p Process
		if e := json.Unmarshal(b, &p); e != nil {
			return nil, fmt.Errorf("进程数据解析失败: %w", err)
		}
		out = []Process{p}
	}
	return out, nil
}

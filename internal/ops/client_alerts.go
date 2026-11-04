package ops

import (
	"fmt"
	"strings"
	"time"

	"github.com/walkmiao/nodeswarm/internal/control"
)

// evaluateClientAlerts turns execution failures into actionable fleet alarms.
// Planned-only Observe routes and unverified experimental egress are NOT
// counted as successful tunnels, but are not execution errors either.
func evaluateClientAlerts(a control.AgentView, now time.Time) []Alert {
	out := make([]Alert, 0, 3)
	base := "agent:" + a.ID + ":"
	if a.Revision != a.Client.Revision && a.DesiredAt != "" {
		if changed, err := time.Parse(time.RFC3339, a.DesiredAt); err == nil && now.Sub(changed) > 30*time.Second && !changed.After(now) {
			out = append(out, Alert{ID: base + "client-policy-drift", Severity: "warning", Kind: "client",
				Title: "Windows 客户端绑定策略尚未确认", Detail: fmt.Sprintf("期望修订 %d，客户端修订 %d；检查客户端连接与进程扫描", a.Revision, a.Client.Revision),
				TargetID: a.ID, TargetName: a.Name})
		}
	}
	if len(a.ClientBindings) == 0 {
		return out
	}
	if a.Client.ScanError != "" {
		out = append(out, Alert{ID: base + "client-scan", Severity: "critical", Kind: "client",
			Title: "Windows 客户端进程扫描失败", Detail: "PID 身份无法验证，客户端应停止托管代理：" + a.Client.ScanError,
			TargetID: a.ID, TargetName: a.Name})
		// The same scan failure is also reflected in every route. Avoid
		// duplicating it as an engine error for each bound PID.
		return out
	}
	if a.Client.Backend != "pidtcp" && a.Client.Backend != "proxybridge" {
		return out
	}
	engineFailures, invalidBindings := 0, 0
	engineDetail, bindingDetail := "", ""
	for _, route := range a.Client.Routes {
		switch route.Status {
		case "engine_error", "engine_exited", "scan_unavailable":
			engineFailures++
			if engineDetail == "" {
				engineDetail = route.Detail
			}
		case "stale_pid", "cannot_enforce", "pid_limit_exceeded":
			invalidBindings++
			if bindingDetail == "" {
				bindingDetail = route.Detail
			}
		}
	}
	if engineFailures > 0 {
		if engineDetail == "" {
			engineDetail = a.Client.Error
		}
		out = append(out, Alert{ID: base + "client-engine", Severity: "critical", Kind: "client",
			Title: "Windows PID 代理引擎未正常执行", Detail: fmt.Sprintf("%d / %d 个绑定发生执行失败；%s", engineFailures, len(a.ClientBindings), strings.TrimSpace(engineDetail)),
			TargetID: a.ID, TargetName: a.Name})
	}
	if invalidBindings > 0 {
		out = append(out, Alert{ID: base + "client-bindings", Severity: "warning", Kind: "client",
			Title: "Windows 客户端存在不可执行的 PID 绑定", Detail: fmt.Sprintf("%d / %d 个绑定需重新校验；%s", invalidBindings, len(a.ClientBindings), strings.TrimSpace(bindingDetail)),
			TargetID: a.ID, TargetName: a.Name})
	}
	return out
}

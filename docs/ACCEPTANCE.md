# NodeSwarm v1.2 Acceptance (2026-11-03)

- [x] Unit tests and race detector: client binding identity, capacity concurrency, role persistence, name-collision rejection, and zero false enforcement claims.
- [x] `go vet ./...`, frontend JavaScript parse.
- [x] Actual Linux Controller + Cloud Agent + local Client process smoke: enroll, heartbeat, process discovery, node binding, policy report, SOCKS5 auth and UDP negotiation, remote stop/start/restart.
- [x] Playwright controller-backed client fleet UI: login, real API list, client drawer, node binding save and persistence check.
- [ ] Windows 11 Client scheduled-task execution and UAC/ProxyBridge driver installation.
- [ ] Same-name PID-specific transparent proxy redirection / independently verified egress IP.
- [ ] Cloud VPS firewalls, public UDP relay, Docker runtime and long-running production scale.
- [ ] Controller HA / RBAC / automated client upgrade.

---

# NodeSwarm v1.1 Acceptance Summary (2026-11-03)

- [x] Go unit tests, race detector, and `go vet`.
- [x] One-time Agent enrollment token issue, expiry, one-use race, revoke, persistence, and replay prevention.
- [x] SOCKS5 TCP with username/password, bounded UDP port allocation/reuse and public bind address encoding.
- [x] Controller + Agent end-to-end: enroll, heartbeat, configure, start, stop, restart, UDP ASSOCIATE.
- [x] Node inventory batch health checks including authenticated SOCKS5 credentials.
- [x] Alerts (offline, runtime, configuration drift, CPU/memory, expiry), persisted mute/unmute.
- [x] Web browser interaction: node create/edit/delete, enrollment issue/revoke, alerts, UDP settings.
- [x] Cross-compile Linux amd64, Linux arm64, Windows amd64.
- [ ] Public VPS TCP/UDP end-to-end validation with NAT/firewall; real Windows host installation and uptime.
- [ ] Windows same-process-name, different-PID transparent routing; current export is application-name-only.
- [ ] Multi-user RBAC/HA and automated Agent updates.

## Prior v1.0 baseline acceptance

# NodeSwarm v1.0 验收记录（2026-11-02）

## 已通过：源码和本机联调

- [x] `go test ./... -count=1`：原节点池容量、导入、加密与进程规划；Controller 注册、令牌与配置 API；SOCKS5 握手/配置校验；云节点批量操作/状态处理。
- [x] `go vet ./...`、`node --check web/app.js` 通过。
- [x] `CGO_ENABLED=0` 编译 Linux amd64 / Linux arm64 / Windows amd64。
- [x] Controller + Agent 独立进程的 HTTP 联调：首次注册、管理员与 Agent 独立鉴权、心跳、云端节点自动同步。
- [x] 主控修改远端配置后 Agent 应用修订版；实际 SOCKS5 监听启动，正确/错误密码均得到符合预期的握手结果。
- [x] SOCKS5 CONNECT 拒绝 127.0.0.1 等私有目标；UDP ASSOCIATE 协商返回成功。
- [x] 主控远程停止、启动、重启 Agent 上 SOCKS5 服务；检查真实端口关闭/恢复和 revision 更新。
- [x] 导出 ProxyBridge 应用名规则格式，非 PID 级别。
- [x] Playwright 对新版 Web 页面（真实 Controller API 桥接）验证登录、总览、Agent 表格、详情抽屉、代理节点池、新增节点、部署说明和事件日志；已产出截图。

## 尚未验证 / 当前不支持

- [ ] 在真实 Windows 11 设备上安装 Agent 计划任务、检查防火墙/UAC 和长时间稳定性；Windows 程序目前只进行了交叉编译。
- [ ] 在公有云 VPS 上部署 Caddy TLS 与跨公网 Agent/客户端连接；当前 Controller/Agent E2E 在同机回环环境通过。
- [ ] 真实公网 TCP SOCKS5 CONNECT 的最终目的 IP、应用侧可观察的出口 IP。
- [ ] UDP ASSOCIATE 后的跨公网 UDP 中继与 NAT/防火墙连通；当前仅验证握手和代码路径。
- [ ] 100+ Agent 的大规模长时间心跳、日志、数据库与资源压力测试。
- [ ] Windows 同名 `Client.exe` 不同 PID 的独立透明代理引擎、DNS/IPv6 完整防泄漏、Windows 驱动兼容与目标游戏连接验收。
- [ ] 商用多用户 RBAC/审计、MFA、集中告警、自动更新/回滚、控制器 HA。

## 发布结论

**可作为“自有 Linux/Windows 云服务器 SOCKS5 服务管理器 v1.0”进行试部署和实际 TCP 场景试用**，但公有云公网联调和 Windows 实机仍需用户环境验证；**不能宣称“热血江湖多开已实现”或“不同 PID 已自动分流”**。

## v1.3.0 acceptance: workstation PID quota (2026-11-03)

- [x] Registry quota 1–128, default 8, independent per workstation; persisted across restart with compatibility for existing v1.2 data.
- [x] Atomic binding counts, node capacity coexistence, stale policy revision rejection and safe shrinking.
- [x] Client-side optional hard cap and `pid_limit_exceeded` status, not falsely reported as transparent proxy enforcement.
- [x] Controller + real process Client E2E: set limit to 1; authenticated policy heartbeat and `ClientSnapshot` report effective limit.
- [x] Browser automated test: modify per-machine quota, block over-limit selection, save one binding, verify persisted API.
- [ ] Independent per-PID Windows network enforcement, actual separate egress verification and Windows in-situ driver acceptance.

## v1.3.1 Windows process discovery

- [x] Native Toolhelp32/Kernel32 process inventory implemented, with PowerShell fallback.
- [x] CGO-free Windows amd64 cross-build succeeds.
- [ ] Windows 11 real-host CPU/memory and PID-reuse regression; no Windows runtime available in this environment.

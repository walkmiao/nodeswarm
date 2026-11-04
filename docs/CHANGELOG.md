## Post-v1.8 maintenance — 2026-11-04

- Cloud Agent SOCKS5 configuration changes prepare the new listener before retiring an old port. On same-port replacement failure the previous configuration is restored; applied revision advances only when the new listener starts successfully.
- Cloud Agent immediately acknowledges rejected desired revisions and reports the live revision/error to Controller; a rollback-specific alert distinguishes a healthy old listener from a failed requested update.
- Windows Desktop avoids recreating unchanged process/node/log DOM on four-second polls. A Chromium WebView fixture tests retention of focused node selection and correct binding dispatch.
- These changes do not provide verified UDP/DNS isolation, a Windows PID kill switch, or a tested public egress path.

## v1.8.0 - 2026-11-04: Unified NodeLink UI

- Apply the supplied DESIGN.md/tokens.css/styleguide.html contract as a shared dark-only design system.
- Rebuild Controller frontend with a 240px textual navigation shell, compact operation tables, fleet/resource summaries, modal confirmations, and a unified Agent detail panel.
- Rebuild the native Wails Windows Desktop frontend with a 76px icon rail, 44px header, process-rule table, quotas, node status, logs, and settings; preserve the Go API bindings.
- Keep cloud agents headless, with their fleet, health, and management rendered in Controller.
- Add design system synchronization, static UI audit and browser snapshot checks to development and CI.
- No proxy protocol changes. PID TCP is still experimental; UDP/DNS leakage isolation and Windows real-host egress remain unverified.

## v1.7.0 — opt-in experimental per-PID TCP SOCKS5 redirection

- Add Windows x64-only `internal/pidtcp` engine: PID + process creation time validation, separate same-name PID -> SOCKS5 routes, bounded initial SYN attribution, TCP NAT relay, IPv4/IPv6 packet parsing, SOCKS5 CONNECT with credentials, and no fallback after captured proxy CONNECT failure.
- Preserve Controller quota and local hard PID cap; start/stop with Client Agent lifecycle and show explicit `pid_tcp_unverified` warning in Wails UI.
- Keep `observe` as the default mode; require user opt-in, administrator rights, and separately installed official WinDivert 2.2 driver files.
- Add deterministic policy, TCP packet/NAT, SOCKS5 handshake and router regression tests; Windows CI compilation is not Windows driver/real-game acceptance.
- **Known limitations:** captured TCP only, no UDP or DNS proxy, no OS kill switch, unknown-owner and driver-failure paths may connect directly. Never claim verified egress or fail-closed network isolation.

## v1.5.0 — Windows native desktop client (Wails)

- Added a standalone Windows desktop App (`desktop/`) with native Wails/WebView2 window. The workstation user no longer needs a browser or CLI as the daily UI.
- Embedded the existing Go Client Agent into the desktop process, including process enumeration, registration/heartbeat, policy state and local PID hard limit.
- Added device-scoped, credential-authenticated self-service context and binding APIs. Client devices cannot access other devices or administrator endpoints; revision CAS, PID freshness and shared capacity remain enforced.
- Added local settings, one-time enrollment handling, node/route views, quota auto-fill, and a clean GitHub-style desktop UI. No workstation-local HTTP API is exposed.
- Added a Windows native desktop build workflow, per-user data storage, and install/uninstall scripts.
- Backend tests and browser UI preview pass on Linux. Native Windows Wails/WebView2 build and in-person execution must be validated on Windows CI/real devices; same-name PID-level proxy isolation is still not implemented.

## v1.4.0 — per-workstation automatic PID slot filling

- Added CAS-protected `POST /api/clients/{id}/auto-assign` with central quota, fresh PID/start-time inventory, node capacity and occupancy checks.
- Preserves incumbent PID to node bindings; selects nodes with the lowest utilization, avoids failed probes.
- Added the Web drawer action **自动填充空位**, manual edit conflict detection, and live acceptance coverage.
- Added concurrent capacity, stale inventory, persistence and authenticated API tests.
- When Windows process inventory fails, invalidate reported routes and stop the managed ProxyBridge CLI rather than continue using stale process identities.
- Still does not claim PID-level transparent network interception.

# NodeSwarm Change Log

## v1.3.1 - 2026-11-03

- Replace polling by repeatedly starting PowerShell with native Windows Toolhelp32 and Kernel32 process discovery (PID, creation-time fingerprint, full executable path).
- Keep PowerShell inventory as an explicit fallback if native enumeration fails; no third-party Windows runtime dependency.
- Windows amd64 build passes; no native Windows execution tests available in this container.

## v1.3.0 - 2026-11-03

- Add per-Windows-client configurable PID quota (default 8, adjustable 1–128) with atomic Controller validation, persisted settings, versioned updates, and safe shrink behavior.
- Add optional client-side `NODE_MANAGER_CLIENT_PID_HARD_LIMIT`, enforcing the lower effective quota before any managed routing process is started. Exceeding quota stops the managed ProxyBridge CLI and reports `pid_limit_exceeded` (does not block unrelated/direct application traffic).
- Add quota usage UI and over-limit selection prevention in the Windows Client drawer, including revision conflict handling and real Controller API browser checks.
- Extend authenticated Controller/Agent E2E tests to cover quota revision, policy delivery and acknowledged effective limit.
- Improve Web toast interaction so messages never obstruct drawer buttons.

### Remaining release blockers

- Native **per-PID** Windows packet redirection, verified separate public IP exits for same executable, leak-free DNS/UDP/IPv6 handling, and live Windows/game compatibility are **not implemented/verified**. Do not label v1.3 a completed game multi-instance proxy product.
- Controller HA, RBAC/MFA, signed unattended upgrades and public-cloud reliability validation remain pending.

## v1.2.0 - 2026-11-03

### Features
- Add independent Windows Client Agent role and authenticated process inventory heartbeats.
- Add per-PID/start-time binding policy APIs and capacity/identity checks across workstations.
- Deliver proxy endpoint credentials only to the authenticated target Client; redact administrative views.
- Add explicit `observe` client backend and optional ProxyBridge CLI app-name backend with rejection of conflicting same-name PID routes.
- Add Windows Client fleet page, process binding drawer, deployment scripts, and end-to-end UI/integration tests.
- Harden client credential file writes and role separation; prompt UDP-peer sockets to close during server restart.

### Limitations
- ProxyBridge app-name backend does **not** support independent SOCKS5 endpoints for same-name PIDs and is not Windows-host verified here.
- Real public VPS, UDP relay egress, and game-specific compatibility checks remain to be performed on target machines.

## v1.1.0 - 2026-11-03

### Features
- Single-use, expiring Agent enrollment grants with on-disk token hashes.
- Central health alerts for offline Agents, runtime errors, configuration drift, resource pressure, and node expiry; persistent mute and unmute.
- Controller-side bounded SOCKS5 health checks, both periodic and on-demand batch probes.
- Configurable per-Agent UDP relay port range with explicit advertised bind address.
- Administrator audit events for enrollment, proxy configuration, remote actions, and alert silences.
- Cross-platform GitHub Actions pipeline and Linux container deployment examples.
- Readability and reduced-motion improvements to the management UI.

### Fixes and validations
- Use stored encrypted credentials when probing authenticated SOCKS5 nodes, without exposing passwords in API snapshots.
- Cancel bounded monitoring scans without acquiring a leaked worker slot.
- SOCKS5 listener closes correctly during open UDP association; covered by a regression test.
- End-to-end Controller / Agent / SOCKS5 start-stop-restart test suite, web interaction test, and Go race detector pass.

### Important limitations
- Linux and Windows binaries were cross-built; Windows native execution and real cloud deployment still require host validation.
- Docker Compose examples were not executed in this environment.
- Windows transparent per-PID traffic redirection is not implemented; app-name ProxyBridge profile export is not equivalent.

## v1.0.0 - 2026-11-02

- Initial self-hosted Controller, outbound Agent registration and heartbeats, SOCKS5 TCP/UDP listener, proxy inventory, monitoring, and admin dashboard.

## v1.6.0 — Windows desktop security and connection safety

- Windows per-device bearer tokens now use per-user Windows DPAPI at rest (`tokenDpapi`) with automatic one-time migration from legacy plaintext `agent.json`.
- Each interactive logon session permits only one running Desktop App instance, preventing two agents from concurrently applying conflicting proxy profiles.
- A workstation heartbeat/authentication failure immediately stops its managed ProxyBridge child. The UI explicitly states that this **does not block direct OS network access**.
- PID table matches **PID + process creation time** and shows errors (stale PID, unreachable controller, scan error, engine exit) instead of silently displaying "planned".
- Desktop selection is preserved during 4-second refreshes. Windows identity, mutex and failsafe tests were added.
- **Not yet completed:** true transparent per-PID Windows traffic redirection, DNS/UDP leakage isolation and game compatibility tests. The application must not claim that a plan or process-name rule proves game egress.

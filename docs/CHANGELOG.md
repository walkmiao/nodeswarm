# NodeSwarm Change Log

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

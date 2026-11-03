# NodeSwarm Desktop — Windows Native App

NodeSwarm now ships **three distinct programs**:

- Controller: headless Go service + Web administration interface; deploy on a server.
- Cloud Agent: headless service that operates SOCKS5 nodes on Linux/Windows.
- **Desktop App**: a Windows GUI (`NodeSwarm-Desktop.exe`) running in the logged-in user's interactive session, not a browser or local B/S server.

Desktop technology: Wails v2.17.1 + Go + embedded HTML/CSS/JS, using Windows WebView2. Its native bridge binds Go methods directly. Wails does **not** bind an HTTP port on the PC for UI/API management.

## Features implemented

1. Desktop onboarding, Controller HTTPS URL, machine name, process filter (`Client.exe` by default), and one-time registration token.
2. Starts/stops the existing Go Client Agent within the desktop process. Heartbeats, registration, process scans and managed ProxyBridge lifecycle are reused, not replaced with a simulation.
3. Shows detected PIDs with executable name and path, live reported routes, Controller-provided proxy nodes, PID quota and local hard limit.
4. Self-service assign/unassign/auto-fill with revision-aware requests to new device-scoped Controller APIs. Device token cannot modify another client's bindings or any admin settings.
5. Settings and registered device identity persist per Windows user under `%APPDATA%\NodeSwarmDesktop`. The one-time enrollment code is *never* written into settings.json, nor is it shown again.
6. Logout/close stops the managed agent and its ProxyBridge child process. Auto-connect on next launch only occurs for a previously registered device.
7. Controller requires HTTPS except for localhost loopback. No admin token is distributed to workstations.

## Build (on Windows)

Install Go 1.23+, then:

```powershell
cd desktop
go mod tidy
go install github.com/wailsapp/wails/v2/cmd/wails@v2.17.1
wails build -clean -webview2 embed
```

The output is `desktop/build/bin/NodeSwarm-Desktop.exe`. A dedicated workflow `.github/workflows/desktop.yml` builds and uploads that binary on Windows GitHub Actions. The Wails dependency is deliberately isolated in `desktop/go.mod` to keep Controller and Cloud Agent small.

To install an existing binary for all Windows users from an elevated PowerShell terminal:

```powershell
powershell -ExecutionPolicy Bypass -File deploy/windows/install-desktop.ps1
```

This copies the native EXE to Program Files and creates Start Menu / Desktop shortcuts. Run `deploy/windows/uninstall-desktop.ps1` to remove the app. Uninstall preserves per-user configuration and device identity unless users explicitly remove them.

## User setup

1. Administrator issues a temporary registration token in Controller's **Agent enrollment** page.
2. On Windows, launch **NodeSwarm Desktop**, open **连接设置**, paste HTTPS Controller URL, workstation name, and enrollment token.
3. Click **保存并连接**; the app registers and obtains its own device credential. Future launches can connect automatically without entering the enrollment token.
4. Launch the game or another configured process. The **游戏进程** view lists matching PIDs with their start identity.
5. Select an allowed SOCKS5 node and press **分配**; the server checks capacity, machine PID quota and optimistic revision before persisting.
6. Verify status in the app. **Observe** means monitoring and planning only. **ProxyBridge** is an optional, explicit *process-name* proxy engine; it cannot safely give the same EXE's different PIDs different exits. It refuses ambiguous mappings rather than falsely claiming enforcement.

## Scope and verification

- Go Controller/agent/device API and desktop core are tested on Linux CI; UI markup is separately tested in headless Chromium (preview/interaction checks).
- Native Windows Wails build is validated through the GitHub Windows build workflow; this development environment cannot launch a Windows GUI or test the actual Windows WebView2 runtime locally.
- Actual game compatibility, WebView2 on offline Windows machines, and true same-name PID transparent network isolation remain outstanding acceptance items.
- The historic `node-console -mode client` CLI still works as a headless compatibility option, but **do not install it as the user-facing application**. Use the native Desktop App.

## v1.6 safety changes

- On Windows, the desktop device token is encrypted with per-user Windows DPAPI. Previous plaintext `agent.json` files are migrated automatically. A copied profile cannot be decrypted under a different Windows user; in that case, re-enroll the workstation.
- Only one instance can run per Windows logon session (`Local\\NodeSwarm-Desktop` mutex).
- If the Controller becomes unreachable, the managed ProxyBridge child is immediately stopped and the UI shows `controller_unreachable`. **This is not firewall fail-closed behavior:** the game or any other application can still connect directly. A separate OS-enforced block policy is needed before promising leak prevention.
- The desktop PID list checks PID and creation time together and does not label failed/unknown engines as successful routing.

## v1.7 experimental PID TCP mode

The optional third mode `pidtcp` uses a Windows x64 WinDivert driver and a separate Go TCP relay to route two different PIDs of the same EXE through separate SOCKS5 nodes. Set `WinDivert driver directory` to a trusted path containing official `WinDivert.dll` and `WinDivert64.sys`, and start the desktop as Administrator. On other operating systems, startup reports unsupported; no backend is silently substituted.

**Do not confuse this with verified or leak-free routing:** UDP/DNS can go direct, TCP ownership lookup can miss the SYN, existing connections may bypass, and stopping the driver or Controller can allow traffic to go direct. The App reports `pid_tcp_unverified`, not success. See [PID_TCP_EXPERIMENT.md](PID_TCP_EXPERIMENT.md) for safety constraints and Windows acceptance steps.

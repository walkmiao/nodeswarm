# Windows Client Agent: deployment, policy, and verification (v1.2)

## What it does

- Runs separately from the cloud SOCKS5 server on a Windows workstation.
- Uses an outbound authenticated Controller heartbeat. First enrollment requires a one-use token; subsequent starts reuse the machine's local credential.
- Scans only the configured executable basenames (default `Client.exe`), reporting `{PID, startTime, name, path}` with 12-second collection caching.
- Receives bindings selected in the Web console, keyed by the *observed* PID and process start time. The Controller checks identity and per-node capacity before accepting a binding.
- Reports explicit route states. **Neither a stored binding nor a successful ProxyBridge CLI start proves which public IP a game uses.**

## Windows installation (administrator PowerShell)

1. Download an official NodeSwarm release and inspect its `SHA256SUMS`.
2. Run from the extracted release directory:

```powershell
.\deploy\windows\install-client.ps1
notepad "$env:ProgramData\NodeSwarmClient\client.env"
```

3. Set `NODE_MANAGER_CONTROLLER` to your HTTPS Controller URL and `NODE_MANAGER_ENROLL_KEY` to a one-use registration key from **Settings → Agent enrollment**. Change `NODE_MANAGER_AGENT_NAME` and `NODE_MANAGER_CLIENT_TARGETS` to suit the workstation. For multiple executable basenames, separate them with semicolons.
4. Keep `NODE_MANAGER_CLIENT_ENGINE=observe` initially. The client **does not intercept application traffic** in this mode.
5. Start and verify:

```powershell
Start-ScheduledTask -TaskName NodeSwarmClient
Get-ScheduledTaskInfo -TaskName NodeSwarmClient
# View Controller > Windows Clients; check actual process PID and startTime.
```

`client.env` and `data/agent.json` are restricted to SYSTEM and Administrators by the install script. Store the bootstrap token only until enrollment succeeds, then remove it from `client.env`. Reuse the existing `data/agent.json` for future restarts; do not clone it to another workstation.

## Optional ProxyBridge mode (process-NAME routing only)

To integrate [ProxyBridge](https://github.com/InterceptSuite/ProxyBridge), install the publisher's Windows CLI separately and set:

```dotenv
NODE_MANAGER_CLIENT_ENGINE=proxybridge
NODE_MANAGER_PROXYBRIDGE_CLI=C:\Program Files\ProxyBridge\ProxyBridge_CLI.exe
```

Restart the NodeSwarm Client task. An elevated interactive session may be required depending on the driver and ProxyBridge installation; a SYSTEM startup task **has not been validated with ProxyBridge**. The client creates an owner-only `.pbprofile` in its locked data directory and launches the CLI with that profile. Its status is `process_name_unverified`, not `pid_enforced`.

### Routing safety boundaries

ProxyBridge's documented `ProcessName` rules apply to every process with a matching executable basename. NodeSwarm refuses to start a by-name profile if:

- Two instances of the same executable name require *different* proxy nodes (`unsupported_pid_isolation`).
- Another running instance with the same executable name is not bound (`unsafe_name_collision`).
- The observed PID or start-time fingerprint changed before policy application (`stale_pid`).
- The selected SOCKS5 endpoint is disabled, expired, missing, or invalid.
- The CLI is unavailable or the machine is not Windows.

**Important:** A new same-name process started in the gap between inventory scans could still match an already active by-name rule. This mode is not suitable when every PID must have isolated egress. It also does not provide fail-closed network blocking; applications may connect directly if the engine fails. For verified strict per-PID isolation, a separately tested, Windows-specific transparent proxy engine is required.

The CLI profile contains the SOCKS5 password; never distribute it as part of a public issue, log, or repository. The proxy credentials are sent only to the authenticated Client using HTTPS. The server's administrative dashboard never returns these passwords.

## Recommended acceptance on an actual Windows 11 machine

1. Enroll a workstation, verify PID/start-time/path of two instances of a harmless test EXE.
2. Bind two instances to one known SOCKS5 node; verify the client reports `planned_only` in observe mode.
3. If choosing ProxyBridge mode, use two **different executable basenames** first, each mapped to a different test SOCKS5 node.
4. Independently verify outbound public IP at a controlled destination, TCP/UDP session behavior, and absence of DNS/IPv6 leaks before claiming transparent routing.
5. Verify new process start and exit, PID reuse, controller disconnection, credential revocation, agent restart and logout.
6. Only after isolated test applications pass, validate compatibility with legitimate application/game use. Do not alter anti-cheat or game protection components.

## API

| Method | Endpoint | Auth | Purpose |
|---|---|---|---|
| `POST` | `/api/agent/enroll` | One-use/bootstrap key | Include `"kind":"client"` |
| `POST` | `/api/agent/heartbeat` | Client's unique credential | Observed processes/routes and desired policy exchange |
| `GET` | `/api/clients` | Admin | Redacted client registry and last known inventory |
| `GET` | `/api/clients/{id}` | Admin | One client's redacted detail |
| `PUT` | `/api/clients/{id}/bindings` | Admin | Replace bound PID/start-time/node list; atomic capacity check |
| `DELETE` | `/api/clients/{id}` | Admin | Revoke workstation registration |

The Controller does not execute arbitrary commands on client workstations. Only an explicitly configured upstream ProxyBridge CLI can be spawned, with fixed command arguments and a generated profile.

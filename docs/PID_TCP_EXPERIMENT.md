# NodeSwarm v1.7 — Experimental PID TCP routing (Windows x64)

**Status: experimental, opt-in, TCP only, NOT leak-proof and NOT ready for game production.**

This implementation is the first executable path for different running instances of the **same EXE** to use different SOCKS5 nodes. It is **not** a substitute for a kernel-enforced fail-closed VPN / firewall policy. A visible route status of `pid_tcp_unverified` means the WinDivert TCP capture engine started; it does **not** establish that a packet was intercepted or that the public exit IP was independently verified.

## Architecture

```
Controller (HTTPS policy: PID + process creation time -> SOCKS node)
                   |
                   v
NodeSwarm Desktop (Wails GUI + Go Client Agent, interactive Windows user)
                   |
          explicit "pidtcp" mode
                   |
           WinDivert NETWORK TCP capture
                   |
   TCP 5-tuple -> Windows GetExtendedTcpTable owner PID
                   |
    verify PID creation time + exact executable name
                   |
     reflect owned TCP SYN through local NAT relay
                   |
          RFC1928/1929 SOCKS5 CONNECT to assigned node
                   |
           cloud SOCKS5 TCP egress
```

This requires no change to the game executable. To identify source processes, the Windows code inspects `GetExtendedTcpTable` and `GetProcessTimes`, not the NETWORK layer itself (which carries no PID). A per-flow NAT mapping freezes its SOCKS5 endpoint for the connection lifetime; new connections can follow a changed policy. Unknown or other-process traffic is left untouched.

## User setup and requirements

1. Use **Windows x64**, Wails Desktop, and a Controller/Cloud Agent running NodeSwarm v1.7.
2. Download official **WinDivert 2.2 x64** from <https://reqrypt.org/windivert.html>. Verify the publisher/hash and read its license before redistribution. Do not obtain driver binaries from random mirrors. NodeSwarm does **not** ship the DLL/SYS binary.
3. Put `WinDivert.dll` and `WinDivert64.sys` in a trusted local directory (e.g. `C:\Program Files\NodeSwarm\drivers`), readable by the Desktop executable. Set this location in the **WinDivert driver directory** field. If left blank, the executable directory is used.
4. Start NodeSwarm Desktop **as Administrator** for this experimental mode so `WinDivertOpen` can load the driver. Observe mode does not require elevation. Never disable UAC or driver signature enforcement to make this work.
5. Select `PID TCP — experimental` and explicitly assign two different running PID identities to two distinct SOCKS5 nodes. The local PID quota (default 8) remains enforced independently of the Controller.
6. Expect the UI status **`PID TCP experimental / exit unverified`**. This is a warning, not a green success indicator. No packet capture or public IP proof is currently attached to this status.

## Safety limitations / failure behavior

- **UDP and DNS are not proxied** by `pidtcp` and may use the machine's ordinary network exit. UDP/53 sent by the Windows DNS Client service may not be attributable to the game process.
- **No fail-closed network protection**: traffic can go direct when the driver has not started, after the driver exits, when process attribution fails/times out, during queue overflow, or for pre-existing TCP sessions. The application exits/reroutes only new TCP flows with a verified PID owner; it does not install persistent Windows Firewall / WFP rules.
- The SOCKS5 upstream TCP CONNECT handshake does **not** retry a failed connection directly, but that protection applies only to TCP flows actually captured and positively matched to a binding.
- Owner-attribution decisions use a bounded cache. If the cache exhausts, the engine reports a fatal error and stops rather than grow memory indefinitely. After the engine has stopped, Windows traffic may go direct.
- WinDivert's NETWORK layer does not itself provide a PID. `GetExtendedTcpTable` attribution is inherently timing-sensitive; the first SYN is held briefly (currently 80 ms), and a lookup miss is allowed through, not blocked.
- The local TCP NAT relay listens on ephemeral IPv4/IPv6 ports and rejects unmatched connections, but may be discoverable from the LAN. Use Windows Firewall and trusted-machine practices, and never expose the relay as a public proxy.
- SOCKS5 username/password protects access, not transit encryption. Prefer VPN/private networking between the workstation and cloud SOCKS5 endpoint.
- This code has **not** been run with a real signed WinDivert driver or a Windows GUI in the Linux authoring environment. The Windows CI job establishes compilation and unit-test compatibility, not host network correctness.

## Concrete acceptance plan on a separate Windows test machine

1. Launch two instances of a **controlled TCP test program** compiled from the same EXE, not a game. Record both PIDs and their creation timestamps.
2. Configure cloud SOCKS5 A and B with distinct verified public IP addresses. Bind PID-A to A and PID-B to B, start experimental mode as Administrator.
3. Connect both test processes to an independently hosted echo/IP-report endpoint. Verify on the receiving server that each connection's source IP matches the assigned proxy, never the Windows host. Log failures and data in both directions.
4. Verify PID reuse, TCP half-close, concurrent connections, process exits, node down/recovery, a driver stop, and a controller disconnect. Check TCP and UDP paths, IPv4/IPv6, DNS queries, and packet capture. **Any direct leak is a release blocker.**
5. Run WinDivert official driver compatibility checks for target Windows versions, security software, and non-admin launch/upgrade. Do not promote from experimental until the leak and lifetime cases are resolved.

## Implementation and tests

- Policy validation: `internal/pidtcp/policy.go` + `policy_test.go`.
- TCP packet decoder and IPv4/IPv6 NAT mapping: `packet.go`, `flows.go` + tests.
- Async bounded TCP owner attribution: `owner_windows.go`, `router.go` + tests.
- Dynamically loaded official WinDivert driver: `driver_windows.go`.
- SOCKS5 CONNECT and TCP half-close relay: `socks.go` + tests.
- Engine lifecycle: `engine.go`, `engine_windows.go`, `engine_other.go`.
- `internal/client/client.go` is the only owner of engine start/stop from the Agent.

Verify with `go test ./... -count=1`, `go test -race ./...`, `go vet ./...`, and `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./...`. The `desktop.yml` workflow builds Wails on a Windows Runner. None of these automated checks prove that a game client's packets are intercepted and isolated in the field.

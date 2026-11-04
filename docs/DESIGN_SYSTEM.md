# NodeSwarm UI Design System

The design contract is the user-provided **NodeLink** visual system. Source references are stored under `design-system/`:

- `DESIGN.md`: design authority, component/layout requirements, review checklist.
- `tokens.css`: single token layer shared across server and desktop.
- `components.css`: shared status pills, buttons, data tables, KPI cards, form controls, dialogs and focus states.
- `styleguide.html`: sample library for visual reference; its examples are not functional product screens.

## Product surfaces

| Surface | Runtime | Layout | Real data source |
| --- | --- | --- | --- |
| Controller | Embedded HTML/CSS/JS in Go HTTP server | 240px textual sidebar + 44px breadcrumb + 52px statusbar | Authenticated Controller REST API |
| Windows Desktop | Native Wails/WebView2 process (NOT a hosted webpage) | 76px icon rail + 44px titlebar + 52px statusbar | Wails Go bindings and local Client Agent |
| Cloud Agent | Headless Go daemon; no separate browser/Web server | Its fleet, details, health, provisioning and lifecycle are rendered by Controller | Agent-to-Controller outbound heartbeat |

A Cloud Agent **must not expose a new unauthenticated management website** merely to meet a visual-design request.

## Visual constraints

- Dark only. `--accent` is the sole decorative highlight; green, amber, rose and blue are reserved for status/traffic semantics.
- All component dimensions and colors come from `tokens.css`: 26/32/42px controls, 60px data rows, 40px table headers, 76px KPIs, and mono tabular numbers.
- Desktop and Controller have different prescribed shells; do not apply the 240px server sidebar to the native Windows client.
- Agent status always distinguishes connected, starting, stopped, error and unverified operation. A configured proxy is not the same as a verified egress.
- Native confirmation dialog for destructive actions; search/filter and forms retain their original API semantics.
- Keep icons inline as small SVGs, maintain focus-visible rings and never rely on color alone for state.

## Editing and verification

Edit `design-system/tokens.css` or `components.css` once, then run:

```sh
python3 scripts/sync_design_system.py
python3 scripts/audit_ui.py
node --check web/app.js
node --check desktop/frontend/dist/app.js
go test ./... -count=1
```

The sync command copies shared files into `web/design-system/` and `desktop/frontend/dist/design-system/` so both `go:embed` and Wails asset embedding work offline. CI runs the read-only audit (`sync_design_system.py --check`).

`styleguide.html` uses some example-only mocked values and historical NodeLink wording. It documents visual rules, not the current NodeSwarm product feature set.

The UI screenshots used during development render test/demo data for visual validation; they do **not** imply deployed cloud capacity or verified Windows TCP/UDP egress. Real backend and Windows native QA remain separate steps.

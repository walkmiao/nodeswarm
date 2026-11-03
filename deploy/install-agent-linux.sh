#!/bin/sh
set -eu
if [ "$(id -u)" -ne 0 ]; then echo 'Run as root (sudo).' >&2; exit 1; fi
ARCH="$(uname -m)"
case "$ARCH" in x86_64) BINARY=node-console-linux-amd64;; aarch64|arm64) BINARY=node-console-linux-arm64;; *) echo "Unsupported architecture: $ARCH" >&2;exit 1;; esac
DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
if [ ! -f "$DIR/$BINARY" ];then echo "Missing $DIR/$BINARY" >&2;exit 1;fi
if ! id nodeconsole >/dev/null 2>&1;then useradd --system --home-dir /var/lib/node-console-agent --shell /usr/sbin/nologin nodeconsole;fi
install -m 0755 "$DIR/$BINARY" /usr/local/bin/node-console
install -d -m 0750 /etc/node-console
install -d -m 0700 -o nodeconsole -g nodeconsole /var/lib/node-console-agent
install -m 0644 "$DIR/deploy/systemd/node-console-agent.service" /etc/systemd/system/node-console-agent.service
if [ ! -f /etc/node-console/agent.env ];then install -m 0600 "$DIR/deploy/systemd/agent.env.example" /etc/node-console/agent.env;fi
systemctl daemon-reload
echo 'Agent installed but not started.'
echo '1) Edit /etc/node-console/agent.env (controller HTTPS URL + enrollment key)'
echo '2) Run: systemctl enable --now node-console-agent'
echo '3) Run: journalctl -u node-console-agent -f'

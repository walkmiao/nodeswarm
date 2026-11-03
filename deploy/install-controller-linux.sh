#!/bin/sh
set -eu
if [ "$(id -u)" -ne 0 ]; then echo 'Run as root (sudo).' >&2;exit 1;fi
ARCH="$(uname -m)"
case "$ARCH" in x86_64) BINARY=node-console-linux-amd64;; aarch64|arm64) BINARY=node-console-linux-arm64;; *) echo "Unsupported arch: $ARCH" >&2;exit 1;; esac
DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
if [ ! -f "$DIR/$BINARY" ];then echo "Missing $DIR/$BINARY" >&2;exit 1;fi
if ! id nodeconsole >/dev/null 2>&1;then useradd --system --home-dir /var/lib/node-console-controller --shell /usr/sbin/nologin nodeconsole;fi
install -m 0755 "$DIR/$BINARY" /usr/local/bin/node-console
install -d -m 0700 -o nodeconsole -g nodeconsole /var/lib/node-console-controller
install -m 0644 "$DIR/deploy/systemd/node-console-controller.service" /etc/systemd/system/node-console-controller.service
systemctl daemon-reload
systemctl enable --now node-console-controller
echo 'Controller running at localhost:8088. Add the TLS reverse proxy.'
echo 'Admin token: /var/lib/node-console-controller/admin.token (read via sudo)'
echo 'Agent enrollment: /var/lib/node-console-controller/enroll.token (read via sudo)'

# Linux / Windows 云端 Agent 部署与运维手册

## 服务器准备

建议一台主控服务器加任意台自有云主机。主控需能提供 HTTPS 公网域名；Agent 可以只建立出站 HTTPS 连接，不要求开放管理端口。自有客户端使用 SOCKS5 时需要直连对应云服务器的代理端口（TCP 和可选 UDP 中继端口）。

- Controller 安装：`sudo sh deploy/install-controller-linux.sh`
- Controller 日志：`sudo journalctl -u node-console-controller -f`
- Controller 令牌：`sudo cat /var/lib/node-console-controller/admin.token` 与 `enroll.token`。
- Agent 安装：`sudo sh deploy/install-agent-linux.sh`
- Agent 配置：`sudo nano /etc/node-console/agent.env`
- Agent 启动/重启：`sudo systemctl enable --now node-console-agent`、`sudo systemctl restart node-console-agent`
- Agent 日志：`sudo journalctl -u node-console-agent -n 200 --no-pager`
- Agent 停止：`sudo systemctl stop node-console-agent`。停止 Agent 不会删除主控注册信息。

## 生产部署流程

1. 配置域名 DNS、HTTPS 证书、主控服务器 443/TCP 防火墙；不要对公网开放 8088/TCP。
2. 安装 Controller，用浏览器登录，确认 `/api/health` 是 `mode=controller`。
3. 在每台 Agent 系统配置 `NODE_MANAGER_CONTROLLER=https://...`、`NODE_MANAGER_ENROLL_KEY=...`、`NODE_MANAGER_AGENT_NAME=...`。
4. Agent 连接后先在界面确认在线和版本，填写**实际对外地址**，选择监听端口，生成或输入足够强的 SOCKS5 账号密码。
5. 在 Agent 页面启用代理。注意“已下发”只是目标状态；观察实际 `runtime.status=running` 且 `runtime.revision=revision`，再开放云安全组中的代理端口。
6. 将 `allowedCidrs` 设成实际客户端公网出口地址（示例：`198.51.100.4/32` 仅作格式说明，不可用作真实公网 IP）；生产建议配合云安全组限制。留空为不限制来源 IP，但仍强制 SOCKS5 账号认证，不推荐。
7. For UDP on VPS: prefer a fixed `udpPortStart` to `udpPortEnd` range (max 128 ports); allow the selected UDP range from trusted client IPs in both provider security group and host firewall. Without this configuration, the relay uses dynamically allocated UDP ports.
8. 从代理节点池点击“检测”验证 TCP SOCKS5；再用自己控制的实际应用测试 TCP/UDP 业务。检测成功不能替代公网出口真实性检查。

## 排障

| 症状 | 检查顺序 |
|---|---|
| Agent 不出现 | 查看 `systemctl status`、`journalctl` → DNS/TLS → `NODE_MANAGER_CONTROLLER` 和注册令牌 → 443 出站 |
| Agent 在线但服务未运行 | 检查配置是否启用、runtime.revision 是否赶上 revision、服务端口是否被占用、Agent 日志 |
| Controller 页面无法登录 | 使用 `admin.token` 而非 `enroll.token`；检查浏览器 URL/反代 HTTPS/令牌剪贴空格 |
| SOCKS5 从公网连接失败 | 确认运行中，监听为 `0.0.0.0` 或正确网卡 IP，云防火墙 TCP 入站，服务器安全组、Allowed CIDR、认证密码 |
| TCP 检测成功但应用不可用 | 认证、目标端口、目标 DNS、客户端是否真的配置代理，是否被上游应用规则限制；TCP 检测不验证 UDP |
| UDP ASSOCIATE succeeds but packets fail | Verify bounded relay UDP port pool and firewall/NAT mapping, publicHost BND address, client UDP source restrictions and target UDP protocol; TCP handshake alone is not UDP data-path validation |
| Agent 离线后代理自动消失 | 安全策略：控制端失联超过 10 分钟后自动停止；修复连接后，代理按目标配置恢复 |
| 主控重启后配置丢失 | 检查 `NODE_MANAGER_DATA` 是否改变以及存储文件所属用户；备份/恢复需要同时保留 `registry.key`、节点密钥及数据文件 |
| 改名/改 IP 后多次注册 | 绑定凭据位于 Agent 数据目录的 `agent.json`；不要复制到另一台主机；重新部署时保留本机数据目录 |

## 凭据与备份

- **必须备份** Controller 数据目录整套文件，尤其 `registry.key`、`agents.json`、管理员/注册令牌和节点存储密钥。密钥丢失可能导致保存的代理密码无法解密。
- Agent 的 `agent.json` 是独立机器访问令牌；谨慎备份并防止复制到不同机器。
- 代理凭据会在 `GET /api/agents/{id}/profile` 生成的 `.pbprofile` 中明文出现，下载后请保存在受限位置，不要上传聊天群或公开 Git 仓库。
- 管理界面的令牌存在浏览器会话存储中；使用专用管理员设备，在用完后点击“退出”。
- 建议在 Caddy 前增加私网/VPN 或身份网关，并在云端用安全组缩窄受信客户端来源。系统不是完整的商用多租户 RMM。

## 注销节点

先在主控下发停止，等待实时状态变成 `stopped` 并且版本已同步；确认服务器上的 SOCKS5 端口已经关闭，再撤销 Agent 注册。离线机器必须先通过主机系统管理手段停止 Agent，不能假定撤销操作能够瞬间关闭已失联进程。随后再卸载 systemd 服务/计划任务、清理主机凭据、关闭云安全组端口。

## 升级

升级前备份数据目录，替换 `/usr/local/bin/node-console`，重启服务并检查 Agent/Controller 版本和健康状态。Windows 更新则替换 `C:\ProgramData\NodeConsoleAgent\node-console.exe`（先停止计划任务）。当前无内置自动升级功能，也没有跨版本数据库迁移协议；请在测试环境先验证旧数据兼容。

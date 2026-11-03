# NodeSwarm v1.1 — 云端代理节点集中管理系统

面向自有云服务器的 **Controller + Cloud Agent + SOCKS5 节点池** 运维平台。
将分散在 Linux / Windows 主机上的 SOCKS5 服务，统一到一个轻量的 Web 控制台中管理。系统提供节点注册、心跳、主机指标、凭据保护、远程配置/启停、批量操作、节点监测与规则导出。Go 标准库构建，前端已嵌入二进制，无需 Node.js、数据库或额外 Web 静态资源。

> **产品边界（重要）**：v1.1 是可以真实运行的 **云端 SOCKS5 服务器与节点管理平台**。它**不是**可让同名 `Client.exe` 的不同 PID 走不同出口的 Windows 透明代理客户端，也不保证突破任何游戏的多开规则。界面中的“本机进程分配”明确属于 **规划**；导出的 ProxyBridge 配置只按 **进程名** 分流。不能把规则导出或规划关系理解为真实 PID 级代理已生效。

## Git 源码仓库与构建

本仓库仅追踪源码、测试、部署脚本及文档；`node-console-linux-*`、`node-console-windows-*.exe` 等二进制属于构建产物，已由 `.gitignore` 排除。下载发行包与从 Git 克隆源码不同：克隆后需要自行构建。

要求 Go 1.23.2 或更新版本。Linux/macOS 下：

```bash
go test ./... -count=1
go vet ./...
make all            # 交叉编译 Linux amd64/arm64 和 Windows amd64
```

Windows PowerShell 下可直接运行 `go test ./...`，然后执行 `go build -o node-console-windows-amd64.exe .`；需要自动执行完整检查时，也可运行 `scripts/build-windows.ps1`。构建完成后才运行 `deploy/` 中的安装脚本；`make zip` 会打包已构建的二进制供部署，不会将其加入 Git。

本地 `.env` / `*.env`、管理员与 Agent 令牌、运行数据、私钥、SQLite、日志和测试产物都不应提交。示例 `*.env.example` 会保留在版本库中。

## 功能与状态

| 能力 | v1.1 状态 | 说明 |
|---|---|---|
| Web 管理端 | **已实现** | 登录、总览、节点池、Agent 列表/详情、事件、进程规划、设置 |
| 多云主机 Agent | **已实现** | Linux amd64/arm64，Windows amd64；Agent 主动出站连接，主控集中查看 |
| 集中配置与批量启停 | **已实现** | 配置、启动、停止、重启指令经心跳同步；在线指令通常数秒生效 |
| 远端 SOCKS5 服务 | **已实现** | 用户名密码认证、TCP CONNECT、UDP ASSOCIATE/中继（可设固定 UDP 端口池）、并发限制、IP 白名单 |
| 指标与流量 | **已实现** | CPU、内存、连接数、连接累计、传输字节；历史采样与最近 Agent 日志 |
| 代理节点池 | **已实现** | Agent 托管节点自动同步，以及第三方 SOCKS5 导入、检测、容量/到期字段 |
| SOCKS5 TCP 探测 | **已实现** | 手动、批量与定时（3 分钟）检测；真实 SOCKS5 握手/认证/CONNECT，不代表 UDP 和游戏协议验收 |
| 一次性 Agent 注册 | **已实现** | 控制台生成 1～1440 分钟有效的单次注册令牌，支持撤销，不回显密钥 |
| 健康告警与静音 | **已实现** | 离线、代理异常、配置待确认、高 CPU/内存、节点即将到期；支持静音 1～168 小时 |
| 固定 UDP 中继端口池 | **已实现** | 可配置 `udpPortStart`～`udpPortEnd`（最多 128 个），可按云安全组固定放行 |
| ProxyBridge 规则导出 | **已实现** | 产生 `.pbprofile`，按应用名路由，文件含敏感代理凭据 |
| Windows 同名 PID 独立透明代理 | **未实现** | 需要 Windows 分流引擎及真实出口隔离验收 |
| 服务器任意命令/远程桌面 | **不提供** | 此版管控代理服务与指标，不运行任意系统命令 |
| 账号体系/RBAC/HA | **未实现** | 单管理员令牌、单主控部署；当前适用于个人/小型自建集群 |

## 界面预览

- 现代运维控制台：暗色固定侧边栏、浅色数据内容区、紧凑可筛选表格、抽屉式 Agent 配置。
- 云端 Agent：节点在线/离线、心跳、资源使用、实际运行状态、配置修订版本、远程操作日志。
- 节点池：托管和手工代理统一显示，可独立进行 TCP 检测。

## 项目结构

```text
.
├── main.go                     Controller HTTP/Web 与运行模式
├── api_agents.go               远程 Agent 管理 REST API
├── internal/
│   ├── control/                Agent 注册、认证、配置、历史与持久化
│   ├── agent/                  Linux/Windows 出站心跳与状态执行
│   ├── proxy/                  独立 SOCKS5 TCP/UDP 服务
│   └── manager/                节点池、探测、本机进程和规划分配
├── web/                        自包含 Web 前端（HTML/CSS/JS）
├── deploy/
│   ├── install-controller-linux.sh
│   ├── install-agent-linux.sh
│   ├── systemd/                Linux 服务配置
│   ├── caddy/                  HTTPS 反向代理样例
│   └── windows/                Windows Agent 开机任务
├── scripts/                    E2E/浏览器验收/多平台构建/打包
├── docs/                       技术审查、架构、云部署与验收报告
└── (运行 make all 后生成 node-console-{linux,windows}-* 二进制)
```

## 快速启动：先在本机试用控制台

Windows 11 (PowerShell)：

```powershell
.\node-console-windows-amd64.exe -mode controller
```

Linux amd64：

```bash
chmod +x ./node-console-linux-amd64
./node-console-linux-amd64 -mode controller
```

浏览器访问 **http://127.0.0.1:8088**。首次运行会在用户配置目录的 `NodeConsole` 中创建 `admin.token` 和 `enroll.token`：

- Windows：`$env:APPDATA\NodeConsole\admin.token`
- Linux：`~/.config/NodeConsole/admin.token`（以实际输出的 Data directory 为准）

把 `admin.token` 文件中的令牌输入登录界面。两个令牌用途不同：**管理员登录令牌**不能用于 Agent 注册；**Agent 注册令牌**不能用于管理操作。

自定义数据目录/端口：

```powershell
$env:NODE_MANAGER_DATA = 'C:\NodeConsoleData'
$env:NODE_MANAGER_PORT = '8188'
.\node-console-windows-amd64.exe
```

默认只监听本机回环地址。**不要为了远程访问直接将未加 TLS 的 8088 端口暴露到互联网。**

## v1.1 新增：云端令牌、告警和 UDP 端口池

在“系统设置 → Agent 一次性注册令牌”点击 **生成令牌**，设置名称和有效分钟数。令牌只会在创建时显示一次；把它填入该台 Agent 的 `NODE_MANAGER_ENROLL_KEY`。注册成功后令牌自动失效，其他 Agent 无法重放。现有 `enroll.token` 共享注册方式暂时保留兼容，建议只在受信任网络中管理。

“告警中心”自动汇总 Agent 离线、SOCKS5 异常、期望配置与实际版本不一致、CPU/内存异常、节点 TCP 检测失败与到期提醒。静音信息写入 `alert-mutes.json`；目前是**控制台内告警**，未接入邮件、Webhook、短信等外部通知。对于外部 SOCKS5 节点，Controller 启动后 20 秒开始首轮检测，之后每 3 分钟按最近检测时间分批扫描，最多每轮 64 个，限制 8 路并发。手动批量检测每次最多 32 个节点。

如云服务器要使用 UDP，在 Agent 详情填写 **UDP 起始端口** / **UDP 结束端口**（例如 `20000`～`20031`，最多 128 个端口）并勾选 UDP。服务器安全组放行所选 UDP 端口段，SOCKS5 客户端通过 UDP ASSOCIATE 获得被分配的端口。若配置了 `publicHost`，BND.ADDR 优先通告该公网 IP/域名，而不是云主机内网地址。**实际公网 UDP 可用性仍需在目标 VPS 上验收。**

## 云部署：Controller 与任意数量 Agent

拓扑：`浏览器 → HTTPS Caddy(443) → Controller(127.0.0.1:8088)`；每台 Agent **出站 HTTPS(443)** 主动注册/心跳，收到控制指令后在该主机的指定端口运行 SOCKS5。Agent 不开放管理 HTTP 端口；代理服务端口应另设严格入站防火墙规则。

### 1. Linux 主控服务器

在已解压的发行目录下：

```bash
sudo sh deploy/install-controller-linux.sh
sudo cat /var/lib/node-console-controller/admin.token
sudo cat /var/lib/node-console-controller/enroll.token
sudo systemctl status node-console-controller
```

安装脚本将二进制复制为 `/usr/local/bin/node-console`，创建 `nodeconsole` 系统用户并启动 systemd 服务。控制台仍仅在 `127.0.0.1:8088` 提供 HTTP。

创建域名解析指向主控服务器，参考 `deploy/caddy/Caddyfile.example` 配置 Caddy（需服务器可满足证书验证条件），由 Caddy 负责自动 TLS：

```text
console.example.com {
    encode zstd gzip
    reverse_proxy 127.0.0.1:8088
}
```

最终通过 `https://console.example.com` 登录。控制端防火墙仅需按部署要求开放 HTTPS 端口（通常 443）；8088 不允许公网访问。

### 2. 安装 Linux 云端 Agent

把 **发行包** 复制到每台云服务器，执行：

```bash
sudo sh deploy/install-agent-linux.sh
sudo nano /etc/node-console/agent.env
```

修改为你自己的主控域名和注册令牌：

```dotenv
NODE_MANAGER_CONTROLLER=https://console.example.com
NODE_MANAGER_ENROLL_KEY=从主控服务器enroll.token获取的令牌
NODE_MANAGER_AGENT_NAME=HK-Cloud-01
```

启用开机自启并查看状态：

```bash
sudo systemctl enable --now node-console-agent
sudo systemctl status node-console-agent
sudo journalctl -u node-console-agent -f
```

Agent 首次连接时向 Controller 注册一次；后续使用本机持久化的独立 Agent 令牌，每次约 7 秒上报心跳。数据文件保存在 `/var/lib/node-console-agent`，不应与另一台 Agent 共享。删除此目录会导致下次启动重新注册。

### 3. 安装 Windows 云端 Agent

以管理员身份打开 PowerShell，在发行包根目录执行：

```powershell
.\deploy\windows\install-agent.ps1
notepad "$env:ProgramData\NodeConsoleAgent\agent.env"
Start-ScheduledTask -TaskName 'NodeConsoleAgent'
Get-ScheduledTask -TaskName 'NodeConsoleAgent'
```

修改 `agent.env` 为实际主控 URL、注册令牌和节点名。Windows 采用 **SYSTEM 身份的开机计划任务**，并非原生 Windows Service；第一次启动和升级需在真实 Windows 环境做权限/防火墙验收。

### 4. 在 Web 控制台管理节点

1. 打开 **云端 Agent**，确认新机器出现并显示“在线”。
2. 进入节点详情，填写：**公网 IP/域名**、监听 IP（通常 `0.0.0.0`）、SOCKS5 TCP 端口（如 `1080`）、用户名/密码、最大连接、地区、允许访问客户端 CIDR。
3. 单击 **保存并下发配置**，然后 **启动代理**（也可保存时启用）。下一个心跳会实施配置，并返回实际监听/版本/错误信息。
4. 到 **代理节点池** 查看自动生成的托管节点，单击“检测”执行 SOCKS5 TCP 探测。
5. 需要为**某种应用名**创建客户端路由规则时，在 Agent 详情点“客户端规则”下载 `.pbprofile`，按 ProxyBridge 官方文档导入；**不是 PID 独立分流**。
6. 多选 Agent 可批量启动/停止。离线 Agent 的指令只会等待下次连接，不代表已经在远端立即执行。

### 云防火墙和 UDP 的重要说明

- 默认 TCP 服务端口示例为 **1080/TCP**。只允许你自己客户端的来源公网 IP / CIDR，并同时在云安全组限制入站。
- 如未指定 `udpPortStart/udpPortEnd`，UDP 仍使用系统动态分配端口；建议云环境配置固定 UDP 端口池并仅允许授权客户端源 IP。SOCKS5 TCP 端口与 UDP 中继端口是两组不同的监听。
- SOCKS5 用户名/密码本身**不提供传输加密**，代理客户端应使用自己的加密应用协议；代理端口不要开放给任意来源。
- 默认代理引擎拒绝访问回环、私有及部分保留目标地址，以降低 SSRF 和内网横向访问风险。
- Controller 丢联超过 10 分钟时 Agent 会关闭 SOCKS5（安全优先）；恢复连接后按目标状态重新应用配置。撤销注册对离线 Agent 不保证瞬间生效。

### Docker（Linux 云服务器可选）

项目根目录提供多架构 `Dockerfile`，`deploy/docker/` 提供 Controller 和 Agent 的 Host Network Compose 示例。Controller 通过本机 HTTPS 反向代理对外提供管理入口；Agent 使用 `network_mode: host` 直接监听主机 TCP/UDP 端口。

```bash
# 在 deploy/docker 下创建仅管理员可读的配置文件
cp deploy/docker/agent.env.example deploy/docker/agent.env
chmod 600 deploy/docker/agent.env
# 先将主控域名与一次性注册密钥填入此文件
# 在工程根目录运行
sudo docker compose -f deploy/docker/compose.agent.yml up -d --build
```

此部署方式不适用于依赖 Docker Desktop 的 Windows/macOS Host Network 环境；本版本的容器部署清单尚未经过真实 VPS 联调。**不要将 agent.env 提交到 Git。**

## 运维与故障排查

见 [`docs/OPERATIONS.md`](docs/OPERATIONS.md)（服务重启、失联、认证、代理失败、端口冲突、密钥备份、代理服务删除）。

## 自测与构建

Go 1.23.2+，业务源码仅使用标准库；GitHub Actions 负责 Linux/Windows CI、端到端 SOCKS5 联调和跨平台构建：

```bash
go test ./... -count=1
go vet ./...
go test -race ./...
node --check web/app.js
make all
```

集成烟测（Linux amd64，使用临时目录及随机本地端口启动真实 Controller 和 Agent）：

```bash
python3 scripts/e2e_smoke.py
```

浏览器验收需要 Python Playwright 和 Chromium，并且须先启动用于测试的 Controller，参见 `scripts/visual_check.py` 的环境变量。**Linux 端到端验收不等同于 Windows 实机验证。**

技术架构见 [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md)，测试清单见 [`docs/ACCEPTANCE.md`](docs/ACCEPTANCE.md)，Windows PID 级分流缺口见 [`docs/PROXY_ENGINE_REVIEW.md`](docs/PROXY_ENGINE_REVIEW.md)。

## 安全与生产限制

本系统适合自建、小型、单管理员 SOCKS5 基础设施管理。API 由静态管理员令牌保护，无细粒度 RBAC、账号找回、登录限速、数据库高可用及集中告警。生产部署应增加 Cloudflare Access / VPN / Caddy IP 白名单等外层保护，并定期备份密钥和数据文件。密钥和密文在相同主机的数据目录，不能抵御已取得主控系统账户权限的攻击者；请保护服务器和备份文件。不要把 `admin.token`、`enroll.token`、`agent.json`、`.pbprofile` 或代理明文凭据提交到公开仓库。

源码自研代理服务不打包任何第三方透明代理驱动。若未来集成 ProxyBridge / WinDivert，应逐一审查其许可证、驱动签名和实际部署风险。

版本变更见 [`docs/CHANGELOG.md`](docs/CHANGELOG.md)。

# Node Console v1.0 架构与控制边界

## 拓扑

```text
                         管理员浏览器
                                |
                         HTTPS 443 (Caddy)
                                |
                  Controller (127.0.0.1:8088)
                      Go HTTP API + embed Web
                 节点库存 + Agent 注册 + 目标状态
                              / | \\
                     outbound HTTPS heartbeats
                            /   |   \\
                   Cloud Agent  Cloud Agent  Cloud Agent
                    Linux x64    Linux arm64   Windows x64
                       |              |           |
               SOCKS5 TCP/UDP   SOCKS5 TCP/UDP  SOCKS5 TCP/UDP
                       \_____________  ____________/
                                     \/
                          自有客户端主动连接代理出口
```

**Controller 不转发代理业务流量**：业务流量直达各自云服务器 Agent 所监听的 SOCKS5 端口；控制流和数据流分离，避免主控成为流量瓶颈。

## 设计决策

1. **Agent 出站控制通道**：无需从公网主动向 Agent 管理端口建立连接。客户端注册后每约 7 秒 POST 心跳，以 TLS + Agent 独立令牌认证。
2. **目标状态 vs 实际状态**：管理员调整配置或启停只增加 `revision`；Agent 按 `revision` 应用，回报 `runtime.revision/status/error`，可从 API 对比识别是否生效。配置操作返回成功并不代表 Agent 已应用。
3. **断联保守策略**：网络或主控失联 10 分钟后 Agent 自动停止已开启的代理监听，恢复联通后重新按目标状态应用。对已撤销凭据的 Agent，下一次收到 HTTP 401 会退出并停止代理。
4. **中央存储**：控制器使用加密 JSON 文件，持久化 Agent 注册、令牌哈希、加密 SOCKS5 密码、节点元数据与历史采样。临时本机进程规划不持久化。所有文件按最小权限访问。
5. **独立账号认证**：Admin 令牌仅用于管理 API；Enrollment 令牌用于首次注册；Agent 独立令牌用于心跳。请避免通过反向代理公开日志记录 Authorization 请求头。
6. **单节点隔离**：每台 Agent 的配置独立，监听失败只影响该 Agent。管理端批量操作按 ID 分开记录结果，不提供执行任意 shell 命令的权限。
7. **代理安全策略**：SOCKS5 强制用户名/密码、支持 `AllowedCIDRs`，默认屏蔽连接私有/回环/保留目标地址；提供并发连接限制。SOCKS5 本身不提供传输加密。

## 核心数据模型

- `AgentView`：机器 ID、名称、版本、平台、最近心跳、CPU/内存历史、运行状态、最近日志、期望代理配置和 revision。
- `ProxyConfig`：监听 IP、端口、公网地址、账号、加密持久化密码、TCP/UDP 能力、允许来源 CIDR、容量和最大连接数。
- `ProxyRuntime`：真实启动状态、监听地址、连接数、累计字节、最后错误、已应用版本。
- `Node`：代理节点库存记录（手工添加或关联 Agent 自动同步）。
- `Assignment`：本机 PID + 进程创建标识到节点的**规划关系**，非系统级路由状态。

## API 摘要

| 方法 | URL | 鉴权 | 功能 |
|---|---|---|---|
| GET | `/api/health` | 无 | 存活检查，仅公开基础状态 |
| GET | `/api/auth/check` | Admin | 管理员凭据校验 |
| GET | `/api/dashboard` | Admin | 总览完整状态 |
| GET | `/api/agents` | Admin | 所有 Agent |
| GET | `/api/agents/{id}` | Admin | Agent 详情 |
| PUT | `/api/agents/{id}/config` | Admin | 修改目标代理配置 |
| POST | `/api/agents/{id}/action` | Admin | `start / stop / restart` |
| POST | `/api/agents/batch` | Admin | 对选定 Agent 批量发送指令 |
| DELETE | `/api/agents/{id}` | Admin | 注销凭据（只允许服务已停止时） |
| GET | `/api/agents/{id}/profile` | Admin | 导出应用名分流的 ProxyBridge 规则，**含敏感明文** |
| POST | `/api/agent/enroll` | Enrollment | Agent 首次注册 |
| POST | `/api/agent/heartbeat` | Agent Token | 上报指标/运行状态与拉取配置 |
| GET | `/api/state` | Admin | 代理节点和本机规划状态 |
| POST | `/api/nodes/{id}/probe` | Admin | 测试 TCP SOCKS5 CONNECT |

## 未实现且不应误解的能力

- 此 Agent 是**云端 SOCKS5 服务端**，不是 Windows 游戏进程的透明代理客户端。
- 导出 ProxyBridge `.pbprofile` 只有应用名规则，不能将同一 `Client.exe` 的不同 PID 分配到不同出口。
- UDP ASSOCIATE 中继采用动态端口，公网使用前需要协调云安全组、NAT；目前未完成跨公网 UDP 端到端验收。
- 目前没有 RBAC、二次认证、分布式协调/HA、节点自动更新或可执行代码分发能力。不要将其当作可任意远程操控主机的 RMM 平台。

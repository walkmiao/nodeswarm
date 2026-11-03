# Node Console v1.0 验收记录（2026-11-02）

## 已通过：源码和本机联调

- [x] `go test ./... -count=1`：原节点池容量、导入、加密与进程规划；Controller 注册、令牌与配置 API；SOCKS5 握手/配置校验；云节点批量操作/状态处理。
- [x] `go vet ./...`、`node --check web/app.js` 通过。
- [x] `CGO_ENABLED=0` 编译 Linux amd64 / Linux arm64 / Windows amd64。
- [x] Controller + Agent 独立进程的 HTTP 联调：首次注册、管理员与 Agent 独立鉴权、心跳、云端节点自动同步。
- [x] 主控修改远端配置后 Agent 应用修订版；实际 SOCKS5 监听启动，正确/错误密码均得到符合预期的握手结果。
- [x] SOCKS5 CONNECT 拒绝 127.0.0.1 等私有目标；UDP ASSOCIATE 协商返回成功。
- [x] 主控远程停止、启动、重启 Agent 上 SOCKS5 服务；检查真实端口关闭/恢复和 revision 更新。
- [x] 导出 ProxyBridge 应用名规则格式，非 PID 级别。
- [x] Playwright 对新版 Web 页面（真实 Controller API 桥接）验证登录、总览、Agent 表格、详情抽屉、代理节点池、新增节点、部署说明和事件日志；已产出截图。

## 尚未验证 / 当前不支持

- [ ] 在真实 Windows 11 设备上安装 Agent 计划任务、检查防火墙/UAC 和长时间稳定性；Windows 程序目前只进行了交叉编译。
- [ ] 在公有云 VPS 上部署 Caddy TLS 与跨公网 Agent/客户端连接；当前 Controller/Agent E2E 在同机回环环境通过。
- [ ] 真实公网 TCP SOCKS5 CONNECT 的最终目的 IP、应用侧可观察的出口 IP。
- [ ] UDP ASSOCIATE 后的跨公网 UDP 中继与 NAT/防火墙连通；当前仅验证握手和代码路径。
- [ ] 100+ Agent 的大规模长时间心跳、日志、数据库与资源压力测试。
- [ ] Windows 同名 `Client.exe` 不同 PID 的独立透明代理引擎、DNS/IPv6 完整防泄漏、Windows 驱动兼容与目标游戏连接验收。
- [ ] 商用多用户 RBAC/审计、MFA、集中告警、自动更新/回滚、控制器 HA。

## 发布结论

**可作为“自有 Linux/Windows 云服务器 SOCKS5 服务管理器 v1.0”进行试部署和实际 TCP 场景试用**，但公有云公网联调和 Windows 实机仍需用户环境验证；**不能宣称“热血江湖多开已实现”或“不同 PID 已自动分流”**。

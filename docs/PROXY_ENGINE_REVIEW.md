# ProxyBridge / Windows 进程级分流可行性审阅

调研日期：2026-11-02。此报告基于 GitHub 公开源码和官方 README，不是对热血江湖国服限制机制的逆向结论，也不保证可扩大游戏官方许可的同时运行数量。

## 竞品原始流程

用户提供的 Markdown 说明包含：购买代理出口 → 获取 IP/端口/账密/到期时间 → 在“有米”管理界面导入节点 → 指定 `Client.exe` 进程 → 根据运行过程中的流量数据判断连接 → 不稳定时更换地区和服务商节点。

事实层面：产品使用说明说明了操作入口和配置格式，但**没有披露核心代理驱动、TCP/UDP 归属判定、同名多 PID 处理以及代理出口防泄漏算法**。说明中关于同时运行数量、掉线规律的描述仅为作者经验，不能当成已验证的官方机制。

## ProxyBridge 源码可复用部分

官方仓库：https://github.com/InterceptSuite/ProxyBridge （Windows 在 `Windows/src/`）

| 源文件 | 当前能力 | 与新需求差异 |
|---|---|---|
| `pb_process.c` | 使用 Windows 连接表/缓存获取 PID，按 PID 解析进程名称 | 连接所有者信息已存在 |
| `pb_sockmap.c` | 从 WinDivert SOCKET 事件维护 TCP PID 归属 | 需验证 race condition / 端口复用 |
| `pb_rules.c` | `check_process_rule` 得到 PID，再调用 `match_rule(_v6)`，根据 `process_name` 匹配 | 缺以 `PID + 启动时间` 为优先键的策略接口 |
| `pb_internal.h` | `PROXY_RULE` 包含 `process_name`、`proxy_config_id`；连接数据保存 PID | 规则结构没有用于分配的 PID 字段 |
| `ProxyBridge.c` | TCP/UDP 拦截、连接记录以及部分统计和回调 | 需补强客户端级别的可验证分配与出站路径审计 |

截至检查时公开文档的 `.pbprofile` 规则通过 `ProcessName` 和 `ProxyConfigId` 指向代理；虽然核心会解析并记录 PID，但这**不意味着提供了直接的 PID -> 多代理分配管理接口**。

## 建议的最小改造点（尚未编写）

把“进程归属”与“进程名称路由”解耦，新增与具体应用无关的临时策略表：

```text
ProcessKey = { pid, start_time_fingerprint, exe_path }
Binding    = { ProcessKey, proxy_config_id, mode, generation }
FlowKey    = { family, protocol, local_ip, local_port, remote_ip, remote_port }
FlowState  = { FlowKey, ProcessKey, proxy_config_id, opened_at }
```

流程：

1. 在应用连接创建时从 Socket/Flow 元数据解析 PID，并校验进程创建时间与路径，防止 PID 复用错配。
2. 在路由时**先尝试匹配 PID 绑定**，未匹配再走已有 `ProcessName` 默认规则。
3. 对新建的 TCP 连接/UDP 流**固定代理身份**；变更节点只影响之后新建的连接。不要在长连接中途无条件改变出口。
4. 给每条实际代理流写入原始五元组、PID、代理 ID、TCP/UDP、成功/失败、错误原因和字节数，供 Go Web API 读取。
5. 对进程退出、PID 复用、监听端口变化、IPv6、DNS、UDP 无服务端 ASSOCIATE、代理停机等情况设计明确策略，避免假阳性“绿色”状态。

不能简单地在 `WinDivertOpen(WINDIVERT_LAYER_NETWORK)` 的抓包结果里直接读取 PID：WinDivert 文档明确说明 NETWORK 层没有 PID 信息，FLOW/SOCKET 才有。因此需要维护跨层的五元组到 PID 关联；如果关联失败，必须给出可诊断结果。

## Windows 专项验收（后续）

1. **先运行两个不同 PID、相同 EXE 的测试客户端**，让它们连接到自己控制的 TCP 回显服务，不使用真实游戏客户端当作最初测试工具。
2. 分别配置 SOCKS5 A/B 出口，后端测试服务器应能独立记录 A/B 的真实公网出口；同名不同 PID 不允许互串。
3. 再验证一个 UDP 测试客户端，经 SOCKS5 UDP ASSOCIATE 中继发包；同时检查 DNS、IPv6 和代理失效情况下是否发生直连。
4. 测试进程退出重启、PID/端口复用、新连接切换节点、驱动停止/重启、内存资源与 CPU/延迟，保证清理正确。
5. 最后才考虑在遵守游戏规则的前提下验证真实 Windows 客户端兼容性，避免修改游戏保护组件或绕过检测机制。

## 许可与产品风险

ProxyBridge 仓库标称 MIT，WinDivert 和其他依赖有独立条款；正式分发应逐一核对许可证、驱动签名及杀毒软件误报。不要从非官方链接获取“多开加速器”的安装包。

参考资料：
- https://github.com/InterceptSuite/ProxyBridge/blob/master/Windows/README.md
- https://github.com/InterceptSuite/ProxyBridge/blob/master/Windows/src/pb_rules.c
- https://github.com/InterceptSuite/ProxyBridge/blob/master/Windows/src/pb_process.c
- https://github.com/InterceptSuite/ProxyBridge/blob/master/Windows/src/pb_internal.h
- https://reqrypt.org/windivert-doc.html

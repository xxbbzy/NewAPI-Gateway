## Context

当前系统已经具备供应商分页管理、供应商余额同步、签到结果记录，以及基于 `provider_token + model` 的路由健康调节与无效响应抑制能力，但仍存在三类运营缺口：

1. 供应商数量增长后，管理端只能按分页浏览，无法快速搜索和汇总余额。
2. 系统只能在特定签到失败语义下自动关闭 `checkin_enabled`，无法表达“站点当前不可访问，因此不应继续用于同步、签到或上游转发”。
3. 所有供应商访问默认直连，无法按站点启用独立代理。

这次变更横跨 `model/`, `controller/`, `service/`, `router/` 和 `web/`，并会引入供应商数据模型扩展、管理 API 扩展、同步/签到/转发链路共用的网络访问配置，以及新的供应商健康可视化，因此需要先统一设计边界和状态语义。

## Goals / Non-Goals

**Goals:**
- 为供应商管理提供搜索入口和余额汇总能力，降低多站点运营成本。
- 为供应商建立独立于人工启停的健康状态，展示最近可访问结果，并在不可访问时阻止继续使用。
- 为供应商增加独立代理配置，并保证同步、签到、转发三条链路使用一致的网络出口策略。
- 对敏感代理信息提供安全返回、导出和日志策略。

**Non-Goals:**
- 不引入全局代理池、代理分组或代理健康调度。
- 不改变现有 `provider_token + model` 级路由健康倍率与 invalid suppression 的核心算法。
- 不把供应商健康状态直接复用为管理员手工 `status` 字段的语义。
- 不实现自动代理探测、代理连通性基准测试或多代理故障切换。

## Decisions

### Decision: Introduce provider-level operational health separate from manual status

新增供应商级运行状态字段，而不是复用现有 `provider.status`。

- `status` 继续表示管理员显式启用/禁用。
- 新增健康相关字段表示系统观测到的站点可达性，例如最近探测结果、最近成功访问时间、最近失败原因、熔断截止时间。
- 实际是否可被使用由“管理员状态 + 健康状态”共同决定。

Rationale:
- 管理员手工禁用和系统自动摘除属于不同来源的决策，混用一个字段会导致 UI、查询和恢复逻辑混乱。
- 现有 `provider-checkin-auto-disable-upstream-unavailable` 只处理签到语义，不适合承载“站点整体不可达”的平台级状态。

Alternatives considered:
- 直接把 `provider.status` 改成自动写入：实现简单，但会丢失人工禁用语义。
- 仅依赖路由层健康倍率/抑制：无法覆盖同步与签到链路，也不利于管理端直观展示。

### Decision: Treat provider health as a coarse circuit breaker across sync, checkin, and relay

供应商健康状态作为 provider 级熔断器，作用于以下链路：
- 后台同步 `pricing/tokens/balance`
- 供应商签到
- 实际 relay 转发选路

当站点访问失败满足“不可达”分类时，系统更新供应商健康状态为不可用，并在冷却期内阻止继续使用该供应商；成功访问后可恢复为健康。

Rationale:
- 用户需求明确要求“如果无法访问要标记出来并停止使用”。
- 仅在 relay 层摘除会导致后台同步/签到仍然反复失败，噪音较大。
- 仅在同步/签到中处理则不能保护线上流量。

Alternatives considered:
- 只停止 relay，不影响后台任务：实现风险小，但与“停止使用”语义不一致。
- 永久自动禁用供应商：恢复成本高，需要人工干预，不利于临时网络波动。

### Decision: Use provider-specific outbound proxy settings and apply them through shared HTTP client construction

新增供应商代理配置字段：
- `proxy_enabled`
- `proxy_url`

所有访问上游供应商的 HTTP 客户端均通过共享构造路径生成，并按供应商配置决定是否挂接代理传输层。这样 `UpstreamClient` 和 relay 转发复用相同的代理决策。

Rationale:
- 用户要求“按站点单独开启”。
- 同步、签到、relay 目前分散使用不同 HTTP 客户端，若不统一构造容易出现链路行为不一致。

Alternatives considered:
- 使用进程级环境变量代理：无法按供应商隔离。
- 仅为 `UpstreamClient` 加代理：会遗漏真实 relay。

### Decision: Keep provider list search server-side and reset pagination on query change

供应商搜索使用服务端查询扩展，在现有 `GET /api/provider/` 基础上增加 `keyword` 参数，并保持统一分页协议。

搜索字段优先覆盖：
- `name`
- `base_url`
- `remark`
- `user_id`

Rationale:
- 数据量增大后前端本页过滤无法解决跨页搜索问题。
- 系统已存在统一分页协议与“筛选变化回到第一页”的前端模式，可直接复用。

Alternatives considered:
- 额外新增 `/search` API：可行，但与当前 provider list API 分裂，没有必要。
- 仅前端过滤当前页：无法满足多页管理需求。

### Decision: Expose aggregated balance and provider operation summary as dedicated management summary data

新增供应商管理汇总响应，至少包含：
- 可统计余额的供应商数
- 余额总额（仅统计可解析余额）
- 余额更新时间概况
- 当前不可用供应商数
- 当前启用代理的供应商数

该汇总可作为 provider 管理页顶部概览，必要时再复用到 dashboard。

Rationale:
- “统计所有账户余额”本质是管理侧聚合视图，不需要新同步机制。
- 现有 `providers.balance` 已落库，适合在读取时聚合。

Alternatives considered:
- 只在 Dashboard 新增卡片：不利于供应商运营场景集中处理。
- 每次前端把列表求和：只能统计当前页，不可靠。

### Decision: Redact sensitive proxy credentials in normal responses, exports, and logs

`proxy_url` 可能内嵌用户名和密码，因此：
- 详情查询与列表查询默认返回 `proxy_enabled` 和脱敏后的代理摘要，不返回完整密文。
- 导出默认不包含完整代理密钥；如未来支持导出，也需显式设计敏感导出路径。
- 错误日志与健康状态中不得原样写出带凭证的代理 URL。

Rationale:
- 代理凭证与访问令牌一样属于高敏感信息。
- 当前 provider 导出会带出 `access_token` 用于重建，直接复制这一策略到代理字段会扩大泄漏面。

Alternatives considered:
- 直接像 `access_token` 一样完整导出：对运维方便，但默认风险过高。

## Risks / Trade-offs

- [健康状态误判导致临时摘除] → 使用明确的失败分类、冷却窗口与成功恢复机制，避免一次偶发失败就永久停用。
- [多链路代理改造带来行为不一致] → 统一 HTTP client / transport 构造，减少 `sync`、`checkin`、`relay` 各自拼装配置的机会。
- [旧数据迁移与 SQLite 容错复杂度上升] → 新增字段采用向后兼容默认值；读取层继续保持对 SQLite 历史脏数据的防御式投影思路。
- [余额总额受字符串格式影响不准确] → 聚合时仅对可解析美元余额计入总额，并返回“未计入账户数”或类似异常提示。
- [代理信息泄漏] → 在 controller 层统一清洗返回，在日志层禁止输出完整代理 URL。

## Migration Plan

1. 为 `providers` 表添加健康与代理相关字段，默认值保持兼容。
2. 扩展 provider 查询与 summary API，同时先接入前端搜索与概览展示。
3. 重构共享上游 HTTP client 构造，将代理能力接入同步、签到和 relay。
4. 在 provider 可达性失败路径中记录健康状态并接入“停止使用”判断。
5. 上线后观察健康摘除与恢复行为，再决定是否把部分汇总复用到 Dashboard。

Rollback:
- 保留新字段但停用健康判断与代理使用逻辑，回退到直连与原有 provider `status` 判断。
- 前端可隐藏新增概览和代理表单项，不影响原有供应商基础管理。

## Open Questions

- “停止使用”是否应同时阻止管理员手动触发同步/签到，还是仅阻止自动流程和 relay 选路。
- 健康恢复应完全依赖后续成功访问自动恢复，还是提供管理员手动“解除摘除/重试探测”入口。
- 代理导出是否需要单独的受限操作，还是明确不支持导出完整代理配置。

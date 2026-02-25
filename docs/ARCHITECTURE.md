# 系统架构

> 返回文档入口：[README.md](./README.md)

## 文档导航

- 上一篇：[QUICK_START.md](./QUICK_START.md)
- 下一篇：[API_REFERENCE.md](./API_REFERENCE.md)
- 研发支线：[PROJECT_STRUCTURE.md](./PROJECT_STRUCTURE.md)

## 架构目标

- 对客户端提供单一入口与单一令牌（`ag-`）。
- 对上游保持透明代理，避免暴露网关存在。
- 支持多供应商聚合路由与失败降级。
- 支持运营侧可观测（日志、统计、成本估算）。

## 逻辑组件

- `router/`：注册管理 API、Relay API、Web 路由。
- `middleware/`：鉴权、限流、CORS、缓存等横切能力。
- `controller/`：请求编排与参数校验。
- `service/`：代理转发、上游同步、签到、路由重建。
- `model/`：数据库实体与核心查询逻辑。
- `web/`：管理后台（React）。

## 核心请求链路（Relay）

```text
客户端
  -> /v1/*
  -> middleware.AggTokenAuth
      - 解析 ag-token（Authorization / x-api-key / x-goog-api-key / key）
      - 校验状态、过期时间、IP 白名单
  -> controller.Relay
      - 提取请求 model
      - 校验聚合 token 的模型白名单
      - BuildRouteAttemptsByPriority(model) 生成分层重试计划
      - 同优先级层按贡献值加权洗牌，失败后再降级下一层
  -> service.ProxyToUpstream
      - 替换认证为上游 sk-token
      - 清理代理特征 Header
      - 按命中路由改写请求体 model 后转发
      - 流式/非流式响应转发
      - 记录 usage_logs
```

## 管理链路（/api）

```text
后台用户
  -> /api/user/login 建立 Session
  -> /api/provider/* /api/route/* /api/log/* 等管理接口
  -> 控制器调用 model/service
  -> 落库并返回统一响应 {success,message,data}
```

说明：绝大多数管理接口使用 `NoTokenAuth`，要求 Session 访问，不接受用户 Token。

## 数据同步与路由重建

### 定时任务

- 同步任务：每 5 分钟执行一次 `syncAllProviders()`。
- 签到任务：每 24 小时执行一次 `CheckinAllProviders()`。

### 单供应商同步流程

1. 拉取上游 `GET /api/pricing`，写入 `model_pricings`。
2. 拉取上游 `GET /api/token/`（分页），写入 `provider_tokens`。
3. 拉取上游 `GET /api/user/self`，更新供应商余额。
4. 按 token 分组与模型可用组关系重建该供应商 `model_routes`。

## 路由算法

实现位置：`model/model_route.go`（`BuildRouteAttemptsByPriority` / `GetModelRouteOverview`）。

在候选筛选前，系统先通过统一模型目录（Model Catalog）解析请求模型，将 canonical/alias/上游目标名收敛为同一模型语义。

1. 汇总候选路由（同一候选池）：
   - 精确模型名匹配；
   - 供应商级手动映射（`providers.model_alias_mapping`）；
   - 模型归一化与版本无关键匹配。
2. 按优先级分层（降序），先尝试最高优先级层。
3. 对每条候选计算性价比评分 `value_score`：
   - 价格侧：基于模型价格计算 `unit_cost_usd`；
   - 预算侧：基于供应商余额与最近窗口内使用金额计算；
   - `recent_usage_cost_usd` 的窗口由 `RoutingUsageWindowHours`（默认 24）控制。
4. 将人工权重与评分融合为贡献值：
   - 基础值：`base = max(weight + 10, 0)`；
   - 同层存在有效评分时：`contribution_base = base * (RoutingBaseWeightFactor + normalize(value_score) * RoutingValueScoreFactor)`；
   - 同层评分不可用时：`contribution_base = base`。
5. 健康调节（默认关闭）：
   - 开关：`RoutingHealthAdjustmentEnabled`；
   - 样本窗口：`RoutingHealthWindowHours`（默认 6）；
   - 仅当样本数达到 `RoutingHealthMinSamples`（默认 5）才生效；
   - 结合失败率、成功率与平均延迟生成 `health_multiplier`，并限制在 `[RoutingHealthMinMultiplier, RoutingHealthMaxMultiplier]`。
6. 最终贡献值：`contribution = contribution_base * health_multiplier`（健康调节关闭时倍率为 `1`）。
7. 同层按贡献值执行“加权随机不放回”生成完整重试顺序，层内全部失败后再降级到下一优先级层。

说明：

- `weight + 10` 仍用于保留低权重路由的基础概率。
- 路由调参项（价值评分 + 健康调节）均可通过系统选项实时调整。

## 透明代理策略

- 认证替换：客户端 `ag-` -> 上游 `sk-`。
- Header 清理：删除 `X-Forwarded-*`、`Via`、`Forwarded`、`X-Real-IP`。
- 请求体改写：Relay 会将请求模型解析为 canonical 语义，再按路由命中的上游目标模型改写 `model` 字段。
- 支持 SSE：实时转发流式响应并记录首 token 延迟。

## 关键数据表

- `providers`：上游实例信息与同步元数据。
- `provider_tokens`：上游 sk token 与分组。
- `model_pricings`：模型定价与可用分组缓存。
- `model_routes`：模型到 token 的路由映射。
- `aggregated_tokens`：用户 ag token。
- `usage_logs`：调用日志、token/cost/延迟统计。

## 相关文档

- 快速开始：[QUICK_START.md](./QUICK_START.md)
- 配置项：[CONFIGURATION.md](./CONFIGURATION.md)
- API 细节：[API_REFERENCE.md](./API_REFERENCE.md)
- 数据表说明：[DATABASE_SCHEMA.md](./DATABASE_SCHEMA.md)
- 运维动作：[OPERATIONS.md](./OPERATIONS.md)
- 模型别名专题：[model-alias-manual-mapping.md](./model-alias-manual-mapping.md)

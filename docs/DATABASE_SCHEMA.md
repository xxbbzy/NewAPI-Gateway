# 数据模型说明

> 返回文档入口：[README.md](./README.md)

## 文档导航

- 上一篇：[DEVELOPMENT.md](./DEVELOPMENT.md)
- 下一篇：[OPERATIONS.md](./OPERATIONS.md)
- 架构主线：[ARCHITECTURE.md](./ARCHITECTURE.md)

## 数据库支持

项目通过 GORM 支持以下数据库：

- SQLite（默认）
- MySQL
- PostgreSQL

启动时会自动执行 `AutoMigrate` 创建/更新表结构。

## 核心数据表

| 表名 | 说明 | 关键字段 |
| --- | --- | --- |
| `users` | 管理台用户 | `id`, `username`, `role`, `status`, `token` |
| `options` | 系统 KV 配置 | `key`, `value` |
| `providers` | 上游供应商 | `id`, `name`, `base_url`, `access_token`, `status`, `priority`, `weight` |
| `provider_tokens` | 上游 sk token 缓存 | `id`, `provider_id`, `upstream_token_id`, `sk_key`, `group_name`, `status` |
| `aggregated_tokens` | 聚合令牌（ag） | `id`, `user_id`, `key`, `status`, `expired_time`, `model_limits`, `allow_ips` |
| `model_pricings` | 上游模型定价与能力缓存 | `model_name`, `provider_id`, `quota_type`, `enable_groups` |
| `model_routes` | 模型路由表 | `model_name`, `provider_id`, `provider_token_id`, `priority`, `weight`, `enabled` |
| `usage_logs` | 调用日志与统计 | `user_id`, `provider_name`, `model_name`, `request_id`, `relay_request_id`, `attempt_index`, `usage_source`, `cost_usd`, `response_time_ms`, `created_at` |

## 字段语义要点

### providers

- `priority`、`weight`：作为上游 token 默认路由权重来源。
- `pricing_group_ratio`：上游分组倍率缓存（JSON）。
- `pricing_usable_group`：上游可用/默认分组信息缓存（JSON）。
- `pricing_supported_endpoint`：上游支持端点缓存（JSON）。

### provider_tokens

- `sk_key`：上游真实鉴权 key（敏感字段）。
- `group_name`：用于匹配 `model_pricings.enable_groups`。
- `model_limits`：可选模型限制（逗号分隔）。

### aggregated_tokens

- `key`：数据库中不含 `ag-` 前缀；对外返回时拼接 `ag-`。
- `model_limits_enabled + model_limits`：控制聚合 token 可用模型。
- `allow_ips`：按行分隔的 IP 白名单。

### model_routes

- 按 `(model_name, provider_token_id)` 形成路由候选。
- 选择算法按 `priority` 分层，并基于 `weight + 10`、`value_score`、`health_multiplier` 计算最终贡献值。
- 同一优先级层内按贡献值进行“加权随机不放回”生成重试顺序。

### usage_logs

- 支持记录流式/非流式请求、首 token 延迟、估算成本。
- `request_id` 表示单次上游尝试；`relay_request_id` 关联同一次客户端请求下的多次尝试。
- `attempt_index` 表示同一 `relay_request_id` 内的尝试顺序。
- `usage_source` 表示 usage 质量：`exact` / `estimated` / `missing`。
- 可按 provider/model/status/关键词筛选与聚合统计，默认以请求级口径（可切换 attempt 口径）。
- 路由健康调节会消费该表中的成功率、失败率与平均延迟统计。

## 数据流关系

1. `providers` 定义上游。
2. 同步任务从上游写入 `provider_tokens` 与 `model_pricings`。
3. 重建任务生成 `model_routes`。
4. 请求通过 `aggregated_tokens` 鉴权后命中 `model_routes`。
5. 代理结果写入 `usage_logs`。

## 相关文档

- 架构说明：[ARCHITECTURE.md](./ARCHITECTURE.md)
- API 参考：[API_REFERENCE.md](./API_REFERENCE.md)
- 运维手册：[OPERATIONS.md](./OPERATIONS.md)
- 上线指引：[USAGE_LOG_ROLLOUT.md](./USAGE_LOG_ROLLOUT.md)
- 项目结构：[PROJECT_STRUCTURE.md](./PROJECT_STRUCTURE.md)

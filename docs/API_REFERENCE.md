# API 参考文档

> 返回文档入口：[README.md](./README.md)

## 文档导航

- 上一篇：[ARCHITECTURE.md](./ARCHITECTURE.md)
- 下一篇：[CONFIGURATION.md](./CONFIGURATION.md)
- 入口索引：[README.md](./README.md)

## 认证与会话

### 1. Relay API 认证（聚合 Token）

支持以下方式携带 `ag-` 令牌：

```http
Authorization: Bearer ag-xxxxxxxx
x-api-key: ag-xxxxxxxx
x-goog-api-key: ag-xxxxxxxx
GET /v1beta/models/xxx?key=ag-xxxxxxxx
```

### 2. 管理 API 认证（Session 为主）

- 先调用 `POST /api/user/login` 登录，使用 Cookie Session 访问管理接口。
- 多数管理接口启用了 `NoTokenAuth`，不支持用户 Token。
- 仅少数未加 `NoTokenAuth` 的接口可用 `Authorization: Bearer <user-token>`（例如 `GET /api/dashboard`）。

## 响应格式

### Relay 接口

- 成功响应：上游透传。
- 失败响应：OpenAI 风格。

```json
{
  "error": {
    "message": "error detail",
    "type": "authentication_error",
    "code": "invalid_api_key"
  }
}
```

### 管理接口

```json
{
  "success": true,
  "message": "",
  "data": {}
}
```

## Relay API（`/v1*`）

| Method | Path | 说明 |
| --- | --- | --- |
| POST | `/v1/chat/completions` | Chat 补全 |
| POST | `/v1/completions` | Text 补全 |
| POST | `/v1/embeddings` | 向量生成 |
| POST | `/v1/images/generations` | 图片生成 |
| POST | `/v1/audio/speech` | 文本转语音 |
| POST | `/v1/audio/transcriptions` | 语音转文本 |
| POST | `/v1/moderations` | 内容审核 |
| POST | `/v1/rerank` | 重排序 |
| POST | `/v1/video/generations` | 视频生成 |
| POST | `/v1/responses` | OpenAI Responses |
| POST | `/v1/messages` | Anthropic 兼容 |
| POST | `/v1beta/models/*path` | Gemini 兼容 |
| GET | `/v1/models` | 获取可用模型 |
| GET | `/v1/models/:model` | 获取模型详情 |
| GET | `/dashboard/billing/subscription` | 兼容返回（模拟） |
| GET | `/dashboard/billing/usage` | 兼容返回（模拟） |

## 公共与登录相关 API（`/api`）

| Method | Path | 认证 | 说明 |
| --- | --- | --- | --- |
| GET | `/api/status` | 无 | 系统状态 |
| GET | `/api/notice` | 无 | 公告 |
| GET | `/api/about` | 无 | 关于 |
| GET | `/api/verification` | 无 | 发送邮箱验证码（限流 + Turnstile） |
| GET | `/api/reset_password` | 无 | 发送密码重置邮件 |
| POST | `/api/user/reset` | 无 | 使用邮箱 token 重置密码 |
| POST | `/api/user/register` | 无 | 注册 |
| POST | `/api/user/login` | 无 | 登录并建立 Session |
| GET | `/api/user/logout` | 无 | 登出 |
| GET | `/api/oauth/github` | 无 | GitHub OAuth 回调 |
| GET | `/api/oauth/wechat` | 无 | 微信登录 |
| GET | `/api/oauth/wechat/bind` | UserAuth | 微信绑定 |
| GET | `/api/oauth/email/bind` | UserAuth | 邮箱绑定 |

## 用户与管理员 API

### 用户自助（Session，`UserAuth + NoTokenAuth`）

| Method | Path | 说明 |
| --- | --- | --- |
| GET | `/api/user/self` | 获取当前用户 |
| PUT | `/api/user/self` | 更新当前用户 |
| DELETE | `/api/user/self` | 删除当前用户 |
| GET | `/api/user/token` | 生成用户 Token |

### 管理员用户管理（Session，`AdminAuth + NoTokenAuth`）

| Method | Path | 说明 |
| --- | --- | --- |
| GET | `/api/user/` | 用户列表 |
| GET | `/api/user/search` | 用户搜索 |
| GET | `/api/user/:id` | 用户详情 |
| POST | `/api/user/` | 创建用户 |
| POST | `/api/user/manage` | 用户状态/角色管理 |
| PUT | `/api/user/` | 更新用户 |
| DELETE | `/api/user/:id` | 删除用户 |

### 系统选项（Session，`RootAuth + NoTokenAuth`）

| Method | Path | 说明 |
| --- | --- | --- |
| GET | `/api/option/` | 读取系统选项（隐藏 Secret/Token 字段） |
| PUT | `/api/option/` | 更新系统选项 |

路由策略相关系统选项（通过 `PUT /api/option/` 更新）：

| Key | 类型 | 默认值 | 取值范围 | 说明 |
| --- | --- | --- | --- | --- |
| `CheckinScheduleEnabled` | bool | `true` | `true/false` | 是否启用每日自动签到任务 |
| `CheckinScheduleTime` | string | `"09:00"` | `HH:mm` | 每日签到执行时间（24 小时制） |
| `CheckinScheduleTimezone` | string | `"Asia/Shanghai"` | IANA 时区 | 每日签到任务使用的时区 |
| `RoutingUsageWindowHours` | int | `24` | `1 ~ 720` | 计算 `recent_usage_cost_usd` 的统计窗口（小时） |
| `RoutingBaseWeightFactor` | float | `0.2` | `0 ~ 10` | 占比贡献中的基础系数 |
| `RoutingValueScoreFactor` | float | `0.8` | `0 ~ 10` | 占比贡献中的性价比系数 |
| `RoutingHealthAdjustmentEnabled` | bool | `false` | `true/false` | 是否启用健康调节倍率 |
| `RoutingHealthWindowHours` | int | `6` | `1 ~ 720` | 健康统计窗口（小时） |
| `RoutingFailurePenaltyAlpha` | float | `4.0` | `0 ~ 20` | 失败率惩罚系数（越大惩罚越强） |
| `RoutingHealthRewardBeta` | float | `0.08` | `0 ~ 2` | 健康奖励系数（越大奖励越强） |
| `RoutingHealthMinMultiplier` | float | `0.05` | `0 ~ 10` | 健康倍率下限 |
| `RoutingHealthMaxMultiplier` | float | `1.12` | `0 ~ 10` | 健康倍率上限 |
| `RoutingHealthMinSamples` | int | `5` | `1 ~ 1000` | 启用健康调节所需最小样本数 |

占比贡献公式：

- 基础贡献：`contribution_base = max(weight + 10, 0) * (RoutingBaseWeightFactor + normalize(value_score) * RoutingValueScoreFactor)`
- 若同层无有效 `value_score`，回退为 `contribution_base = max(weight + 10, 0)`
- 最终贡献：`contribution = contribution_base * health_multiplier`
- 健康调节关闭或样本不足时，`health_multiplier = 1`

## Provider 相关 API（Session，`AdminAuth + NoTokenAuth`）

| Method | Path | 说明 |
| --- | --- | --- |
| GET | `/api/provider/` | 供应商列表 |
| GET | `/api/provider/export` | 导出供应商 |
| POST | `/api/provider/import` | 导入供应商 |
| GET | `/api/provider/checkin/summary` | 获取签到任务汇总（支持 `?limit=`） |
| GET | `/api/provider/checkin/messages` | 获取签到结果消息（支持 `?limit=`） |
| GET | `/api/provider/checkin/uncheckin` | 获取当日未签到渠道列表 |
| POST | `/api/provider/checkin/run` | 触发全量签到任务 |
| GET | `/api/provider/:id` | 供应商详情 |
| POST | `/api/provider/` | 创建供应商 |
| PUT | `/api/provider/` | 更新供应商 |
| DELETE | `/api/provider/:id` | 删除供应商 |
| POST | `/api/provider/:id/sync` | 触发单供应商同步 |
| POST | `/api/provider/:id/checkin` | 手动签到 |
| GET | `/api/provider/:id/tokens` | 获取供应商 token 列表 |
| GET | `/api/provider/:id/pricing` | 获取供应商 pricing 缓存 |
| GET | `/api/provider/:id/model-alias-mapping` | 获取模型别名手动映射 |
| PUT | `/api/provider/:id/model-alias-mapping` | 更新模型别名手动映射 |
| POST | `/api/provider/:id/tokens` | 在上游创建 token 并回同步 |
| PUT | `/api/provider/token/:token_id` | 更新本地 token 字段 |
| DELETE | `/api/provider/token/:token_id` | 删除 token（先删上游再删本地） |

## 聚合 Token API（Session，`UserAuth + NoTokenAuth`）

| Method | Path | 说明 |
| --- | --- | --- |
| GET | `/api/agg-token/` | 当前用户聚合 token 列表 |
| POST | `/api/agg-token/` | 创建聚合 token |
| PUT | `/api/agg-token/` | 更新聚合 token |
| DELETE | `/api/agg-token/:id` | 删除聚合 token |

`POST /api/agg-token/` 成功后 `data` 直接返回完整令牌字符串（形如 `ag-xxxx`）。

## 路由管理 API（Session，`AdminAuth + NoTokenAuth`）

| Method | Path | 说明 |
| --- | --- | --- |
| GET | `/api/route/` | 路由列表（支持 `?model=`） |
| GET | `/api/route/overview` | 路由总览（支持 `model/provider_id/enabled_only`） |
| GET | `/api/route/models` | 已接入模型列表 |
| PUT | `/api/route/:id` | 更新单条路由（priority/weight/enabled） |
| POST | `/api/route/batch-update` | 批量更新路由 |
| POST | `/api/route/rebuild` | 触发全量路由重建 |

## 日志与统计 API

### 日志查询（Session）

| Method | Path | 认证 | 说明 |
| --- | --- | --- | --- |
| GET | `/api/log/self` | UserAuth + NoTokenAuth | 当前用户日志 |
| GET | `/api/log/` | AdminAuth + NoTokenAuth | 全部日志 |

日志查询支持参数：

- `p`：页码（从 0 起）
- `page_size`：每页条数
- `keyword`：关键词（匹配模型/供应商/request_id/error/client_ip）
- `provider`：供应商名称精确筛选
- `status`：`all` / `success` / `error`
- `view`：`all` / `error`

### 仪表盘

| Method | Path | 认证 | 说明 |
| --- | --- | --- | --- |
| GET | `/api/dashboard` | AdminAuth | 聚合统计（请求量/成功率/模型排行/趋势） |

## 常见请求示例

### 1. 登录

```bash
curl -X POST http://localhost:3000/api/user/login \
  -H "Content-Type: application/json" \
  -d '{"username":"root","password":"123456"}'
```

### 2. 创建聚合 Token（需带登录 Cookie）

```bash
curl -X POST http://localhost:3000/api/agg-token/ \
  -H "Content-Type: application/json" \
  -b 'session=<your-session-cookie>' \
  -d '{"name":"demo","expired_time":-1}'
```

### 3. 触发供应商同步（需管理员 Session）

```bash
curl -X POST http://localhost:3000/api/provider/1/sync \
  -b 'session=<your-session-cookie>'
```

### 4. Relay 调用

```bash
curl http://localhost:3000/v1/chat/completions \
  -H "Authorization: Bearer ag-your-token" \
  -H "Content-Type: application/json" \
  -d '{"model":"gpt-4o","messages":[{"role":"user","content":"hello"}]}'
```

## Relay 常见错误码

| HTTP | type | code | 说明 |
| --- | --- | --- | --- |
| 401 | `authentication_error` | `invalid_api_key` | 聚合 token 缺失/无效/过期 |
| 403 | `permission_error` | `ip_not_allowed` | IP 不在白名单 |
| 403 | `permission_error` | `model_not_allowed` | 模型不在白名单 |
| 503 | `server_error` | `service_unavailable` | 无可用路由或上游不可用 |
| 502 | `server_error` | - | 上游请求失败 |

## 相关文档

- 架构说明：[ARCHITECTURE.md](./ARCHITECTURE.md)
- 开发指南：[DEVELOPMENT.md](./DEVELOPMENT.md)
- 配置说明：[CONFIGURATION.md](./CONFIGURATION.md)
- 模型别名专题：[model-alias-manual-mapping.md](./model-alias-manual-mapping.md)

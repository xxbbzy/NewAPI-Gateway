# 供应商插件 Token 对接改造实施文档

> 返回文档入口：[README.md](./README.md)

## 文档导航

- 上一篇：[API_REFERENCE.md](./API_REFERENCE.md)
- 下一篇：[OPERATIONS.md](./OPERATIONS.md)
- 入口索引：[README.md](./README.md)

## 1. 背景与目标

目标是在不依赖浏览器登录 Session 的前提下，让外部自动化客户端（如 Chrome 插件）可以基于“个人访问令牌”安全地对接供应商管理能力，包括：

- 拉取供应商与同步状态；
- 新增/更新供应商配置；
- 触发供应商同步；
- 管理供应商 token（新增/更新/删除）；
- 更新供应商模型别名映射。

本次仅定义后端管理 API 改造方案，不涉及插件端实现细节。

## 2. 现状与问题

当前管理路由（`/api/provider/*`）统一使用 `AdminAuth + NoTokenAuth`，会显式拒绝 token 鉴权。

- 用户令牌生成接口：`GET /api/user/token`（需 Session）
- token 校验：`Authorization: Bearer <token>`
- 关键限制：`NoTokenAuth` 下返回“本接口不支持使用 token 进行验证”

因此，当前“个人设置中生成访问令牌”不能直接调用供应商管理 API。

## 3. 改造原则

1. 保持现有 Web 管理台行为不变（继续走 Session + 旧路由）。
2. 新增插件专用路由，不直接改旧路由的鉴权策略。
3. 插件专用路由强制 `TokenOnlyAuth`，避免 Session 混用。
4. 最小权限原则：仅开放供应商管理所需能力，不扩大系统配置权限。
5. 响应协议与现有管理 API 对齐（`success/message/data`）。

## 4. 目标接口设计

### 4.1 路由分组

- 前缀：`/api/plugin/provider`
- 鉴权：`AdminAuth + TokenOnlyAuth`
- Header：`Authorization: Bearer <personal_access_token>`

说明：插件需使用管理员（`role >= 10`）的个人访问令牌。

### 4.2 接口清单（建议）

| Method | Path | 说明 |
| --- | --- | --- |
| GET | `/api/plugin/provider/?p=0&page_size=20` | 分页查询供应商 |
| GET | `/api/plugin/provider/:id` | 查询供应商详情 |
| POST | `/api/plugin/provider/` | 新增供应商 |
| PUT | `/api/plugin/provider/` | 更新供应商 |
| POST | `/api/plugin/provider/import` | 批量导入（upsert） |
| POST | `/api/plugin/provider/:id/sync` | 触发供应商同步（异步） |
| GET | `/api/plugin/provider/:id/pricing` | 查询 pricing/分组/endpoint |
| GET | `/api/plugin/provider/:id/tokens` | 查询供应商 token 列表（脱敏） |
| POST | `/api/plugin/provider/:id/tokens` | 在上游创建 token 并回同步 |
| PUT | `/api/plugin/provider/token/:token_id` | 更新本地 token 字段 |
| DELETE | `/api/plugin/provider/token/:token_id` | 删除 token（上游+本地） |
| GET | `/api/plugin/provider/:id/model-alias-mapping` | 查询别名映射 |
| PUT | `/api/plugin/provider/:id/model-alias-mapping` | 更新别名映射 |

实现策略建议：

- 第一阶段直接复用现有 `controller/provider.go` 处理函数；
- 如需插件专属审计字段，再增加适配层 controller。

### 4.3 请求与响应约定

- 统一响应：

```json
{
  "success": true,
  "message": "",
  "data": {}
}
```

- 分页结构：沿用 `items/p/page_size/total/total_pages/has_more`。
- 失败判定：优先看 `success=false`，不要仅依赖 HTTP 状态码。

### 4.4 关键业务行为

1. `GET /:id/pricing` 返回：
   - `token_group_options`
   - `default_group`
   - `supported_endpoint`
2. `POST /:id/tokens` 约束：
   - `group_name` 不能为空；
   - 必须属于该供应商可用分组；
   - 创建成功后异步回同步。
3. 供应商与 token 查询默认脱敏：
   - 供应商响应中 `access_token` 为空；
   - token 响应中 `sk_key` 为脱敏字符串。

## 5. 插件调用推荐流程

1. 管理员在 Web 端登录后，通过“个人设置 -> 生成访问令牌”获取 token。
2. 插件保存 token，并调用 `/api/plugin/provider/import` 写入抓取到的供应商配置。
3. 对新增/更新的供应商调用 `/api/plugin/provider/:id/sync` 触发同步。
4. 轮询 `/api/plugin/provider/:id/pricing` 与 `/api/plugin/provider/:id/tokens`，确认同步结果。
5. 需要精细化维护上游 token 时，调用 token 管理接口进行增删改。

## 6. 安全与风控要求

1. 令牌安全：
   - 插件端仅存储管理员个人 token，不存储上游 `access_token/sk_key` 明文；
   - 用户重置个人 token 后，旧 token 必须立即失效（沿用现有逻辑）。
2. 最小暴露：
   - 插件只开放 `/api/plugin/provider/*`，不开放 `/api/option/*`、`/api/user/manage` 等高危接口。
3. 审计建议：
   - 新增日志标签（如 `source=plugin_token_api`）区分插件调用与 Web 调用。
4. 速率控制：
   - 沿用全局 API 限流；
   - 对高频同步场景建议增加调用节流（插件侧与服务端侧双重控制）。

## 7. 兼容性策略

1. 旧前端零改动：
   - 保留 `/api/provider/*` 的 `NoTokenAuth`。
2. 增量开放：
   - 新增 `/api/plugin/provider/*` 路由组，避免破坏既有权限边界。
3. 文档同步：
   - `API_REFERENCE.md` 增加插件 Token 管理接口章节；
   - `FAQ.md` 增补“插件 token 接口如何使用”。

## 8. 实施步骤

1. 路由层改造
   - 新增插件路由组与中间件组合（`AdminAuth + TokenOnlyAuth`）。
2. 接口复用接线
   - 复用现有 Provider 相关 controller。
3. 文档与示例
   - 更新 API 文档与调用示例（curl）。
4. 测试补齐
   - 新增 token-only 的鉴权测试与核心流程回归测试。

## 9. 测试与验收标准

### 9.1 功能验收

- 使用管理员 token 可完成供应商增删改查、同步、token 管理、别名映射更新。
- 使用普通用户 token 访问插件路由返回权限不足。
- 使用 Session 访问插件路由返回“仅支持 token”。

### 9.2 安全验收

- 所有插件路由都不返回上游敏感明文（`access_token/sk_key`）。
- token 失效后请求被拒绝。

### 9.3 回归验收

- 原有 `/api/provider/*` Web 操作流程不受影响。
- 现有管理页面功能与自动同步/路由重建行为一致。

## 10. 风险与回滚

### 10.1 主要风险

1. 误开放权限：插件路由误挂 `UserAuth` 或漏加 `TokenOnlyAuth`。
2. 行为偏差：复用 controller 时出现插件与 Web 参数口径不一致。
3. 调用放大：插件批量同步触发上游限频。

### 10.2 回滚方案

1. 关闭或移除 `/api/plugin/provider/*` 路由组；
2. 保留旧 `/api/provider/*` 路由不变；
3. 下线插件调用并恢复人工/后台同步流程。

## 11. 里程碑建议

- M1：完成路由改造与鉴权联调。
- M2：完成核心接口回归与文档更新。
- M3：小范围插件灰度（单管理员账号）。
- M4：正式开放并持续监控错误率与同步耗时。

## 相关文档

- [API_REFERENCE.md](./API_REFERENCE.md)
- [FAQ.md](./FAQ.md)
- [OPERATIONS.md](./OPERATIONS.md)

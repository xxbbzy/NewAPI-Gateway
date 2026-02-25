# 常见问题（FAQ）

> 返回文档入口：[README.md](./README.md)

## 文档导航

- 上一篇：[OPERATIONS.md](./OPERATIONS.md)
- 下一篇：[DEVELOPMENT.md](./DEVELOPMENT.md)
- 接口参考：[API_REFERENCE.md](./API_REFERENCE.md)

## 1. 首次启动账号密码是什么？

默认是 `root / 123456`。首次启动时若数据库无用户，会自动创建该账号。

## 2. 为什么我设置了用户 Token 访问管理接口仍失败？

大多数管理接口启用了 `NoTokenAuth`，要求使用登录 Session。请通过 `/api/user/login` 登录后访问。

## 3. 插件如何使用个人访问令牌管理供应商？

请使用插件专用路由 `/api/plugin/provider/*`，并在请求头携带管理员个人令牌：

- `Authorization: Bearer <admin-user-token>`
- 该分组要求 `AdminAuth + TokenOnlyAuth`：仅 Session 会被拒绝，非管理员 token 也会被拒绝。
- 旧的 `/api/provider/*` 仍是 Session 管理路由，不支持 token 调用。

## 4. `ag-` 令牌和上游 `sk-` 令牌有什么区别？

- `ag-`：网关对外提供给客户端使用。
- `sk-`：上游 NewAPI token，仅网关内部使用，不对客户端暴露。

## 5. 为什么调用模型时提示 `model_not_allowed`？

聚合 token 开启了模型白名单，但请求模型不在 `model_limits` 内。更新 token 白名单后重试。

## 6. 为什么会返回 `service_unavailable`？

当前模型没有可用路由，或所有候选上游失败。请同步供应商并重建路由后再试。

## 7. 项目是否处理计费或充值？

不处理。项目只做透明代理与使用统计。

## 8. 不配置 Redis 可以运行吗？

可以。系统会回退为 Cookie Session + 内存限流。

## 9. 如何查看可用模型列表？

调用 `GET /v1/models`，返回当前可调用的 canonical 模型列表（会按聚合 token 的 `model_limits` 过滤）。
`GET /v1/models/:model` 支持 canonical 或 alias 查询。

## 10. 如何让不同供应商按比例分流？

通过路由的 `priority` 与 `weight` 控制基础流量，再叠加动态评分：

- 先按 `priority` 分层，高优先级先尝试；
- 层内以 `weight + 10` 作为基础权重；
- 结合价格、余额、最近消耗计算 `value_score` 动态放大/缩小占比；
- 可选开启健康调节（成功率/失败率/延迟）进一步修正贡献值。

因此最终分流比例不是固定静态权重，而是“人工权重 + 实时状态”共同决定。

## 11. 如何启用更详细的代理认证日志？

设置环境变量 `DEBUG_PROXY_AUTH=1`，排障结束后应关闭。

## 12. 为什么反向代理后流式输出很慢？

通常是代理层开启了响应缓冲。请关闭缓冲并提高读超时。

## 13. 文档更新后，如何确保团队一致？

建议在 PR 流程中把“文档同步更新”作为必选检查项：接口变更更新 `API_REFERENCE`，配置变更更新 `CONFIGURATION`。

## 相关文档

- 快速开始：[QUICK_START.md](./QUICK_START.md)
- 运维手册：[OPERATIONS.md](./OPERATIONS.md)
- API 参考：[API_REFERENCE.md](./API_REFERENCE.md)
- 模型别名专题：[model-alias-manual-mapping.md](./model-alias-manual-mapping.md)

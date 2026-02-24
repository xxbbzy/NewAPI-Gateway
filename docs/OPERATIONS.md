# 运维手册

> 返回文档入口：[README.md](./README.md)

## 文档导航

- 上一篇：[DEPLOYMENT.md](./DEPLOYMENT.md)
- 下一篇：[FAQ.md](./FAQ.md)
- 接口索引：[API_REFERENCE.md](./API_REFERENCE.md)

## 日常巡检清单

1. 登录后台检查 `Dashboard` 请求量、失败率、耗时趋势。
2. 检查供应商列表中的余额更新时间与同步状态。
3. 抽查 `Routes` 页面，确认关键模型存在可用路由。
4. 检查日志中是否有持续 `upstream request failed` 或 `service_unavailable`。

## 关键运维动作

### 手动同步单个供应商

- 后台：`供应商 -> 同步`
- API：`POST /api/provider/:id/sync`

### 手动签到

- 后台：`供应商 -> 签到`
- API：`POST /api/provider/:id/checkin`

### 手动触发未签到渠道签到

- 后台：`供应商 -> 签到未签到渠道`
- API：`POST /api/provider/checkin/run`
- 行为：仅执行当日未签到且已启用签到的渠道；若无未签到渠道会返回明确完成提示。

### 查看签到结果与未签到渠道

- 签到汇总：`GET /api/provider/checkin/summary?limit=1`
- 签到消息：`GET /api/provider/checkin/messages?limit=20`
- 未签到渠道：`GET /api/provider/checkin/uncheckin`

### 全量重建路由

- API：`POST /api/route/rebuild`
- 适用场景：大量变更 token 分组、模型定价或供应商权重后。

## 故障排查

### 现象：Relay 返回 401 invalid_api_key

排查步骤：

1. 确认请求头是否带 `ag-` 聚合 token。
2. 检查聚合 token 是否禁用或过期。
3. 检查调用 IP 是否在 `allow_ips` 白名单内。

### 现象：Relay 返回 403 model_not_allowed

排查步骤：

1. 检查聚合 token 的 `model_limits_enabled`。
2. 确认请求 `model` 是否在白名单。

### 现象：Relay 返回 503 service_unavailable

排查步骤：

1. 检查目标模型在 `model_routes` 是否存在并启用。
2. 触发供应商同步与路由重建。
3. 检查上游网络、上游 token 可用性。

### 现象：流式响应卡住

排查步骤：

1. 确认上游接口本身支持 SSE。
2. 确认反向代理关闭了响应缓冲（`proxy_buffering off`）。
3. 检查网络层超时配置（`proxy_read_timeout`）。

## 数据备份与恢复

### SQLite

备份：

```bash
cp gateway-aggregator.db gateway-aggregator.db.bak.$(date +%Y%m%d%H%M%S)
```

恢复：

```bash
cp gateway-aggregator.db.bak.<timestamp> gateway-aggregator.db
```

建议在服务停止或低峰时执行，避免并发写入冲突。

### MySQL / PostgreSQL

按各自标准工具进行逻辑备份（`mysqldump` / `pg_dump`）。

## 安全建议

1. 首次登录后立即修改 `root` 密码。
2. 为聚合 token 启用模型白名单与 IP 白名单。
3. 定期轮换上游 `access_token` 与 `provider_tokens`。
4. 生产环境固定 `SESSION_SECRET`，避免会话漂移。
5. 排障完成后关闭 `DEBUG_PROXY_AUTH`。

## 相关文档

- 部署指南：[DEPLOYMENT.md](./DEPLOYMENT.md)
- 配置说明：[CONFIGURATION.md](./CONFIGURATION.md)
- 常见问题：[FAQ.md](./FAQ.md)
- API 参考：[API_REFERENCE.md](./API_REFERENCE.md)

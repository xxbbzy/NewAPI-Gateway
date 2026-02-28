# Usage Log 请求聚合与解析扩展上线说明

## 目标

本次变更把 `usage_logs` 语义拆分为：

- 请求级（`relay_request_id`）：面向业务统计，默认聚合口径
- 尝试级（`request_id` + `attempt_index`）：面向重试链路排障

并新增 usage 质量标记：

- `usage_source`: `exact` / `estimated` / `missing`
- `usage_parser`: 命中解析器 ID

## 数据迁移（加法变更）

通过 GORM `AutoMigrate` 自动加字段（向后兼容）：

- `relay_request_id`（index）
- `attempt_index`（index）
- `usage_source`（index）
- `usage_parser`

旧数据不会被重写。历史行在查询时会使用回退逻辑（基于 `request_id` 或 `id`）参与请求级聚合。

## 发布顺序建议

1. 部署包含新字段写入与查询逻辑的版本。
2. 观察日志列表/仪表盘默认 `request` 聚合表现。
3. 对比 `aggregation=request` 与 `aggregation=attempt` 的差异，确认重试链可见性不受影响。
4. 关注 `usage_source` 分布，确认 `missing` 比例下降并稳定。

## 验证清单（生产样本）

### 1) 解析命中率

```sql
SELECT usage_source, COUNT(*) AS cnt
FROM usage_logs
GROUP BY usage_source
ORDER BY cnt DESC;
```

```sql
SELECT usage_parser, COUNT(*) AS cnt
FROM usage_logs
GROUP BY usage_parser
ORDER BY cnt DESC
LIMIT 20;
```

### 2) 零 token 比例（按模型族）

```sql
SELECT
  CASE
    WHEN LOWER(model_name) LIKE '%claude%' THEN 'claude'
    WHEN LOWER(model_name) LIKE '%gpt%' OR LOWER(model_name) LIKE '%o1%' OR LOWER(model_name) LIKE '%o3%' THEN 'gpt-like'
    WHEN LOWER(model_name) LIKE '%deepseek%' THEN 'deepseek'
    WHEN LOWER(model_name) LIKE '%glm%' OR LOWER(model_name) LIKE '%z-ai%' THEN 'glm'
    WHEN LOWER(model_name) LIKE '%grok%' THEN 'grok'
    ELSE 'other'
  END AS family,
  COUNT(*) AS cnt,
  SUM(CASE WHEN prompt_tokens=0 AND completion_tokens=0 THEN 1 ELSE 0 END) AS zero_cnt
FROM usage_logs
WHERE status=1
GROUP BY family
ORDER BY cnt DESC;
```

### 3) 请求级与尝试级总量关系

- 请求级（`aggregation=request`）总量应小于或等于尝试级（`aggregation=attempt`）总量。
- 失败后成功场景应在请求级统计为一次请求。

## 回滚方案

1. 查询端回滚为 `aggregation=attempt` 默认口径。
2. 保留新字段，不做 destructive migration。
3. 如需临时关闭解析扩展，可回退到旧解析分支（保留 `usage_source/usage_parser` 字段）。


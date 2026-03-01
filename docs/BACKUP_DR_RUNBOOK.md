# 备份与灾备演练 Runbook

> 返回文档入口：[README.md](./README.md)

## 目标

- 验证备份链路可用（触发、上传、重试、保留）。
- 验证恢复链路可用（dry-run、执行、健康检查）。
- 记录一次演练的 RTO/RPO 与改进项。

## 演练前检查

1. `BackupEnabled=true`，且 `BackupWebDAVURL`、加密口令配置完整。
2. `GET /api/backup/status` 中 `preflight.ready=true`。
3. 已确认维护窗口，并完成风险告知。

## 演练步骤

### 1) 触发一份新备份

```bash
curl -X POST "http://localhost:3000/api/backup/trigger?trigger=drill" \
  -H "Content-Type: application/json" \
  -b "<root-session-cookie>"
```

### 2) 检查备份结果

- `GET /api/backup/runs?limit=5`：确认新运行已完成。
- `GET /api/backup/retries?limit=20`：确认无异常堆积。

### 3) dry-run 恢复校验

```bash
curl -X POST "http://localhost:3000/api/backup/restore/validate" \
  -H "Content-Type: application/json" \
  -d '{"local_path":"/abs/path/to/backup.zip.enc","dry_run":true}' \
  -b "<root-session-cookie>"
```

### 4) 执行恢复（维护窗口）

```bash
curl -X POST "http://localhost:3000/api/backup/restore" \
  -H "Content-Type: application/json" \
  -d '{"local_path":"/abs/path/to/backup.zip.enc","dry_run":false,"confirm":true}' \
  -b "<root-session-cookie>"
```

### 5) 恢复后验证

1. 后台可登录，用户/供应商列表可读。
2. 路由列表可读，关键模型可转发。
3. 日志中无持续 restore/backup 错误。

## 演练记录模板

- 演练时间：
- 演练环境：
- 使用备份文件：
- dry-run 结果：通过 / 失败
- 执行恢复结果：通过 / 失败
- RTO（分钟）：
- RPO（分钟）：
- 发现问题与后续行动：

## 相关文档

- 运维手册：[OPERATIONS.md](./OPERATIONS.md)
- 配置说明：[CONFIGURATION.md](./CONFIGURATION.md)
- API 参考：[API_REFERENCE.md](./API_REFERENCE.md)

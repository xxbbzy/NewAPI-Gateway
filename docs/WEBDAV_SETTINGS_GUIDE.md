# WebDAV 设置与恢复指南

> 返回文档入口：[README.md](./README.md)

## 文档导航

- 上游配置： [CONFIGURATION.md](./CONFIGURATION.md)
- 运维流程： [OPERATIONS.md](./OPERATIONS.md)
- 灾备演练： [BACKUP_DR_RUNBOOK.md](./BACKUP_DR_RUNBOOK.md)

## 1. 适用范围

本文用于指导管理员在前端完成 WebDAV（你也可以理解为 webdev）备份与恢复配置，目标是：

1. 只填写同一个 WebDAV 地址。
2. 备份与恢复共用该地址，减少重复配置。
3. 在恢复前通过 dry-run 校验，避免误操作。

## 2. 一句话原则

- 你只需要在系统设置里配置一个 `WebDAV URL`。
- 系统会把它同时用于备份上传和恢复候选定位。
- 恢复时优先使用最近一次成功备份；找不到候选时再走手动路径回退。

## 3. 前置条件

开始前请确认：

1. 你有 `Root` 管理权限。
2. WebDAV 服务已可访问（推荐 `https://`）。
3. WebDAV 目录具备读写权限。
4. 若启用加密，已准备备份口令并妥善保管。

## 4. 前端配置步骤（推荐）

路径：`系统设置 -> 备份基础配置（WebDAV）`

### 4.1 基础必填

1. 打开 `启用备份功能`。
2. 填写 `WebDAV URL`（例如 `https://dav.example.com/backup`）。
3. 按需填写用户名和密码。
4. 按需开启 `启用备份加密` 并设置口令。
5. 点击 `保存备份设置`。

### 4.2 验证配置是否生效

1. 点击 `刷新状态`。
2. 检查 `预检状态` 是否为 `就绪`。
3. 点击 `立即备份` 触发一次手动备份。
4. 再次刷新，确认“最近运行状态/时间”已更新。

## 5. 恢复流程（高风险）

路径：`系统设置 -> 恢复中心（高风险）`

### 5.1 自动恢复（首选）

1. 页面会自动展示“自动恢复候选”（最新成功备份）。
2. 点击 `先做自动候选 Dry-Run`。
3. dry-run 通过后，在确认框输入 `RESTORE`。
4. 点击 `执行恢复（需确认）`。

### 5.2 手动恢复（回退）

如果没有自动候选：

1. 展开 `高级配置（默认折叠）`。
2. 在 `手动恢复路径（无自动候选时）` 填写本地备份包绝对路径。
3. 再执行 dry-run 与恢复。

## 6. Warning 对照与处理建议

### 6.1 `已启用备份，但未填写 WebDAV URL`

含义：备份已开启，但缺少核心地址，系统无法稳定执行备份/恢复。

处理：填写同一个 WebDAV URL 后再保存。

### 6.2 `WebDAV URL 格式不正确，仅支持 http/https`

含义：地址协议不合法。

处理：改为 `http://` 或 `https://` 开头。

### 6.3 `未找到可用恢复候选`

含义：当前没有成功备份可用于恢复。

处理顺序：

1. 点击 `刷新状态` 同步最新运行记录。
2. 确认上方 WebDAV URL 已配置且可访问。
3. 必要时填写手动恢复路径作为回退。

### 6.4 `请输入 RESTORE 以确认恢复`

含义：恢复属于高风险动作，未完成二次确认。

处理：在确认输入框填写大写 `RESTORE`。

## 7. API 对照（自动化场景）

### 7.1 触发备份

```bash
curl -X POST "http://localhost:3000/api/backup/trigger?trigger=manual" \
  -H "Content-Type: application/json" \
  -b "<root-session-cookie>"
```

### 7.2 恢复 dry-run

```bash
curl -X POST "http://localhost:3000/api/backup/restore/validate" \
  -H "Content-Type: application/json" \
  -d '{"local_path":"/abs/path/to/backup.zip.enc","dry_run":true}' \
  -b "<root-session-cookie>"
```

### 7.3 执行恢复

```bash
curl -X POST "http://localhost:3000/api/backup/restore" \
  -H "Content-Type: application/json" \
  -d '{"local_path":"/abs/path/to/backup.zip.enc","dry_run":false,"confirm":true}' \
  -b "<root-session-cookie>"
```

## 8. 上线前检查清单

1. `BackupEnabled=true` 且 `BackupWebDAVURL` 已配置。
2. `GET /api/backup/status` 中 `preflight.ready=true`。
3. 最近一次备份状态为 `success`。
4. 已完成至少一次 dry-run 恢复校验。

## 相关文档

- 配置说明：[CONFIGURATION.md](./CONFIGURATION.md)
- 运维手册：[OPERATIONS.md](./OPERATIONS.md)
- 备份演练：[BACKUP_DR_RUNBOOK.md](./BACKUP_DR_RUNBOOK.md)
- API 参考：[API_REFERENCE.md](./API_REFERENCE.md)

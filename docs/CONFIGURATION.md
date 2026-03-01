# 配置说明

> 返回文档入口：[README.md](./README.md)

## 文档导航

- 上一篇：[API_REFERENCE.md](./API_REFERENCE.md)
- 下一篇：[DEPLOYMENT.md](./DEPLOYMENT.md)
- 快速入口：[QUICK_START.md](./QUICK_START.md)

## 配置优先级

1. 命令行参数（如 `--port`）
2. 环境变量（如 `PORT`）
3. 代码默认值

说明：`PORT` 为空时，服务端口回退到 `--port`（默认 3000）。

## 环境变量

| 变量 | 作用 | 默认值 | 示例 |
| --- | --- | --- | --- |
| `PORT` | 服务监听端口 | 空（回退 `--port`） | `3000` |
| `GIN_MODE` | Gin 运行模式 | `release`（除 `debug` 外均视为 release） | `debug` |
| `SQL_DRIVER` | 数据库驱动 | 自动检测 | `sqlite` / `mysql` / `postgres` |
| `SQL_DSN` | 数据库连接串 | 空 | `root:pwd@tcp(127.0.0.1:3306)/gateway` |
| `SQLITE_PATH` | SQLite 文件路径 | `gateway-aggregator.db` | `/data/gateway.db` |
| `SESSION_SECRET` | Session 密钥 | 启动时随机生成 | `your-session-secret` |
| `REDIS_CONN_STRING` | Redis 连接串（限流与 Session） | 空（禁用 Redis） | `redis://default:pass@127.0.0.1:6379/0` |
| `UPLOAD_PATH` | 上传目录 | `upload` | `/data/upload` |
| `DEBUG_PROXY_AUTH` | 开启代理认证调试日志 | 关闭 | `1` |

## 命令行参数

| 参数 | 说明 | 默认值 |
| --- | --- | --- |
| `--port` | 监听端口 | `3000` |
| `--log-dir` | 日志目录（自动创建） | 空 |
| `--version` | 打印版本并退出 | - |
| `--help` | 打印帮助并退出 | - |

## 数据库模式

### SQLite（默认）

- 未设置 `SQL_DRIVER` 且未设置 `SQL_DSN` 时自动启用。
- 数据文件默认 `gateway-aggregator.db`。

```bash
SQLITE_PATH=/data/gateway-aggregator.db ./gateway-aggregator
```

### MySQL

```bash
SQL_DRIVER=mysql
SQL_DSN='user:pass@tcp(127.0.0.1:3306)/gateway?charset=utf8mb4&parseTime=True&loc=Local'
```

### PostgreSQL

```bash
SQL_DRIVER=postgres
SQL_DSN='postgres://user:pass@127.0.0.1:5432/gateway?sslmode=disable'
```

如果只设置 `SQL_DSN` 而未设置 `SQL_DRIVER`，系统会自动识别 PostgreSQL DSN；否则按 MySQL 处理。

## Redis 行为

- 设置 `REDIS_CONN_STRING`：
  - Session 使用 Redis 存储。
  - 限流使用 Redis 实现。
- 不设置：
  - Session 使用 Cookie 存储。
  - 限流回退为进程内内存实现。

## 运行时备份配置（`PUT /api/option/`）

以下配置为运行时系统选项，不是环境变量。建议由 `Root` 管理员在后台“系统设置 -> 备份与恢复”中维护。

| Key | 类型 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `BackupEnabled` | bool | `false` | 是否启用备份系统 |
| `BackupTriggerMode` | string | `hybrid` | 触发模式：`hybrid/event/schedule` |
| `BackupScheduleCron` | string | `0 */6 * * *` | 定时兜底备份 cron（5 段） |
| `BackupMinIntervalSeconds` | int | `600` | 两次备份最小间隔 |
| `BackupDebounceSeconds` | int | `30` | 事件触发备份去抖时间 |
| `BackupWebDAVURL` | string | 空 | WebDAV 服务地址（`http/https`） |
| `BackupWebDAVUsername` | string | 空 | WebDAV 用户名 |
| `BackupWebDAVPassword` | string | 空 | WebDAV 密码 |
| `BackupWebDAVBasePath` | string | `/newapi-gateway-backups` | WebDAV 远端目录 |
| `BackupEncryptEnabled` | bool | `true` | 是否启用备份包加密 |
| `BackupEncryptPassphrase` | string | 空 | 备份加密口令（建议至少 8 位） |
| `BackupRetentionDays` | int | `14` | 远端备份按天保留 |
| `BackupRetentionMaxFiles` | int | `100` | 远端备份最大文件数 |
| `BackupSpoolDir` | string | `upload/backup-spool` | 本地备份与重试队列目录 |
| `BackupCommandTimeoutSeconds` | int | `600` | dump/restore 命令超时 |
| `BackupMaxRetries` | int | `8` | 上传失败最大重试次数 |
| `BackupRetryBaseSeconds` | int | `30` | 重试指数退避基准秒数 |
| `BackupMySQLDumpCommand` | string | `mysqldump` | MySQL dump 命令名 |
| `BackupPostgresDumpCommand` | string | `pg_dump` | PostgreSQL dump 命令名 |
| `BackupMySQLRestoreCommand` | string | `mysql` | MySQL restore 命令名 |
| `BackupPostgresRestoreCommand` | string | `psql` | PostgreSQL restore 命令名 |

WebDAV 前端配置与 warning 处理的完整步骤见：
[WEBDAV_SETTINGS_GUIDE.md](./WEBDAV_SETTINGS_GUIDE.md)

## 推荐生产配置

1. 固定设置 `SESSION_SECRET`，避免重启后 Session 失效。
2. 显式设置 `SQL_DRIVER` 与 `SQL_DSN`，避免环境漂移。
3. 使用 Redis 提升多实例场景下的 Session/限流一致性。
4. 保持 `GIN_MODE=release`。
5. 只在排障时短期开启 `DEBUG_PROXY_AUTH=1`。

## 相关文档

- 快速开始：[QUICK_START.md](./QUICK_START.md)
- 部署方案：[DEPLOYMENT.md](./DEPLOYMENT.md)
- 架构说明：[ARCHITECTURE.md](./ARCHITECTURE.md)
- WebDAV 设置教程：[WEBDAV_SETTINGS_GUIDE.md](./WEBDAV_SETTINGS_GUIDE.md)

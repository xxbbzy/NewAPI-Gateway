<p align="right">
   <strong>中文</strong> | <a href="./README.en.md">English</a>
</p>

<div align="center">

# NewAPI Gateway

_✨ 多供应商 NewAPI 聚合网关 — 统一接入、透明代理、使用统计 ✨_

</div>

## 项目简介

NewAPI Gateway 是一个聚合多个 [NewAPI](https://github.com/QuantumNous/new-api) 供应商的透明网关。用户使用单一聚合 Token（`ag-xxx`）即可调用所有已接入供应商的 AI 模型服务，系统自动进行**优先级分层 + 价值评分 + 可选健康调节**的智能路由，上游供应商无法感知网关的存在。

### 核心特性

- ✅ **透明代理**：Header 清洗、body 零改动、UA 透传，上游仅看到"真实客户端"
- ✅ **多供应商管理**：统一管理多个 NewAPI 实例的 Token、定价、余额
- ✅ **智能路由**：候选归一匹配 + Priority 分层 + 价值评分加权 + 可选健康调节
- ✅ **自动同步**：每 5 分钟从上游同步 pricing/tokens/balance，自动重建路由表
- ✅ **签到服务**：自动为启用签到的供应商执行每日签到
- ✅ **SSE 流式支持**：完整支持 Server-Sent Events 流式代理
- ✅ **使用统计**：详细记录每次调用的模型/供应商/耗时/状态
- ✅ **OpenAI 兼容**：支持 OpenAI / Anthropic / Gemini 等多种 API 格式
- ✅ **Web 管理面板**：React 前端，含仪表盘、供应商管理、Token管理、日志查看

> **本网关不涉及充值/计费**，仅做透明代理和使用情况统计。

---

## 系统架构

```
用户客户端
  │ Authorization: Bearer ag-xxx
  ▼
┌─────────────────────────┐
│    NewAPI Gateway │
│  ┌─────┐  ┌──────┐      │
│  │ Auth │→│Router│      │
│  └─────┘  └──┬───┘      │
│              ▼           │
│         ┌────────┐       │
│         │ Proxy  │       │
│         └────┬───┘       │
│              │ Bearer sk-xxx
└──────────────┼───────────┘
        ┌──────┼──────┐
        ▼      ▼      ▼
    NewAPI-A NewAPI-B NewAPI-C
```

---

## 快速开始

### 方式一：DockerHub 预编译镜像（推荐）

```bash
docker pull xxbbzy/newapi-gateway:latest
docker run -d --name newapi-gateway \
  --restart always \
  -p 3000:3000 \
  -v ./data:/data \
  xxbbzy/newapi-gateway:latest
```

### 方式二：预编译二进制启动

1. 从 [Releases](https://github.com/xxbbzy/newapi-gateway) 下载对应系统/架构的二进制文件。
2. 赋予执行权限并启动：

```bash
chmod +x ./gateway-aggregator
./gateway-aggregator --port 3000 --log-dir ./logs
```

### 方式三：源码构建（保留）

> 适用于二次开发。需要 Go 1.18+、Node.js 16+，数据库支持 SQLite（默认）/ MySQL / PostgreSQL。

```bash
# 1. 克隆项目
git clone <repo-url>
cd NewAPI-Gateway-main

# 2. 构建前端
cd web
npm install
npm run build
cd ..

# 3. 构建后端并启动
go mod download
go build -ldflags "-s -w -X 'NewAPI-Gateway/common.Version=$(cat VERSION)'" -o gateway-aggregator
./gateway-aggregator --port 3000 --log-dir ./logs
```

### 首次登录

访问 `http://localhost:3000/` — 初始账号：`root` / `123456`

---

## 使用指南

### 1. 添加供应商

登录后进入 **供应商** 页面，点击"添加供应商"：

| 字段         | 说明                                | 示例                         |
| ------------ | ----------------------------------- | ---------------------------- |
| 名称         | 供应商标识名                        | `Provider-A`                 |
| Base URL     | 上游 NewAPI 地址                    | `https://api.provider-a.com` |
| Access Token | 上游NewAPI提供的系统访问令牌                   | `eyJhbGci...`                |
| 上游 User ID | 上游用户 ID（用于 New-Api-User 头） | `1`                          |
| 权重         | 路由权重（越高越优先）              | `10`                         |
| 优先级       | 路由层级（越高越优先）              | `0`                          |
| 启用签到     | 是否自动签到                        | ☑️                            |

字段获取与填写示例文档：

- [添加供应商字段获取指南](./docs/provider-form-guide.md)
- [添加供应商填写示例](./docs/provider-form-example.md)

### 2. 同步数据

点击供应商列表中的 **同步** 按钮，系统会：

1. 从上游获取模型定价（`GET /api/pricing`）
2. 获取 sk-Token 列表（`GET /api/token/`）
3. 获取用户余额（`GET /api/user/self`）
4. 自动重建模型路由表

### 3. 创建聚合 Token

进入 **令牌** 页面，创建聚合 Token：

- 支持设置过期时间
- 支持模型白名单
- 支持 IP 白名单

创建成功后获得 `ag-xxx` 格式的 Token。

### 4. 调用 API

使用聚合 Token 调用 OpenAI 兼容接口：

```bash
curl https://your-gateway.com/v1/chat/completions \
  -H "Authorization: Bearer ag-xxxxxxxxxxxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

系统自动从所有供应商中选择可用的 `gpt-4o` 路由进行调用。

---

## API 文档

完整接口与认证细节见：[`docs/API_REFERENCE.md`](./docs/API_REFERENCE.md)

### Relay API（OpenAI 兼容，使用 ag-Token 认证）

| Method | Path                              | 说明                  |
| ------ | --------------------------------- | --------------------- |
| POST   | `/v1/chat/completions`            | 对话补全              |
| POST   | `/v1/completions`                 | 文本补全              |
| POST   | `/v1/embeddings`                  | 向量化                |
| POST   | `/v1/images/generations`          | 文生图                |
| POST   | `/v1/audio/speech`                | 文本转语音            |
| POST   | `/v1/audio/transcriptions`        | 语音转文本            |
| POST   | `/v1/messages`                    | Anthropic Claude 兼容 |
| POST   | `/v1/responses`                   | OpenAI Responses API  |
| POST   | `/v1/rerank`                      | 重排序                |
| POST   | `/v1/moderations`                 | 内容审核              |
| POST   | `/v1/video/generations`           | 视频生成              |
| POST   | `/v1beta/models/*`                | Gemini 兼容           |
| GET    | `/v1/models`                      | 查看所有可用模型      |
| GET    | `/v1/models/:model`               | 查看特定模型信息      |
| GET    | `/dashboard/billing/subscription` | 余额查询（兼容）      |
| GET    | `/dashboard/billing/usage`        | 用量查询（兼容）      |

### 管理 API（Session/Token 认证）

#### 供应商管理（Admin）

| Method | Path                        | 说明             |
| ------ | --------------------------- | ---------------- |
| GET    | `/api/provider/`            | 列出所有供应商   |
| POST   | `/api/provider/`            | 创建供应商       |
| PUT    | `/api/provider/`            | 更新供应商       |
| DELETE | `/api/provider/:id`         | 删除供应商       |
| POST   | `/api/provider/:id/sync`    | 触发同步         |
| POST   | `/api/provider/:id/checkin` | 手动签到         |
| POST   | `/api/provider/checkin/run` | 触发全量签到     |
| GET    | `/api/provider/checkin/summary` | 获取签到任务汇总 |
| GET    | `/api/provider/checkin/messages` | 获取签到结果消息 |
| GET    | `/api/provider/checkin/uncheckin` | 获取未签到渠道列表 |
| GET    | `/api/provider/:id/tokens`  | 查看供应商 Token |

#### 聚合 Token 管理（User）

| Method | Path                 | 说明            |
| ------ | -------------------- | --------------- |
| GET    | `/api/agg-token/`    | 我的 Token 列表 |
| POST   | `/api/agg-token/`    | 创建 Token      |
| PUT    | `/api/agg-token/`    | 更新 Token      |
| DELETE | `/api/agg-token/:id` | 删除 Token      |

#### 模型路由管理（Admin）

| Method | Path                 | 说明              |
| ------ | -------------------- | ----------------- |
| GET    | `/api/route/`        | 查看路由表        |
| GET    | `/api/route/models`  | 所有可用模型      |
| PUT    | `/api/route/:id`     | 更新路由权重/状态 |
| POST   | `/api/route/rebuild` | 重建路由表        |

#### 日志与统计

| Method | Path             | 说明                 |
| ------ | ---------------- | -------------------- |
| GET    | `/api/log/self`  | 我的调用日志（User） |
| GET    | `/api/log/`      | 全部日志（Admin）    |
| GET    | `/api/dashboard` | 仪表盘统计（Admin）  |

#### 用户管理 & 系统设置

沿用原始 NewAPI-Gateway 框架的用户管理和系统设置接口，详见 `/api/user/*` 和 `/api/option/*`。

---

## 配置

### 环境变量

| 变量                | 说明                                  | 示例                                   |
| ------------------- | ------------------------------------- | -------------------------------------- |
| `PORT`              | 监听端口                              | `3000`                                 |
| `SQL_DRIVER`        | SQL 驱动（可选）                      | `sqlite` / `mysql` / `postgres`        |
| `SQL_DSN`           | 数据库连接串（MySQL/PostgreSQL 必填） | `root:pwd@tcp(localhost:3306)/gateway` |
| `REDIS_CONN_STRING` | Redis 连接（用于限流和 Session）      | `redis://default:pw@localhost:6379`    |
| `SESSION_SECRET`    | 固定 Session 密钥                     | `random_string`                        |
| `GIN_MODE`          | 运行模式                              | `release` / `debug`                    |

未设置 `SQL_DRIVER` 时，程序会保持兼容旧行为：
- 未设置 `SQL_DSN`：使用 SQLite（`SQLITE_PATH`，默认 `gateway-aggregator.db`）
- 已设置 `SQL_DSN`：自动识别 PostgreSQL（`postgres://`、`postgresql://`、或 `dbname=... user=...`），否则按 MySQL 处理

### 命令行参数

| 参数        | 说明     | 默认值 |
| ----------- | -------- | ------ |
| `--port`    | 服务端口 | `3000` |
| `--log-dir` | 日志目录 | 不保存 |
| `--version` | 打印版本 | -      |

---

## 路由算法

实现入口：`model.BuildRouteAttemptsByPriority`（`model/model_route.go`）与 `controller.Relay`。

1. **候选路由筛选**（仅启用路由）：
   - 精确模型名匹配；
   - 模型名归一化匹配；
   - 版本无关键匹配；
   - 供应商级别名映射（`providers.model_alias_mapping`）匹配。
2. **Priority 分层**：按 `priority` 降序分组，先尝试高优先级层。
3. **计算价值评分 `value_score`**：
   - `unit_cost_usd`：基于 `model_pricings` + token 分组倍率计算；
   - `recent_usage_cost_usd`：统计窗口内成功请求花费（默认 24h，可调）；
   - `value_score = cost_score * budget_score`，其中
     - `cost_score = 1 / (1 + unit_cost_usd)`；
     - `budget_score = (provider_balance + 1) / (provider_balance + recent_usage_cost_usd + 1)`。
4. **计算路由贡献值**：
   - 基础权重：`base = max(weight + 10, 0)`；
   - 当同层存在有效评分时：`contribution_base = base * (RoutingBaseWeightFactor + normalize(value_score) * RoutingValueScoreFactor)`；
   - 当同层评分不可用时，退化为 `contribution_base = base`。
5. **可选健康调节（默认关闭）**：
   - 开启 `RoutingHealthAdjustmentEnabled=true` 后，根据窗口内成功率/失败率/平均延迟计算 `health_multiplier`；
   - 最终贡献值：`contribution = contribution_base * health_multiplier`（并受最小/最大倍率约束）。
6. **同层加权洗牌重试**：
   - 在同一优先级层按 `contribution` 做“加权随机不放回”生成完整重试顺序；
   - 当前层全部失败后才降级到下一优先级层；
   - 遇到不可重试错误（如明确上游失败）会提前终止，不再继续降级。

---

## 隐匿策略

确保上游供应商**感知不到网关的存在**：

| 策略               | 实现方式                                 |
| ------------------ | ---------------------------------------- |
| Authorization 替换 | `ag-xxx` → 上游 `sk-xxx`                 |
| 代理头清除         | 删除 `X-Forwarded-*`、`Via`、`Forwarded` |
| User-Agent 透传    | 保留客户端原始 UA                        |
| Body 零改动        | 请求体原样转发                           |
| 无自定义头         | 不添加任何网关标识 Header                |

---

## 数据模型

| 表名                | 说明                      |
| ------------------- | ------------------------- |
| `users`             | 网关用户                  |
| `options`           | 系统配置（KV）            |
| `providers`         | 上游供应商                |
| `provider_tokens`   | 供应商 sk-Token           |
| `aggregated_tokens` | 用户聚合 Token (ag-xxx)   |
| `model_routes`      | 模型 → 供应商Token 路由表 |
| `model_pricings`    | 上游模型定价缓存          |
| `usage_logs`        | 调用日志                  |

---

## 项目结构

```text
NewAPI-Gateway/
|- .github/workflows/              # CI/CD 与发布流程
|- docs/                           # 项目文档（架构/配置/API/运维）
|- web/                            # React 管理前端
|  |- src/components/              # 页面组件与业务组件
|  |- src/pages/                   # 页面入口
|  |- src/helpers/                 # 前端请求与工具函数
|  `- src/constants/               # 前端常量
|- common/                         # 全局常量、配置、日志、工具、Redis
|- controller/                     # 控制器层（参数处理 + 响应）
|- middleware/                     # 鉴权、限流、CORS、缓存
|- model/                          # 数据模型与查询逻辑
|- router/                         # 路由注册（api / relay / web）
|- service/                        # 业务服务（代理/同步/签到/上游客户端）
|- main.go                         # 程序入口（初始化 DB、Redis、定时任务、路由）
|- go.mod / go.sum                 # Go 依赖
|- Dockerfile                      # Docker 构建
|- Makefile                        # 本地开发命令
`- README.md / README.en.md        # 仓库级说明
```

更细粒度的目录与模块说明见：[`docs/PROJECT_STRUCTURE.md`](./docs/PROJECT_STRUCTURE.md)

---

## 文档中心

- 文档总入口：[`docs/README.md`](./docs/README.md)
- 文档架构规范：[`docs/DOCS_ARCHITECTURE.md`](./docs/DOCS_ARCHITECTURE.md)
- 快速开始：[`docs/QUICK_START.md`](./docs/QUICK_START.md)
- 架构说明：[`docs/ARCHITECTURE.md`](./docs/ARCHITECTURE.md)
- API 参考：[`docs/API_REFERENCE.md`](./docs/API_REFERENCE.md)
- 配置说明：[`docs/CONFIGURATION.md`](./docs/CONFIGURATION.md)
- 部署指南：[`docs/DEPLOYMENT.md`](./docs/DEPLOYMENT.md)
- 运维手册：[`docs/OPERATIONS.md`](./docs/OPERATIONS.md)
- 项目结构：[`docs/PROJECT_STRUCTURE.md`](./docs/PROJECT_STRUCTURE.md)
- 开发指南：[`docs/DEVELOPMENT.md`](./docs/DEVELOPMENT.md)
- 数据模型：[`docs/DATABASE_SCHEMA.md`](./docs/DATABASE_SCHEMA.md)
- 模型别名专题：[`docs/model-alias-manual-mapping.md`](./docs/model-alias-manual-mapping.md)
- 常见问题：[`docs/FAQ.md`](./docs/FAQ.md)
---

## License

MIT License

本项目包含第三方 MIT 许可代码，详见 `THIRD_PARTY_NOTICES.md`。

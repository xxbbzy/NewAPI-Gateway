# 文档中心

> 本页是 `docs/` 的统一入口，先看本页，再按场景进入专题文档。

## 文档信息架构

- `L0 仓库入口`：`README.md` / `README.en.md`（对外介绍与快速入口）
- `L1 主线文档`：`快速开始 -> 架构 -> API -> 配置 -> 部署 -> 运维`
- `L2 研发文档`：`项目结构 -> 开发指南 -> 数据模型`
- `L3 专题文档`：`模型别名手动映射`、FAQ、第三方许可
- 详细规则见：[DOCS_ARCHITECTURE.md](./DOCS_ARCHITECTURE.md)

## 按场景阅读

| 场景 | 推荐阅读顺序 |
| --- | --- |
| 首次部署/试用 | [QUICK_START.md](./QUICK_START.md) -> [CONFIGURATION.md](./CONFIGURATION.md) -> [DEPLOYMENT.md](./DEPLOYMENT.md) |
| 接入调用 API | [ARCHITECTURE.md](./ARCHITECTURE.md) -> [API_REFERENCE.md](./API_REFERENCE.md) -> [FAQ.md](./FAQ.md) |
| 供应商接入填写 | [provider-form-guide.md](./provider-form-guide.md) -> [provider-form-example.md](./provider-form-example.md) -> [OPERATIONS.md](./OPERATIONS.md) |
| 插件对接供应商管理 | [provider-plugin-token-implementation.md](./provider-plugin-token-implementation.md) -> [API_REFERENCE.md](./API_REFERENCE.md) -> [FAQ.md](./FAQ.md) |
| 日常运维值守 | [OPERATIONS.md](./OPERATIONS.md) -> [DATABASE_SCHEMA.md](./DATABASE_SCHEMA.md) -> [FAQ.md](./FAQ.md) |
| 二次开发 | [PROJECT_STRUCTURE.md](./PROJECT_STRUCTURE.md) -> [DEVELOPMENT.md](./DEVELOPMENT.md) -> [API_REFERENCE.md](./API_REFERENCE.md) |

## 完整文档清单

| 分组 | 文档 | 说明 |
| --- | --- | --- |
| 规范 | [DOCS_ARCHITECTURE.md](./DOCS_ARCHITECTURE.md) | 文档分层、命名、联动维护规则 |
| 入口 | [README.md](../README.md) | 仓库中文主页 |
| 入口 | [README.en.md](../README.en.md) | 仓库英文主页 |
| 主线 | [QUICK_START.md](./QUICK_START.md) | 启动与最小可用流程 |
| 主线 | [ARCHITECTURE.md](./ARCHITECTURE.md) | 请求链路、路由策略、同步机制 |
| 主线 | [API_REFERENCE.md](./API_REFERENCE.md) | Relay 与管理 API 清单 |
| 主线 | [CONFIGURATION.md](./CONFIGURATION.md) | 环境变量、参数、配置优先级 |
| 主线 | [DEPLOYMENT.md](./DEPLOYMENT.md) | 二进制、Docker、systemd、Nginx |
| 主线 | [OPERATIONS.md](./OPERATIONS.md) | 巡检、故障、备份恢复 |
| 研发 | [PROJECT_STRUCTURE.md](./PROJECT_STRUCTURE.md) | 目录结构与职责边界 |
| 研发 | [DEVELOPMENT.md](./DEVELOPMENT.md) | 本地开发、调试、贡献流程 |
| 研发 | [DATABASE_SCHEMA.md](./DATABASE_SCHEMA.md) | 核心表结构与数据流 |
| 专题 | [model-alias-manual-mapping.md](./model-alias-manual-mapping.md) | 供应商模型别名手动映射 |
| 专题 | [provider-form-guide.md](./provider-form-guide.md) | 添加供应商字段获取说明（Access Token / Upstream User ID） |
| 专题 | [provider-form-example.md](./provider-form-example.md) | 添加供应商与创建上游令牌填写示例 |
| 专题 | [provider-plugin-token-implementation.md](./provider-plugin-token-implementation.md) | 插件通过个人访问令牌对接供应商管理的改造实施方案 |
| 专题 | [pagination-migration.md](./pagination-migration.md) | 管理列表分页协议迁移与兼容策略 |
| 专题 | [FAQ.md](./FAQ.md) | 高频问题与排障捷径 |
| 合规 | [THIRD_PARTY_NOTICES.md](../THIRD_PARTY_NOTICES.md) | 第三方许可声明 |

## 联动维护矩阵

| 变更类型 | 必须同步文档 |
| --- | --- |
| 新增/修改 Relay 或管理接口 | [API_REFERENCE.md](./API_REFERENCE.md), [README.md](../README.md) |
| 管理列表分页协议变更 | [API_REFERENCE.md](./API_REFERENCE.md), [pagination-migration.md](./pagination-migration.md), [DEVELOPMENT.md](./DEVELOPMENT.md) |
| 路由算法、代理行为、同步流程变更 | [ARCHITECTURE.md](./ARCHITECTURE.md), [OPERATIONS.md](./OPERATIONS.md) |
| 环境变量、启动参数、默认值变更 | [CONFIGURATION.md](./CONFIGURATION.md), [DEPLOYMENT.md](./DEPLOYMENT.md) |
| 数据表/字段语义变更 | [DATABASE_SCHEMA.md](./DATABASE_SCHEMA.md), [ARCHITECTURE.md](./ARCHITECTURE.md) |
| 新增模块或目录调整 | [PROJECT_STRUCTURE.md](./PROJECT_STRUCTURE.md), [DEVELOPMENT.md](./DEVELOPMENT.md) |
| 供应商模型映射策略变更 | [model-alias-manual-mapping.md](./model-alias-manual-mapping.md), [ARCHITECTURE.md](./ARCHITECTURE.md) |
| 供应商接入表单字段与交互变更 | [provider-form-guide.md](./provider-form-guide.md), [provider-form-example.md](./provider-form-example.md), [README.md](../README.md) |
| 插件 Token 接口或鉴权策略变更 | [provider-plugin-token-implementation.md](./provider-plugin-token-implementation.md), [API_REFERENCE.md](./API_REFERENCE.md), [FAQ.md](./FAQ.md) |

## 文档维护约定

1. 所有 `docs/*.md` 顶部保留“返回文档入口”链接。
2. 每篇文档底部至少提供 2 个“相关文档”反向链接，避免孤岛文档。
3. 文档新增后必须同时更新本页“完整文档清单”与“联动维护矩阵”。
4. 链接统一使用相对路径，保证仓库离线可读。

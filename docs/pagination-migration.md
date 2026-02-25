# 分页协议迁移说明

> 返回文档入口：[README.md](./README.md)

## 背景

历史列表接口存在两种返回风格：

- 仅返回数组（前端通过条数猜测是否有下一页）；
- 返回 `items + total` 元数据（可确定翻页）。

本次统一为单一分页协议，避免空白页、漏页和模块间行为不一致。

## 统一协议

- 请求参数：`p`（0 基）、`page_size`
- 响应字段：`items`、`p`、`page_size`、`total`、`total_pages`、`has_more`

详见：[API_REFERENCE.md](./API_REFERENCE.md)

## 渐进迁移策略（兼容窗口）

1. 后端列表接口统一输出新分页结构。
2. 前端通过 `normalizePaginatedData` 做统一解析，并在兼容窗口内临时保留数组响应兼容分支。
3. 组件层移除旧的“本地猜下一页/追加缓存页”逻辑，改为后端元数据驱动翻页。

## 兼容窗口结束后的清理

- 当前变更已完成前端数组兼容分支移除，前端仅接受统一分页结构。
- 清理前需确保：联调环境、灰度环境和生产环境均已升级。

## 回滚要点

- 若后端临时回滚到旧接口格式，需要同时回滚前端或恢复兼容分支，否则分页列表将无法按旧数组格式渲染。
- 若前端回滚，后端新字段为新增字段，不影响旧页面读取已有数据。

## 相关文档

- [API_REFERENCE.md](./API_REFERENCE.md)
- [DEVELOPMENT.md](./DEVELOPMENT.md)
- [ARCHITECTURE.md](./ARCHITECTURE.md)

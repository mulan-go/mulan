---
name: mulan-skill
description: Mulan Go 微服务框架总入口与最佳实践。当需要基于 Mulan 生态开发 Go 微服务时使用。涵盖：`mulan`、`mulan-web`、`mulan-rdb`、`mulan-req`、`mulan-rsp`、`mulan-auth`、`mulan-captcha`、`mulan-dbx`、`mulan-heartbeat`、`mulan-log`、`mulan-passwd`、`mulan-vault`、`webx-ez`。
---

# Mulan 微服务框架

Mulan 是一个多模块 monorepo；根路径为 `github.com/mulan-go/mulan`，子模块使用目录名作为模块后缀，并以 `<目录>/vX.Y.Z` 标签独立发布。
本文件作为总入口，只放导航、组合建议和统一约定；具体 API 请直接查看各模块文档。

## 核心原则

1. 优先使用框架已有能力，避免重复封装
2. 遵循分层结构：`Handler -> Biz -> Repo -> DB/Redis`
3. Handler 层统一用 `req` 收参、`rsp` 返回
4. 全链路使用结构化日志，并带上 `requestid`
5. 数据访问统一带 `ctx`

## 核心模块

### `mulan`

应用启动框架，负责：

- 应用元信息 `Meta`
- 命令行入口
- 配置加载
- 环境变量绑定
- 生命周期管理
- 优雅退出

文档：`references/mulan.md`

### `mulan-web`

HTTP 服务封装，基于 `gin`，内置：

- `requestid`
- `zap` 日志中间件
- `recovery`
- `/health`
- `/version`
- 可选 `/metrics`
- 仅回环监听器可选 `/pprof`

文档：`references/mulan-web.md`

### `mulan-rdb`

Redis 封装，提供：

- Redis 配置结构
- 快速创建客户端
- 调试 Hook
- `redis.Nil` 别名

文档：`references/mulan-rdb.md`

### `mulan-req`

请求绑定扩展，适合 Handler 层：

- 统一绑定 path / body / query
- 默认值处理
- 参数校验
- `Bind`
- `ShouldBind`

文档：`references/mulan-req.md`

### `mulan-rsp`

统一响应结构，适合 API 返回：

- 成功响应
- 错误响应
- 列表响应
- 内部错误屏蔽
- 业务错误码

文档：`references/mulan-rsp.md`

## 扩展模块

- `mulan-auth`：认证扩展；提供 API Key、Session、OAuth2、OIDC
  文档：`references/mulan-auth.md`
- `mulan-captcha`：验证码扩展  
  文档：`references/mulan-captcha.md`
- `mulan-dbx`：多驱动数据库扩展  
  文档：`references/mulan-dbx.md`
- `mulan-heartbeat`：心跳检测扩展  
  文档：`references/mulan-heartbeat.md`
- `mulan-log`：日志扩展  
  文档：`references/mulan-log.md`
- `mulan-passwd`：密码工具扩展  
  文档：`references/mulan-passwd.md`
- `mulan-vault`：Vault AppRole 远程配置注入
  文档：`references/mulan-vault.md`
- `webx-ez`：项目模板  
  文档：`references/webx-ez.md`

## 推荐查看顺序

如果你在开发标准 HTTP 微服务，建议按这个顺序看：

1. `references/mulan.md`
2. `references/mulan-web.md`
3. `references/mulan-req.md`
4. `references/mulan-rsp.md`
5. `references/mulan-rdb.md`

如果要接认证、验证码、日志、多数据库，再继续看对应扩展文档。

如果配置由 Vault 托管，再看 `references/mulan-vault.md`；应用层只保留 `vault.Inject(meta.Name)`。

## 常见组合

### HTTP 服务启动

- `mulan`
- `mulan-web`

### Handler 参数处理

- `mulan-req`
- `mulan-rsp`

### Redis 接入

- `mulan-rdb`

### 远程配置注入

- `mulan-vault`
- `mulan`

### 常见完整组合

- `mulan`
- `mulan-web`
- `mulan-req`
- `mulan-rsp`
- `mulan-rdb`

## 分层约定

### Handler

职责：

- 收参
- 校验
- 调用 Biz
- 返回统一响应

建议：

- 优先使用 `req.Bind(...)`
- 返回优先使用 `rsp.S(...)`、`rsp.OK()`、`rsp.E(...)`

### Biz

职责：

- 业务逻辑
- 事务控制
- 错误转换
- 记录业务日志

### Repo

职责：

- 数据访问
- 只处理 CRUD 和查询拼装
- 返回带调用栈的错误，不直接记录业务日志

### DB / Redis

职责：

- 底层存储
- 保持连接与读写能力简单清晰

## 错误处理约定

- Repo 层：返回带栈错误，保留现场，不记录业务日志
- Biz 层：记录日志，转换成业务错误
- Handler 层：统一通过 `rsp.E(err)` 返回

## 命名约定

- 请求 DTO：`{Entity}{Action}Req`
- 响应 DTO：`{Entity}Item`
- Data 层：提供 `From(model)` 和 `To()` 转换方法

## 开发建议

1. 不重复造轮子，先查 Mulan 生态是否已有能力
2. HTTP 服务统一通过 `web.New(...)` 构建
3. Redis 客户端统一通过 `rdb.New(...)` 创建
4. Vault 业务逻辑留在 `mulan-vault`，应用启动层只调用 `vault.Inject(meta.Name)`
5. 保持 Handler 薄、Biz 厚、Repo 专注数据访问
6. 示例代码以对应模块文档为准，本页不重复展开

## 快速跳转

- 启动应用：`references/mulan.md`
- 注册 HTTP 路由：`references/mulan-web.md`
- 绑定请求参数：`references/mulan-req.md`
- 返回统一响应：`references/mulan-rsp.md`
- 创建 Redis 客户端：`references/mulan-rdb.md`
- 注入 Vault 远程配置：`references/mulan-vault.md`
- 采用当前项目模板：`references/webx-ez.md`

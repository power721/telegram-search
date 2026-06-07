# Teldrive Reference Notes

本文记录对 `/home/harold/workspace/teldrive` 的调查结论，重点关注 `tg-provider` 可以借鉴的工程实践。结论偏向当前项目定位：本地运行、SQLite + FTS5、面向 AList-TVBox 的轻量 Telegram 搜索/链接 provider。

## Summary

`teldrive` 是完整的 Telegram-backed drive 应用，包含认证、文件管理、缓存、事件、HTTP range、UI、Rclone 兼容、PostgreSQL 迁移和性能测试。对 `tg-provider` 最有参考价值的不是文件盘业务，而是 Telegram 客户端运行时、限速重试、配置扩展、事件通知和测试思路。

## High-value Ideas

### 1. gotd 客户端中间件化

参考文件：

- `/home/harold/workspace/teldrive/internal/tgc/tgc.go`
- 当前对照：`internal/telegram/gotd_client.go`

`teldrive` 将 Telegram 客户端创建集中在 `newClient`，并通过 middleware 组合 flood wait、recovery、retry 和 rate limit。当前 `GotdClient` 每次操作都新建短生命周期 client，简单但长期同步时对断线恢复、FloodWait 和限速的控制较弱。

可借鉴能力：

- `floodwait.NewSimpleWaiter()`
- gotd rate limit middleware
- 指数 backoff reconnect
- Telegram proxy dialer
- 可配置 Telegram device metadata
- Telegram 内部日志开关

建议先在当前配置中增加：

- `telegram.proxy`
- `telegram.rate_limit`
- `telegram.rate_burst`
- `telegram.rate_per_minute`
- `telegram.reconnect_timeout`
- `telegram.enable_logging`

### 2. 按账号复用客户端的 ClientPool

参考文件：

- `/home/harold/workspace/teldrive/internal/tgc/client_pool.go`
- 当前对照：`internal/account/manager.go`、`internal/update/*`、`internal/history/*`

`teldrive` 的 `ClientPool` 有几个实用设计：

- 按 `userID/token` 复用 Telegram client。
- 使用 per-key lock 避免并发重复创建同一个 client。
- 使用 ready channel 等待认证完成。
- client context 结束后自动从 pool 删除。
- `Close()` 统一取消所有运行中的 client。

`tg-provider` 可以做轻量版：按 `account_id` 复用用户 client，给历史同步和 update runtime 共用，减少频繁 `client.Run` 的开销。

### 3. session storage 抽象

参考文件：

- `/home/harold/workspace/teldrive/internal/tgstorage/storage.go`
- `/home/harold/workspace/teldrive/internal/tgstorage/bolt.go`
- `/home/harold/workspace/teldrive/internal/tgstorage/postgres.go`

`teldrive` 将 gotd session storage 抽象为：

- `LoadSession`
- `StoreSession`
- `Type`
- `Close`

并支持 memory、bolt、postgres。当前项目不适合引入 postgres，但可以借鉴“可关闭 session storage”的接口设计。后续如果希望避免每账号一个 session 文件，可以实现 SQLite-backed session storage。

### 4. 配置扩展方向

参考文件：

- `/home/harold/workspace/teldrive/internal/config/config.go`
- 当前对照：`internal/config/config.go`

`teldrive` 使用 koanf/cobra/validator 做默认值、配置文件、环境变量和 flags 的分层加载。当前项目的 YAML loader 更简单，符合本地 provider 的部署方式，不建议整体替换。

适合借鉴的是配置字段，而不是配置框架：

- Telegram proxy
- Telegram rate limit
- Telegram reconnect timeout
- HTTP read/write timeout
- 日志级别和 Telegram 内部日志开关
- 后台任务 interval 和 worker/buffer 配置

### 5. 事件广播和 SSE

参考文件：

- `/home/harold/workspace/teldrive/internal/events/broadcaster.go`
- `/home/harold/workspace/teldrive/pkg/services/api.go`

`teldrive` 的事件接口是：

- `Subscribe(userID)`
- `Unsubscribe(userID, ch)`
- `Record(eventType, userID, source)`
- `Shutdown()`

它还支持 SSE keepalive 和事件流。`tg-provider` 可以先做内存版，不需要 Redis 或 DB worker。适合用于：

- 手动同步进度
- retry job 状态
- 账号在线/重连/断开状态
- 新消息或链接入库通知

### 6. 性能测试矩阵

参考文件：

- `/home/harold/workspace/teldrive/tests/performance/perf_test.go`
- 当前对照：`internal/repository/search_benchmark_test.go`

`teldrive` 的性能测试覆盖多种真实查询场景，而不是只测单个查询。`tg-provider` 可以扩展 benchmark 矩阵：

- FTS 搜索不同关键词分布
- 按频道过滤搜索
- 最新消息分页
- 链接类型过滤
- merged links 聚合
- 大批量 soft-delete 后的 FTS 查询

## Cautious Ideas

### OpenAPI/ogen

`teldrive` 使用 OpenAPI/ogen 生成 API，并对少数流式接口做扩展。当前 `tg-provider` 使用 gin 手写路由，API 面较小，维护成本更低。除非 API 面明显扩大，不建议切换。

### Redis/cache

`teldrive` 支持 Redis 和 memory cache。当前项目是本地 SQLite provider，引入 Redis 会提高部署复杂度。可以保留内存缓存思路，用于账号状态、短期 code hash 或 channel metadata，但不引入 Redis。

### PostgreSQL/GORM/goose 迁移

`teldrive` 的数据库体系面向 PostgreSQL 和 GORM。当前项目依赖 SQLite、FTS5 和手写 SQL，更适合保持轻量。可以借鉴“嵌入 SQL migrations + 可重复运行 + 测试覆盖”的严谨性，不建议迁移 ORM。

### HTTP range 和文件流

`teldrive` 的 `internal/http_range`、reader、streaming 主要服务文件盘下载。当前搜索 provider 暂时没有直接代理 Telegram 文件下载的需求，优先级低。

## Do Not Copy Directly

- 文件、上传、分享、Rclone 兼容模型。
- bot selector、自动建频道、频道容量管理。
- PostgreSQL schema 和 GORM model。
- 默认 Telegram app id/hash、WebK device 配置。
- UI、文件管理、上传下载业务流。

这些属于 `teldrive` 的业务域，和当前 AList-TVBox 搜索/链接 provider 的目标不一致。

## Suggested Roadmap

1. 给 `GotdClient` 增加 gotd middleware、proxy、rate limit、reconnect backoff。
2. 实现轻量 `ClientPool`，按 `account_id` 复用长期运行 client。
3. 抽象 session storage，保留文件 session，同时为 SQLite-backed session storage 留接口。
4. 增加同步进度事件或 `/api/jobs/{id}` 查询接口。
5. 扩展大数据 benchmark，覆盖 FTS 搜索、链接聚合和分页场景。

## Implementation Notes

- 保持当前项目轻量：不要因为借鉴 `teldrive` 而引入 Redis、PostgreSQL、GORM 或完整 OpenAPI 生成链。
- 优先解决 Telegram 运行时可靠性：FloodWait、断线重连、限速和 client 复用。
- 新增配置项时继续保持默认 localhost、本地文件和 SQLite 的部署模型。
- 所有借鉴都应按当前包边界落地：`internal/telegram`、`internal/account`、`internal/update`、`internal/history`、`internal/scheduler`。

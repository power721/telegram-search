# 追更助手插件参考分析

来源文件：

`/home/harold/Downloads/Telegram Desktop/【插件】追更助手.js`

该文件是一个追更系统的 T4 播放源和管理 API，约 9386 行。它不是单纯的插件脚本，而是把追更清单、资源搜索、网盘解析、TMDB 同步、Emby 导出、播放代理、通知和语音控制等能力揉在一起的业务编排层。

结论：代码不适合直接搬进当前 Go 项目，但有不少业务模型和接口形态值得参考。

## 可借鉴点

### 统一资源模型

它将不同来源的资源统一成接近下面的结构：

- `url`
- `password`
- `note`
- `source`
- `datetime`
- `cloudType`
- `sourceKind`
- `exportEnabled`

当前项目已有 `telegram_links` 和搜索结果模型，可以参考这个语义补强结果元数据，尤其是 `note`、`sourceKind`、`cloudType`。这对后续接入 AList-TVBox、PanSou 或资源导出很有用。

### 多来源搜索编排

核心函数 `searchTrackingSources` 同时聚合：

- 盘搜结果
- Telegram 频道搜索结果
- Telegram 已入库结果
- 玩偶聚合结果

它统一处理超时、过滤、排序、链接校验和搜索报表。当前 `tg-provider` 主要关注用户自己的 Telegram 私有索引，但可以借鉴这种响应结构，为后续聚合外部搜索源留出扩展点。

### 网盘类型归一化

文件中维护了多组网盘映射，例如：

- `DRIVE_LABELS`
- `CLOUD_TO_DRIVE`
- `PAN_TYPE_TO_DRIVE`
- `DRIVE_ORDER`

当前 `internal/link/extractor.go` 已经能提取主流网盘链接，但可以进一步借鉴别名归一化、展示名称和默认排序偏好，让 API 输出更稳定。

### 搜索结果排序策略

它不只按时间排序，还综合考虑：

- 标题命中度
- 黑白名单
- 网盘偏好
- 清晰度
- 资源发布时间
- 剧集覆盖度
- 是否计划导出

当前 `internal/search/service.go` 仍比较薄，后续可以拆出独立 ranking 层，用于统一搜索、最新链接、TVBox 输出等场景。

### 影视追更 Watchlist

它围绕以下字段管理追更：

- `tmdbId`
- `mediaType`
- `trackingSeason`
- `watchedEpisode`
- `nextAirDate`
- `lastAirDate`
- `status`

当前项目已有 watch rule，但偏关键词监听。如果要做影视追更，应增加“影视条目级追更”模型，而不是只依赖关键词规则。

### 自动入库状态机

`refreshTrackingResources` 实现了：

- running 状态
- 并发限制
- `onlyMissing`
- `onlyUpdating`
- 手动触发
- 状态查询
- 入库摘要

这可以借鉴到当前 scheduler/retry queue 体系中，做成可观测的后台任务。

### T4 输出形态

它完整实现了 TVBox T4 的：

- 首页分类
- 搜索
- 分类筛选
- 详情页
- 播放入口
- 自定义 ID 协议

常见 ID 协议包括：

- `track://`
- `trackdrive://`
- `tracksmart://`
- `link://`

如果当前项目最终要服务 AList-TVBox，这部分接口形态很有参考价值。

### 防误删和失效巡检

它在资源入库和读取链路里实现了多种保护：

- sparse protect：避免一次搜索结果过少时覆盖掉已有资源
- 链接有效性检查
- 轻量 pancheck 缓存
- inflight 去重
- 失效资源降权或剔除
- 播放候选失败冷却

这些是生产环境里很实用的保护机制，适合按需移植设计思想。

## 不建议照搬

- 文件是大单体，业务、API、缓存、导出、播放、通知混在一起。
- 强依赖原运行环境，例如 `/app` 路径、Fastify server、T4 插件体系和 `../API/*` 模块。
- TMDB、Emby、小爱 TTS、玩偶聚合等能力超出当前 `tg-provider` 核心边界。
- JavaScript 实现大量使用全局状态和内存 Map，不适合直接映射到当前 Go 服务。

## 推荐迁移顺序

1. 补强网盘链接归一化和搜索结果元数据。
2. 增加统一的搜索 ranking/report 层。
3. 扩展 watch rule，支持影视条目级 watchlist。
4. 增加后台自动入库任务状态和查询接口。
5. 在核心服务稳定后，再设计 T4 provider 输出。

## 对当前项目的落地点

优先关注这些现有模块：

- `internal/link/extractor.go`：补充网盘别名、类型归一化、展示标签。
- `internal/search/service.go`：增加结果排序、聚合报表和 sourceKind。
- `internal/repository/link.go`：评估是否需要保存更多资源元数据。
- `internal/scheduler`：承载自动入库、巡检、重试等后台任务。
- `internal/repository/watch_rule.go`：后续演进为影视条目级追更或新增独立 watchlist 仓储。

## 摘要

这个插件最有价值的是业务设计，不是代码复用。它证明了一个可行方向：以 Telegram 私有索引为基础，再叠加资源归一化、搜索排序、追更 watchlist、自动入库和 T4 输出，最终形成面向 AList-TVBox 的个人资源检索和追更服务。

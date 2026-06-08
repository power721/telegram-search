# TG Provider for AList-TVBox
## Product Requirement Document + Technical Design

Version: v1.0

---

# 1. 项目背景

AList-TVBox 已经具备：

- Spring Boot 后端
- AList 集成
- 网盘资源管理
- STRM 生成
- 播放能力

目前缺少：

- Telegram 私密频道资源检索

现有公开方案（PanSou 等）已经覆盖公开频道。

本项目仅关注：

- 用户自己的 Telegram 账号
- 用户自己加入的频道
- 用户自己加入的群组
- 用户自己的收藏消息

目标是建立用户个人 Telegram 资源索引。

---

# 2. 总体架构

AList-TVBox
↓
HTTP API
↓
tg-provider
↓
gotd
↓
Telegram

tg-provider 作为独立 Go 服务运行。

部署于 AList-TVBox 容器内部。

监听：

127.0.0.1:9900

---

# 3. 技术选型

## Go

- Go 1.24+

## Telegram

- gotd/td

禁止：

- TDLib
- Telegram4J
- Telethon

## Web

- Gin

## 数据库

- SQLite
- SQLite FTS5

## 日志

- zap
- lumberjack

## 配置

- yaml

---

# 4. 数据目录

/data/tg-provider

目录结构：

/data/tg-provider
├── telegram.db
├── config.yaml
├── sessions
├── logs
└── backup

---

# 5. 数据库设计

## telegram_accounts

字段：

- id
- phone
- telegram_user_id
- first_name
- last_name
- username
- status
- created_at
- updated_at

## telegram_channels

字段：

- id
- account_id
- telegram_channel_id
- access_hash
- title
- username
- type
- last_message_id
- last_sync_time
- created_at

type:

- channel
- supergroup
- saved_messages

## telegram_messages

字段：

- id
- account_id
- channel_id
- telegram_message_id
- sender_id
- text
- raw_json
- date
- edit_date
- deleted
- created_at

## telegram_links

字段：

- id
- message_id
- type
- url
- password
- created_at

## FTS表

telegram_messages_fts

索引字段：

- text

---

# 6. 模块设计

## AccountManager

职责：

- 管理账号生命周期
- 自动恢复 Session
- 自动重连
- 状态管理

接口：

Start()

Stop()

Restart()

List()

---

## SessionManager

职责：

- Session 持久化
- Session 加载
- Session 更新

目录：

/data/tg-provider/sessions

---

## ChannelSyncService

职责：

- 获取频道列表
- 获取群组列表
- 获取 Saved Messages

同步频道元数据。

---

## HistorySyncService

职责：

首次同步历史消息。

支持：

- 断点续传
- 增量同步
- 自动恢复

---

## UpdateListener

职责：

监听：

- 新消息
- 编辑消息
- 删除消息

使用 gotd Updates Engine。

---

## LinkExtractor

识别：

- 115
- 123
- 阿里云盘
- 夸克
- UC
- 百度网盘
- 天翼云盘
- 移动云盘
- PikPak
- 迅雷
- 磁力
- ED2K

自动提取：

- 名称
- 海报
- 简介
- 分享链接
- 提取码

---

## SearchService

基于 SQLite FTS5。

支持：

关键词搜索

频道过滤

账号过滤

时间过滤

网盘过滤

---

# 7. API设计

Base URL:

http://127.0.0.1:9900

## 登录

POST /api/login/send-code

POST /api/login/sign-in

POST /api/login/password

## 账号

GET /api/accounts

DELETE /api/accounts/{id}

## 频道

GET /api/channels

GET /api/channels/{id}

POST /api/channels/{id}/sync

## 搜索

GET /api/search?q=keyword

## 最新消息

GET /api/messages/latest

## 网盘资源

GET /api/links

## 状态

GET /api/status

---

# 8. 配置文件

config.yaml

telegram:

  api_id:

  api_hash:

server:

  host: 127.0.0.1

  port: 9900

sync:

  workers: 5

  history_batch_size: 100

storage:

  path: /data/tg-provider

---

# 9. FloodWait策略

必须实现。

要求：

自动识别 FloodWait。

自动等待。

指数退避。

日志记录。

不得导致服务崩溃。

---

# 10. 日志规范

日志文件：

/data/tg-provider/logs

分类：

- app.log
- sync.log
- telegram.log
- error.log

日志轮转：

lumberjack

---

# 11. AList-TVBox集成

Spring Boot 不直接访问 SQLite。

统一调用：

http://127.0.0.1:9900

例如：

GET /api/search?q=庆余年

返回：

- 消息内容
- 来源频道
- 时间
- 网盘链接

---

# 12. 开发阶段

## Phase 1 MVP

目标：

建立最小可运行版本。

实现：

- SQLite
- 登录
- Session
- 获取频道
- 历史同步
- 搜索
- REST API

验收：

能够搜索频道历史消息。

预计：

1-2周

---

## Phase 2 实时同步

实现：

- Updates Engine
- 新消息监听
- 编辑监听
- 删除监听
- 自动恢复

验收：

消息实时写入数据库。

预计：

1周

---

## Phase 3 多账号

实现：

- 多账号
- AccountManager
- 自动重连
- 状态监控

验收：

同时运行多个账号。

预计：

1周

---

## Phase 4 网盘解析

实现：

- LinkExtractor
- links表
- 网盘分类

验收：

自动识别网盘资源。

预计：

3-5天

---

## Phase 5 性能优化

实现：

- FTS5优化
- WAL模式
- 批量写入
- 索引优化

目标：

100万消息

搜索小于200ms

预计：

1周

---

## Phase 6 AList-TVBox集成

实现：

- Spring Boot调用
- Provider封装
- 搜索聚合

预计：

3天

---

# 13. 非目标

当前阶段不实现：

- 文件下载
- 在线播放
- STRM生成
- Web管理后台
- 独立Vue前端
- 媒体转码
- TMDB刮削
- 海报生成

---

# 14. 生产要求

要求：

- 长期稳定运行
- 低内存占用
- 自动恢复
- 消息零丢失
- 搜索快速响应
- Docker友好
- 支持NAS部署

最终目标：

成为 AList-TVBox 内置 Telegram Resource Provider。

# tg-search 管理 API 文档

本文描述 tg-search 管理端 REST API。公开搜索 API 的兼容响应、API Key 用法和外部接入示例见 [public-api.md](public-api.md)。

## 基本约定

默认地址：

```text
http://127.0.0.1:9900
```

除公开 API 和媒体代理外，管理接口均返回 JSON。错误响应：

```json
{
  "error": {
    "code": "bad_request",
    "message": "message"
  }
}
```

分页参数通用约定：

| 参数 | 说明 |
| --- | --- |
| `limit` | 返回数量。未传时不同接口使用默认值，负数会报错。 |
| `offset` | 偏移量，负数会报错。 |
| `q` / `keyword` | 关键词。 |
| `account_id` | 账号 ID。 |
| `channel_id` | 频道 ID。 |
| `date_from` / `date_to` | 日期过滤，格式为 `YYYY-MM-DD`。 |
| `sort` / `order` | 排序字段和方向，按具体接口支持范围生效。 |

## 认证

管理 API 使用管理员会话 Cookie：

```text
tg_search_session=<token>
```

登录：

```bash
curl -i -c cookies.txt \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"secret123"}' \
  http://127.0.0.1:9900/api/auth/login
```

使用会话：

```bash
curl -b cookies.txt http://127.0.0.1:9900/api/status
```

API Key 只用于公开 API 和媒体代理，不授予管理权限。

## 公开基础接口

### `GET /api/health`

健康检查。无认证时返回：

```json
{ "service": "ok" }
```

如果请求头带 API Key，会校验 key；校验成功时额外返回 `version`。

### `GET /api/ready`

就绪检查，检查数据库和运行时目录。

```json
{
  "ready": true,
  "checks": {
    "database": "ok",
    "runtime_dirs": "ok"
  }
}
```

## 初始化接口

初始化接口用于首次启动流程。

### `GET /api/setup/status`

返回初始化状态。

```json
{
  "complete": false,
  "admin_configured": true,
  "api_key_configured": true,
  "api_key_step_complete": true,
  "telegram_configured": false,
  "telegram_login_complete": false,
  "listen_rules_configured": false,
  "current_step": "telegram_api"
}
```

`current_step` 可能值：

```text
admin
api_key
telegram_api
telegram_login
listen_rules
channel_selection
complete
```

### `POST /api/setup/admin`

创建第一个管理员。

```json
{
  "username": "admin",
  "password": "secret123"
}
```

响应 `201`：

```json
{
  "id": 1,
  "username": "admin",
  "role": "admin"
}
```

### `POST /api/setup/api-key`

生成或返回初始化默认 API Key，无请求体。

响应 `201`：

```json
{
  "id": 1,
  "name": "default",
  "prefix": "0123abcd",
  "key": "0123abcd...",
  "usage_count": 0,
  "created_at": "2026-06-08T02:00:00Z",
  "updated_at": "2026-06-08T02:00:00Z"
}
```

### `POST /api/setup/telegram-api`

保存 Telegram API 凭据。`app_hash` 不会在响应里回显。

```json
{
  "app_id": 123456,
  "app_hash": "your_app_hash"
}
```

响应：

```json
{
  "configured": true,
  "app_id": 123456,
  "app_hash_set": true
}
```

### `POST /api/setup/listen-rules`

保存全局监听规则。

```json
{
  "includes": ["电影"],
  "excludes": ["广告"],
  "message_types": ["text", "photo", "video", "document"],
  "link_types": ["cloud_drive", "magnet", "ed2k", "http"],
  "ignored_link_patterns": ["t.me"]
}
```

`message_types` 和 `link_types` 不能为空。

### `POST /api/setup/complete`

标记初始化完成，返回最新 setup status。

## 登录和会话

### `POST /api/auth/login`

```json
{
  "username": "admin",
  "password": "secret123"
}
```

响应会设置 `tg_search_session` Cookie，并返回用户对象。

### `POST /api/auth/logout`

清除当前 Cookie。

```json
{ "logged_out": true }
```

### `GET /api/auth/me`

返回当前登录用户。未登录返回 `401`。

## 设置接口

### `GET /api/settings/telegram-api`

返回 Telegram API 设置状态。

```json
{
  "configured": true,
  "app_id": 123456,
  "app_hash_set": true
}
```

### `PUT /api/settings/telegram-api`

更新 Telegram API 凭据。

```json
{
  "app_id": 123456,
  "app_hash": "new_app_hash"
}
```

### `GET /api/settings/telegram-bot`

返回 Telegram Bot 设置状态。响应不会包含 Bot Token。

```json
{
  "enabled": true,
  "configured": true,
  "token_set": true,
  "poll_interval": "3s"
}
```

### `PUT /api/settings/telegram-bot`

保存 Telegram Bot 设置。`token` 为空时沿用已有 Token；启用 Bot 且没有已保存 Token 时会返回 `400`。保存后重启服务生效。

```json
{
  "enabled": true,
  "token": "123456:telegram_bot_token",
  "poll_interval": "3s"
}
```

### `GET /api/settings/api-key`

返回当前启用 API Key。需要管理员会话。

```json
{
  "id": 1,
  "name": "default",
  "prefix": "0123abcd",
  "key": "0123abcd...",
  "usage_count": 5,
  "last_used_at": "2026-06-08T03:00:00Z",
  "created_at": "2026-06-08T02:00:00Z",
  "updated_at": "2026-06-08T02:00:00Z"
}
```

### `POST /api/settings/api-key/regenerate`

生成新的 API Key，并立即禁用旧 key。响应同 `GET /api/settings/api-key`，包含新 key 明文。

### `GET /api/settings/runtime`

返回运行时设置。

```json
{
  "sync": {
    "workers": 5,
    "history_batch_size": 100,
    "telegram_request_interval": "2s"
  },
  "storage": {
    "max_db_size": "10GB",
    "max_media_cache": "20GB"
  },
  "telegram": {
    "proxy": "",
    "reconnect_timeout": "5m",
    "dial_timeout": "10s",
    "rate_limit": {
      "enabled": true,
      "rate_per_second": 10,
      "burst": 5
    },
    "stream": {
      "concurrency": 2,
      "buffers": 4,
      "chunk_timeout": "20s"
    },
    "media": {
      "concurrency": 2
    }
  }
}
```

### `PUT /api/settings/runtime`

保存运行时设置。请求体同 `GET /api/settings/runtime`。保存后会立即更新媒体下载并发；其他设置在相关服务读取运行时配置时生效，重启后仍会从数据库加载。

### `PUT /api/settings/admin`

修改管理员用户名或密码。

```json
{
  "username": "admin",
  "current_password": "old-password",
  "new_password": "new-password"
}
```

### `GET /api/settings/version`

返回当前版本。

```json
{
  "current_version": "v1.2.3",
  "update_available": false
}
```

传 `check_update=1` 时会请求 GitHub latest release：

```text
GET /api/settings/version?check_update=1
```

### `GET /api/settings/system-info`

返回系统信息。

```json
{
  "name": "Linux",
  "version": "6.8.0",
  "architecture": "amd64",
  "go_version": "go1.25.0",
  "cpu_count": 4,
  "hostname": "host"
}
```

## Telegram 登录

这些接口需要管理员会话。

### `POST /api/telegram/login/send-code`

发送手机验证码。

```json
{ "phone": "+10000000000" }
```

响应：

```json
{
  "status": "LOGIN_REQUIRED",
  "phone": "+10000000000"
}
```

### `POST /api/telegram/login/sign-in`

提交验证码。

```json
{
  "phone": "+10000000000",
  "code": "12345"
}
```

如果需要两步验证密码，响应 `202`：

```json
{
  "status": "LOGIN_REQUIRED",
  "password_required": true
}
```

成功响应会返回账号和元数据同步结果。

### `POST /api/telegram/login/password`

提交两步验证密码。

```json
{
  "phone": "+10000000000",
  "password": "2fa-password"
}
```

### `POST /api/telegram/login/qr/start`

开始 QR 登录。

```json
{
  "login_id": "abc",
  "status": "pending",
  "qr_url": "tg://login?token=...",
  "expires_at": "2026-06-08T02:00:00Z"
}
```

### `GET /api/telegram/login/qr/:login_id`

轮询 QR 登录状态。成功时返回在线账号响应；未完成时返回新的 `qr_url` 和 `expires_at`。

### `DELETE /api/telegram/login/qr/:login_id`

取消 QR 登录。

```json
{ "canceled": true }
```

## 账号接口

### `GET /api/accounts`

返回所有 Telegram 账号。

```json
{
  "items": [
    {
      "id": 1,
      "phone": "+10000000000",
      "telegram_user_id": 42,
      "first_name": "Ada",
      "last_name": "Lovelace",
      "username": "ada",
      "status": "ONLINE",
      "last_error": "",
      "created_at": "2026-06-08T02:00:00Z",
      "updated_at": "2026-06-08T02:00:00Z"
    }
  ]
}
```

账号状态：

```text
NEW
LOGIN_REQUIRED
SYNCING
ONLINE
RECONNECTING
FLOOD_WAIT
DISCONNECTED
```

### `POST /api/accounts/:id/logout`

退出 Telegram 登录，停止监听并移除本地 session，账号状态变为 `LOGIN_REQUIRED`。

### `DELETE /api/accounts/:id`

删除账号及关联状态。

### `POST /api/accounts/:id/channels/sync-metadata`

同步该账号的频道元数据。可能返回后台任务或直接返回同步结果。

## 频道接口

### `GET /api/channels`

返回频道列表。支持 `account_id` 过滤。

```text
GET /api/channels?account_id=1
```

响应：

```json
{
  "items": [
    {
      "id": 10,
      "account_id": 1,
      "telegram_channel_id": 100,
      "title": "Channel",
      "username": "channel",
      "type": "channel",
      "member_count": 1000,
      "listen_enabled": true,
      "indexed_message_count": 120
    }
  ]
}
```

### `GET /api/channels/:id`

返回单个频道。

### `PATCH /api/channels/:id/control`

更新单个频道控制项。当前有效控制字段是 `listen_enabled`。`history_sync_enabled`、`sync_profile` 为废弃兼容字段，`remote_search_allowed` 为预留字段，调用方不要依赖这些字段产生实际效果。

```json
{
  "listen_enabled": true
}
```

### `PATCH /api/channels/control`

批量更新频道控制项。当前建议只传 `listen_enabled`。

```json
{
  "channel_ids": [10, 11],
  "control": {
    "listen_enabled": true
  }
}
```

### `POST /api/channels/:id/analyze`

返回频道控制项、Watch Rule 和索引统计概览。响应中可能包含废弃或预留控制字段，调用方应以管理界面当前可用操作为准。

### `POST /api/channels/:id/sync`

为单个频道发起历史同步任务。

响应 `202`：

```json
{
  "task_id": 123,
  "job_id": "123",
  "status": "queued",
  "task": {
    "id": 123,
    "type": "history_sync",
    "status": "queued"
  }
}
```

### `POST /api/channels/sync`

批量发起历史同步。

```json
{
  "channel_ids": [10, 11],
  "max_messages": 500
}
```

`max_messages` 可省略。传入时必须为正整数。

### `POST /api/channels/web-access/check`

检测 Telegram Web 访问能力。

```json
{
  "channel_ids": [10, 11]
}
```

响应：

```json
{
  "items": [
    {
      "channel_id": 10,
      "web_access": true,
      "checked_at": "2026-06-08T02:00:00Z",
      "error": ""
    }
  ]
}
```

## 监听规则

### `GET /api/listen-rules`

返回全局监听规则。

### `PUT /api/listen-rules`

更新全局监听规则，请求体同 `POST /api/setup/listen-rules`。

### `GET /api/watch-rules`

返回频道级 Watch Rules。

### `POST /api/watch-rules`

创建频道级 Watch Rule。

```json
{
  "channel_id": 10,
  "enabled": true,
  "includes": ["电影"],
  "excludes": ["广告"],
  "message_types": ["text", "video"],
  "link_types": ["cloud_drive", "magnet"]
}
```

### `GET /api/watch-rules/:id`

返回单个 Watch Rule。

### `PUT /api/watch-rules/:id`

更新 Watch Rule。`enabled` 必填。

### `DELETE /api/watch-rules/:id`

删除 Watch Rule。

```json
{ "deleted": true }
```

## 搜索接口

管理搜索接口只搜索本地已索引内容，远程 Telegram 搜索除外。

### `GET /api/admin/search`

兼容消息搜索接口，返回 `{"items":[]}`。参数：

| 参数 | 说明 |
| --- | --- |
| `q` | 必填，搜索关键词。 |
| `account_id` / `channel_id` | 可选过滤。 |
| `link_type` | 可选链接类型过滤。 |
| `date_from` / `date_to` | 日期过滤。 |
| `before_date` / `before_id` | 游标分页。 |
| `limit` / `offset` | 分页。 |

### `GET /api/admin/search/global`

全局搜索，返回 `messages`、`links`、`files`、`channels` 四组结果。

### `GET /api/admin/search/messages`

搜索消息，返回：

```json
{
  "items": [],
  "total": 0
}
```

### `GET /api/admin/search/links`

搜索链接。额外支持 `link_type` 或 `type`。

### `GET /api/admin/search/files`

搜索文件。额外支持 `file_type` 或 `category`。

### `GET /api/admin/search/channels`

搜索频道标题、用户名和描述。

### `POST /api/admin/search/remote`

执行 Telegram 远程搜索。该能力仍在内部演进中，`remote_search_allowed` 暂时不要作为稳定权限开关依赖。

```json
{
  "channel_id": 10,
  "query": "keyword"
}
```

响应 `202`，返回远程搜索任务。

### `GET /api/admin/search/remote/:task_id`

返回远程搜索结果。远程结果不会写入本地索引。

## 消息和链接接口

### `GET /api/messages/latest`

返回最新消息，支持 `account_id`、`channel_id`、`before_date`、`before_id`、`limit`。

### `GET /api/links`

返回链接列表，支持：

| 参数 | 说明 |
| --- | --- |
| `type` | 链接类型。 |
| `keyword` | 关键词。 |
| `account_id` / `channel_id` | 过滤。 |
| `date_from` / `date_to` | 日期过滤。 |
| `sort` | 排序。 |
| `limit` / `offset` | 分页。 |

### `GET /api/links/grouped`

按链接类型返回数量。

```json
{
  "grouped": {
    "cloud_drive": 10,
    "magnet": 3
  }
}
```

### `GET /api/links/merged`

按链接类型聚合返回资源链接。支持 `q` 或 `keyword`、`type`、账号/频道、日期和分页过滤。

## 资源库接口

### `GET /api/resources`

返回资源库条目。支持参数：

| 参数 | 说明 |
| --- | --- |
| `q` / `keyword` | 关键词。 |
| `type` | 资源类型或文件分类。 |
| `category` | `cloud_drive`、`magnet`、`ed2k`、`http`、`files` 等。 |
| `extension` | 文件扩展名过滤。 |
| `account_id` / `channel_id` | 过滤。 |
| `sort` | 排序。 |
| `limit` / `offset` | 分页。 |

响应：

```json
{
  "items": [
    {
      "id": "link:https://example.com",
      "kind": "link",
      "type": "quark",
      "category": "cloud_drive",
      "url": "https://example.com",
      "password": "abcd",
      "title": "资源标题",
      "datetime": "2026-06-08T02:00:00Z",
      "channel_id": 10,
      "channel_title": "Channel",
      "telegram_message_id": 123
    }
  ],
  "total": 1,
  "grouped": {
    "cloud_drive": 1,
    "magnet": 0,
    "ed2k": 0,
    "http": 0,
    "files": 0
  }
}
```

### `GET /api/resources/grouped`

返回全局资源分类数量。

### `GET /api/resources/:id`

返回单个资源。资源 ID 形如 `link:<url>` 或 `file:<id>`，URL 中的特殊字符需要进行 URL 编码。

### `DELETE /api/resources/:id`

删除单个资源。

### `POST /api/resources/bulk-delete`

批量删除资源。

```json
{
  "ids": ["link:https://example.com", "file:12"]
}
```

响应：

```json
{
  "deleted": 2,
  "missing_ids": []
}
```

## 搜索订阅与通知接口

### `GET /api/saved-searches`

返回保存的搜索订阅。

### `POST /api/saved-searches`

创建搜索订阅。新资源由历史同步或实时监听写入后，会匹配启用的保存搜索并生成通知投递记录。

```json
{
  "name": "哪吒3",
  "keyword": "哪吒3",
  "filters": {
    "category": "cloud_drive",
    "cloud_types": ["quark"]
  },
  "notify_rss": true,
  "notify_webhook": true,
  "notify_telegram": false,
  "enabled": true
}
```

### `GET /api/saved-searches/:id`

返回单个搜索订阅。

### `PUT /api/saved-searches/:id`

更新搜索订阅。

### `DELETE /api/saved-searches/:id`

删除搜索订阅。

### `POST /api/saved-searches/:id/test`

使用当前资源库测试搜索订阅匹配结果。

### `GET /api/webhooks`

返回 Webhook 配置。响应不会返回 `secret`。

### `POST /api/webhooks`

创建 Webhook。

```json
{
  "name": "n8n",
  "url": "https://example.com/hook",
  "events": ["resource.created", "saved_search.matched"],
  "secret": "optional-shared-secret",
  "enabled": true
}
```

投递请求为 `POST`，`Content-Type` 是 `application/json`：

```json
{
  "event_type": "saved_search.matched",
  "delivery_id": 1,
  "created_at": "2026-06-10T12:00:00Z",
  "payload": {
    "saved_search_id": 1,
    "keyword": "哪吒3",
    "resource_title": "哪吒3 4K"
  }
}
```

请求头：

| Header | 说明 |
| --- | --- |
| `X-TG-Search-Event` | 事件类型。 |
| `X-TG-Search-Delivery` | 投递记录 ID。 |
| `X-TG-Search-Signature` | 配置 `secret` 时返回 `sha256=<hmac>`。 |

非 2xx 响应会标记失败并按退避时间重试。达到最大重试次数后保留为 `failed` 且不再调度。

### `GET /api/webhooks/:id`

返回单个 Webhook 配置。

### `PUT /api/webhooks/:id`

更新 Webhook。省略 `secret` 时保留原值。

### `DELETE /api/webhooks/:id`

删除 Webhook。

### `GET /api/notification-deliveries`

返回通知投递记录。支持参数：

| 参数 | 说明 |
| --- | --- |
| `status` | `pending`、`succeeded`、`failed`。 |
| `limit` / `offset` | 分页。 |

## 任务接口

### `GET /api/tasks`

返回任务列表。支持 `status`、`type`、`q`、`sort`、`order`、`limit`、`offset`。

```json
{
  "items": [
    {
      "id": 1,
      "type": "history_sync",
      "status": "running",
      "progress": 50,
      "total": 100,
      "message": "",
      "retry_count": 0
    }
  ],
  "total": 1
}
```

任务状态：

```text
queued
running
succeeded
failed
canceling
canceled
paused
flood_wait
reconnecting
```

### `GET /api/tasks/:id`

返回任务详情。

### `POST /api/tasks/:id/retry`

重试失败任务。

### `POST /api/tasks/:id/cancel`

取消可取消任务。

### `POST /api/tasks/:id/pause`

暂停可暂停任务。

### `POST /api/tasks/:id/resume`

恢复暂停任务。

### `DELETE /api/tasks/:id`

删除可删除任务。

### `POST /api/tasks/bulk-delete`

批量删除任务。

```json
{ "ids": [1, 2, 3] }
```

## 日志接口

### `GET /api/logs`

查询日志。支持：

| 参数 | 说明 |
| --- | --- |
| `file` | 日志文件名。 |
| `level` | 日志级别。 |
| `q` | 文本关键词。 |
| `order` | 排序方向。 |
| `limit` / `offset` | 分页。 |

### `GET /api/logs/:file/download`

下载指定日志文件。

## 状态和事件

### `GET /api/status`

返回服务统计。

```json
{
  "service": "ok",
  "accounts": 1,
  "channels": 10,
  "messages": 1000,
  "links": 200,
  "account_states": {
    "ONLINE": 1
  }
}
```

### `GET /api/storage/usage`

返回存储用量。

```json
{
  "db_bytes": 3200000000,
  "index_bytes": 1100000000,
  "media_cache_bytes": 0,
  "total_bytes": 4300000000,
  "max_db_bytes": 10000000000,
  "max_media_bytes": 20000000000,
  "db_over_quota": false,
  "media_over_quota": false
}
```

### `GET /api/events`

Server-Sent Events。连接成功后先发送注释 `: connected`，随后按事件类型推送：

```text
event: task.updated
data: {"type":"task.updated","payload":{...}}
```

常见事件类型：

```text
task.updated
account.updated
listener.updated
activity.created
```

## 维护接口

### `POST /api/maintenance/sqlite`

执行 SQLite 维护/优化。

```json
{
  "operations": ["PRAGMA optimize"]
}
```

### `POST /api/maintenance/backup`

创建 SQLite 备份。

```json
{
  "path": "/data/tg-search/backup/tg-search-20260608T020000Z.db"
}
```

## 媒体代理

媒体代理既可用管理员 Cookie，也可用 API Key 或签名 URL 访问。

```text
GET  /v/:fileid
HEAD /v/:fileid
GET  /i/:fileid
HEAD /i/:fileid
```

`/v/:fileid` 支持 HTTP Range 请求，并设置 `Accept-Ranges: bytes`。公开搜索 API 返回的媒体 URL 会带 `exp` 和 `sig`，默认 24 小时有效。

`/i/:fileid` 会把图片写入 `storage.path/thumbnails/image-proxy/` 本地缓存。命中缓存时直接返回本地文件；未命中时从 Telegram 下载并原子写入缓存。清理任务每小时运行一次，删除 7 天未访问的图片；当媒体缓存超过 `storage.max_media_cache` 时，按最近访问时间淘汰最旧文件到上限的 90%。`/v/:fileid` 仍按 Range 流式代理，不写入图片缓存。

## 路由速查

```text
GET    /api/health
GET    /api/ready
GET    /api/setup/status
POST   /api/setup/admin
POST   /api/setup/api-key
POST   /api/setup/telegram-api
POST   /api/setup/listen-rules
POST   /api/setup/complete
POST   /api/auth/login
POST   /api/auth/logout
GET    /api/auth/me
GET    /api/settings/telegram-api
PUT    /api/settings/telegram-api
GET    /api/settings/telegram-bot
PUT    /api/settings/telegram-bot
GET    /api/settings/runtime
PUT    /api/settings/runtime
PUT    /api/settings/admin
GET    /api/settings/version
GET    /api/settings/api-key
POST   /api/settings/api-key/regenerate
GET    /api/settings/system-info
POST   /api/telegram/login/send-code
POST   /api/telegram/login/sign-in
POST   /api/telegram/login/password
POST   /api/telegram/login/qr/start
GET    /api/telegram/login/qr/:login_id
DELETE /api/telegram/login/qr/:login_id
GET    /api/accounts
POST   /api/accounts/:id/logout
DELETE /api/accounts/:id
POST   /api/accounts/:id/channels/sync-metadata
GET    /api/channels
GET    /api/channels/:id
PATCH  /api/channels/:id/control
PATCH  /api/channels/control
POST   /api/channels/:id/analyze
POST   /api/channels/:id/sync
POST   /api/channels/sync
POST   /api/channels/web-access/check
GET    /api/listen-rules
PUT    /api/listen-rules
GET    /api/watch-rules
POST   /api/watch-rules
GET    /api/watch-rules/:id
PUT    /api/watch-rules/:id
DELETE /api/watch-rules/:id
GET    /api/admin/search
GET    /api/admin/search/global
GET    /api/admin/search/messages
GET    /api/admin/search/links
GET    /api/admin/search/files
GET    /api/admin/search/channels
POST   /api/admin/search/remote
GET    /api/admin/search/remote/:task_id
GET    /api/messages/latest
GET    /api/links
GET    /api/links/grouped
GET    /api/links/merged
GET    /api/resources
GET    /api/resources/grouped
GET    /api/resources/:id
DELETE /api/resources/:id
POST   /api/resources/bulk-delete
GET    /api/saved-searches
POST   /api/saved-searches
GET    /api/saved-searches/:id
PUT    /api/saved-searches/:id
DELETE /api/saved-searches/:id
POST   /api/saved-searches/:id/test
GET    /api/webhooks
POST   /api/webhooks
GET    /api/webhooks/:id
PUT    /api/webhooks/:id
DELETE /api/webhooks/:id
GET    /api/notification-deliveries
GET    /api/tasks
GET    /api/tasks/:id
POST   /api/tasks/:id/retry
POST   /api/tasks/:id/cancel
POST   /api/tasks/:id/pause
POST   /api/tasks/:id/resume
DELETE /api/tasks/:id
POST   /api/tasks/bulk-delete
GET    /api/logs
GET    /api/logs/:file/download
GET    /api/status
GET    /api/storage/usage
GET    /api/events
POST   /api/maintenance/sqlite
POST   /api/maintenance/backup
GET    /api/search
POST   /api/search
GET    /feeds/latest
GET    /feeds/search
GET    /feeds/saved/:id
GET    /v/:fileid
HEAD   /v/:fileid
GET    /i/:fileid
HEAD   /i/:fileid
```

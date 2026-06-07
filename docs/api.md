# tg-provider API 文档

本文档描述 `tg-provider` 当前 HTTP API。服务默认只监听本机地址，供 AList-TVBox 或同机客户端通过 HTTP 调用。

## 基本信息

- 默认 Base URL: `http://127.0.0.1:6000`
- API 前缀: `/api`
- 数据格式: JSON
- 请求体 `Content-Type`: `application/json`
- 时间格式: RFC3339，例如 `2026-06-07T12:00:00Z`
- 鉴权: 当前 provider API 不做 HTTP 鉴权，必须通过本机监听和容器网络隔离保护，不应公开暴露端口。

## 通用规则

### 成功响应

列表接口统一返回：

```json
{
  "items": []
}
```

同步接口在运行时 retry queue 启用时返回异步任务：

```json
{
  "job_id": "1",
  "status": "queued"
}
```

当前没有公开的 job 查询 API。任务状态仅在进程内存中维护，服务重启后不会保留。

### 错误响应

所有 API 错误使用统一 envelope：

```json
{
  "error": {
    "code": "bad_request",
    "message": "phone is required"
  }
}
```

错误码：

- `bad_request`: 参数、请求体、资源不存在等 4xx 错误。
- `internal_error`: 数据库、Telegram、运行时等 5xx 错误。

敏感信息不会出现在错误响应中，包括 `api_hash`、登录验证码、2FA 密码、session 内容和 Telegram code hash。

### 通用查询参数

部分读接口支持以下参数：

| 参数 | 类型 | 说明 |
| --- | --- | --- |
| `account_id` | positive integer | 按账号过滤。必须大于 0。 |
| `channel_id` | positive integer | 按频道过滤。必须大于 0。 |
| `limit` | non-negative integer | 返回数量。未传时服务使用默认值。 |
| `offset` | non-negative integer | offset 分页。搜索和链接接口支持。 |
| `date_from` | date or RFC3339 | 起始时间，包含边界。支持 `YYYY-MM-DD` 或 RFC3339。 |
| `date_to` | date or RFC3339 | 结束时间，不包含边界；日期格式会按次日 00:00 处理。 |
| `before_date` | RFC3339/date | cursor 分页时间字段，必须和 `before_id` 同时提供。 |
| `before_id` | positive integer | cursor 分页 ID 字段，必须和 `before_date` 同时提供。 |

无效查询参数返回 `400`。

## 状态 API

### GET `/api/status`

返回服务状态和 SQLite 统计信息。

示例：

```bash
curl -s http://127.0.0.1:6000/api/status
```

响应 `200`：

```json
{
  "service": "ok",
  "accounts": 1,
  "channels": 12,
  "messages": 3000,
  "links": 900,
  "account_states": {
    "ONLINE": 1
  }
}
```

字段：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `service` | string | 当前固定为 `ok`。 |
| `accounts` | integer | 账号数量。 |
| `channels` | integer | 频道数量。 |
| `messages` | integer | 消息数量。 |
| `links` | integer | 链接数量。 |
| `account_states` | object | 按账号状态聚合的数量。 |

## 登录 API

### POST `/api/login/send-code`

向 Telegram 发送短信或 App 登录验证码，并创建或更新账号为 `LOGIN_REQUIRED`。

请求体：

```json
{
  "phone": "+123456789"
}
```

示例：

```bash
curl -s -X POST http://127.0.0.1:6000/api/login/send-code \
  -H 'content-type: application/json' \
  -d '{"phone":"+123456789"}'
```

响应 `200`：

```json
{
  "status": "LOGIN_REQUIRED"
}
```

校验：

- `phone` 必填。
- Telegram 返回的 `phone_code_hash` 只保存在内存，不会返回给客户端。

### POST `/api/login/sign-in`

使用验证码完成 Telegram 登录。

请求体：

```json
{
  "phone": "+123456789",
  "code": "12345"
}
```

响应 `200`：

```json
{
  "status": "ONLINE"
}
```

如果账号需要 2FA 密码，响应 `202`：

```json
{
  "status": "LOGIN_REQUIRED",
  "password_required": true
}
```

校验：

- `phone` 必填。
- `code` 必填。
- 必须先调用 `/api/login/send-code`，否则会返回 `400`。

### POST `/api/login/password`

提交 Telegram 2FA 密码。

请求体：

```json
{
  "phone": "+123456789",
  "password": "your-2fa-password"
}
```

响应 `200`：

```json
{
  "status": "ONLINE"
}
```

校验：

- `phone` 必填。
- `password` 必填。
- 密码不会写入日志或响应。

## 账号 API

### GET `/api/accounts`

列出所有 Telegram 账号。

示例：

```bash
curl -s http://127.0.0.1:6000/api/accounts
```

响应 `200`：

```json
{
  "items": [
    {
      "id": 1,
      "phone": "+123456789",
      "telegram_user_id": 10001,
      "first_name": "First",
      "last_name": "Last",
      "username": "telegram_user",
      "status": "ONLINE",
      "created_at": "2026-06-07T12:00:00Z",
      "updated_at": "2026-06-07T12:00:00Z"
    }
  ]
}
```

账号状态：

- `NEW`
- `LOGIN_REQUIRED`
- `SYNCING`
- `ONLINE`
- `RECONNECTING`
- `FLOOD_WAIT`
- `DISCONNECTED`

### DELETE `/api/accounts/{id}`

删除账号。服务会先停止该账号运行时监听，再删除 session 文件和账号记录。

示例：

```bash
curl -s -X DELETE http://127.0.0.1:6000/api/accounts/1
```

响应 `200`：

```json
{
  "deleted": true
}
```

路径参数：

| 参数 | 类型 | 说明 |
| --- | --- | --- |
| `id` | positive integer | 账号 ID。 |

### POST `/api/accounts/{id}/channels/sync`

同步指定账号的 Telegram 对话、频道、超级群和 Saved Messages 虚拟频道。

示例：

```bash
curl -s -X POST http://127.0.0.1:6000/api/accounts/1/channels/sync
```

响应 `202`：

```json
{
  "job_id": "1",
  "status": "queued"
}
```

说明：

- 正常运行时使用异步 retry queue，接口会立即返回。
- 如果内部未启用 queue，响应会包含同步结果 `items`。

## 频道 API

### GET `/api/channels`

列出频道。支持按账号过滤。

查询参数：

| 参数 | 类型 | 说明 |
| --- | --- | --- |
| `account_id` | positive integer | 可选。只返回指定账号的频道。 |

示例：

```bash
curl -s 'http://127.0.0.1:6000/api/channels?account_id=1'
```

响应 `200`：

```json
{
  "items": [
    {
      "id": 1,
      "account_id": 1,
      "telegram_channel_id": 12345,
      "access_hash": 67890,
      "title": "Resource Channel",
      "username": "resource_channel",
      "type": "channel",
      "last_message_id": 100,
      "last_sync_time": "2026-06-07T12:00:00Z",
      "created_at": "2026-06-07T12:00:00Z",
      "updated_at": "2026-06-07T12:00:00Z"
    }
  ]
}
```

频道类型：

- `channel`
- `supergroup`
- `saved_messages`

### GET `/api/channels/{id}`

获取单个频道详情。

示例：

```bash
curl -s http://127.0.0.1:6000/api/channels/1
```

响应 `200`：

```json
{
  "id": 1,
  "account_id": 1,
  "telegram_channel_id": 12345,
  "access_hash": 67890,
  "title": "Resource Channel",
  "username": "resource_channel",
  "type": "channel",
  "last_message_id": 100,
  "last_sync_time": "2026-06-07T12:00:00Z",
  "created_at": "2026-06-07T12:00:00Z",
  "updated_at": "2026-06-07T12:00:00Z"
}
```

### POST `/api/channels/{id}/sync`

同步单个频道历史消息和链接。

示例：

```bash
curl -s -X POST http://127.0.0.1:6000/api/channels/1/sync
```

响应 `202`：

```json
{
  "job_id": "2",
  "status": "queued"
}
```

说明：

- 使用配置 `sync.history_batch_size` 控制 Telegram 批量拉取大小。
- 成功批次会同时提交消息、FTS 索引和提取出的链接。
- 同一频道不会被多个 worker 同时同步。

### POST `/api/channels/sync`

批量同步多个频道。

请求体：

```json
{
  "channel_ids": [1, 2, 3]
}
```

示例：

```bash
curl -s -X POST http://127.0.0.1:6000/api/channels/sync \
  -H 'content-type: application/json' \
  -d '{"channel_ids":[1,2,3]}'
```

响应 `202`：

```json
{
  "job_id": "3",
  "status": "queued"
}
```

校验：

- `channel_ids` 必填。
- `channel_ids` 不能是空数组。
- 每个频道 ID 必须大于 0。

## 监听规则 API

监听规则绑定本地频道 ID。实时监听只使用 `enabled=true` 的规则；手动历史同步只要频道存在规则就应用 `includes`、`excludes` 和“必须包含链接”的过滤，并忽略 `enabled`。

### GET `/api/watch-rules`

返回所有监听规则。

响应 `200`：

```json
{
  "items": [
    {
      "id": 1,
      "channel_id": 1,
      "enabled": true,
      "includes": ["庆余年"],
      "excludes": ["预告"],
      "created_at": "2026-06-07T12:00:00Z",
      "updated_at": "2026-06-07T12:00:00Z"
    }
  ]
}
```

### POST `/api/watch-rules`

创建监听规则。每个频道最多一条规则。

```bash
curl -s -X POST http://127.0.0.1:6000/api/watch-rules \
  -H 'content-type: application/json' \
  -d '{"channel_id":1,"enabled":true,"includes":["庆余年"],"excludes":["预告"]}'
```

响应 `201`：

```json
{
  "id": 1,
  "channel_id": 1,
  "enabled": true,
  "includes": ["庆余年"],
  "excludes": ["预告"],
  "created_at": "2026-06-07T12:00:00Z",
  "updated_at": "2026-06-07T12:00:00Z"
}
```

请求字段：

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `channel_id` | positive integer | 是 | 本地频道 ID。 |
| `enabled` | boolean | 否 | 是否启用实时监听过滤。创建时默认 `true`。 |
| `includes` | string array | 否 | 包含关键词。非空时命中任意一个才保留。 |
| `excludes` | string array | 否 | 排除关键词。命中任意一个就丢弃。 |

校验：

- `channel_id` 必须引用已存在频道。
- `includes` 和 `excludes` 必须是字符串数组。
- 重复创建同一频道规则返回 `409`。

### GET `/api/watch-rules/{id}`

返回单条监听规则。

### PUT `/api/watch-rules/{id}`

更新监听规则。更新请求必须显式传入 `enabled`。

### DELETE `/api/watch-rules/{id}`

删除监听规则。

## 搜索 API

### GET `/api/search`

通过 SQLite FTS5 搜索消息文本。

查询参数：

| 参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `q` | string | 是 | FTS 搜索关键词。 |
| `account_id` | positive integer | 否 | 按账号过滤。 |
| `channel_id` | positive integer | 否 | 按频道过滤。 |
| `link_type` | string | 否 | 只返回包含指定链接类型的消息，例如 `quark`。 |
| `date_from` | date/RFC3339 | 否 | 起始时间。 |
| `date_to` | date/RFC3339 | 否 | 结束时间。 |
| `limit` | non-negative integer | 否 | 返回数量。 |
| `offset` | non-negative integer | 否 | offset 分页。 |
| `before_date` | date/RFC3339 | 否 | cursor 分页时间，必须与 `before_id` 同时提供。 |
| `before_id` | positive integer | 否 | cursor 分页 ID，必须与 `before_date` 同时提供。 |

示例：

```bash
curl -s 'http://127.0.0.1:6000/api/search?q=短剧&account_id=1&link_type=quark&limit=20'
```

响应 `200`：

```json
{
  "items": [
    {
      "id": 10,
      "account_id": 1,
      "channel_id": 1,
      "telegram_message_id": 8001,
      "sender_id": 123,
      "text": "短剧合集 https://pan.quark.cn/s/abc",
      "raw_json": "{}",
      "date": "2026-06-07T12:00:00Z",
      "deleted": false,
      "created_at": "2026-06-07T12:00:01Z",
      "updated_at": "2026-06-07T12:00:01Z",
      "account_phone": "+123456789",
      "account_username": "telegram_user",
      "account_first_name": "First",
      "channel_title": "Resource Channel",
      "channel_username": "resource_channel",
      "links": [
        {
          "id": 5,
          "message_id": 10,
          "type": "quark",
          "url": "https://pan.quark.cn/s/abc",
          "password": "abcd",
          "created_at": "2026-06-07T12:00:01Z"
        }
      ]
    }
  ]
}
```

说明：

- 删除消息会被过滤。
- `links` 总是数组，可能为空。
- `edit_date` 只有消息存在编辑时间时才返回。

## 最新消息 API

### GET `/api/messages/latest`

按消息时间倒序返回最新消息。

查询参数：

| 参数 | 类型 | 说明 |
| --- | --- | --- |
| `account_id` | positive integer | 可选。按账号过滤。 |
| `channel_id` | positive integer | 可选。按频道过滤。 |
| `limit` | non-negative integer | 可选。返回数量。 |
| `before_date` | date/RFC3339 | 可选。cursor 时间，必须与 `before_id` 同时提供。 |
| `before_id` | positive integer | 可选。cursor ID，必须与 `before_date` 同时提供。 |

示例：

```bash
curl -s 'http://127.0.0.1:6000/api/messages/latest?limit=20'
```

响应字段与 `/api/search` 的 `items` 相同。

## 链接 API

### GET `/api/links`

查询已提取的网盘、磁力、ED2K 或普通 URL 链接。

查询参数：

| 参数 | 类型 | 说明 |
| --- | --- | --- |
| `type` | string | 链接类型过滤，例如 `aliyun`、`quark`、`baidu`、`magnet`。 |
| `account_id` | positive integer | 按账号过滤。 |
| `channel_id` | positive integer | 按频道过滤。 |
| `keyword` | string | 按消息文本 `LIKE` 过滤。 |
| `date_from` | date/RFC3339 | 起始时间。 |
| `date_to` | date/RFC3339 | 结束时间。 |
| `limit` | non-negative integer | 返回数量。 |
| `offset` | non-negative integer | offset 分页。 |

示例：

```bash
curl -s 'http://127.0.0.1:6000/api/links?type=quark&keyword=短剧&limit=20'
```

响应 `200`：

```json
{
  "items": [
    {
      "id": 5,
      "message_id": 10,
      "type": "quark",
      "url": "https://pan.quark.cn/s/abc",
      "password": "abcd",
      "created_at": "2026-06-07T12:00:01Z",
      "message_text": "短剧合集 https://pan.quark.cn/s/abc",
      "message_date": "2026-06-07T12:00:00Z",
      "account_id": 1,
      "channel_id": 1,
      "channel_title": "Resource Channel",
      "telegram_message_id": 8001
    }
  ]
}
```

常见链接类型：

- `115`
- `123`
- `aliyun`
- `quark`
- `uc`
- `baidu`
- `tianyi`
- `mobile`
- `pikpak`
- `xunlei`
- `magnet`
- `ed2k`
- `url`

## 维护 API

维护 API 只应本机调用。

### POST `/api/maintenance/sqlite`

执行 SQLite 维护操作。

示例：

```bash
curl -s -X POST http://127.0.0.1:6000/api/maintenance/sqlite
```

响应 `200`：

```json
{
  "operations": [
    "ANALYZE",
    "PRAGMA optimize",
    "telegram_messages_fts optimize"
  ]
}
```

说明：

- 不会在普通请求路径自动运行。
- 不执行 `VACUUM`。

### POST `/api/maintenance/backup`

使用 SQLite `VACUUM INTO` 创建数据库备份文件。

示例：

```bash
curl -s -X POST http://127.0.0.1:6000/api/maintenance/backup
```

响应 `200`：

```json
{
  "path": "/data/tg-provider/backup/telegram-20260607-120000.000000000.db"
}
```

说明：

- 备份目录来自运行时配置，默认是 `/data/tg-provider/backup`。
- 备份文件不应公开下载。
- 外部备份系统应控制保留周期。

## 推荐调用流程

### 首次登录和同步

1. `POST /api/login/send-code`
2. `POST /api/login/sign-in`
3. 如需 2FA，调用 `POST /api/login/password`
4. `GET /api/accounts`
5. `POST /api/accounts/{id}/channels/sync`
6. `GET /api/channels?account_id={id}`
7. `POST /api/channels/{id}/sync`
8. `GET /api/search?q=keyword`
9. `GET /api/links`
10. `GET /api/status`

### AList-TVBox 搜索集成

1. 调用 `/api/search?q=...&limit=...` 获取消息结果。
2. 使用每条结果的 `links` 数组映射到网盘资源。
3. 使用 `channel_title`、`channel_username`、`date` 和 `telegram_message_id` 做展示或追踪。
4. 如果 provider 不可用，AList-TVBox 可以回退到原有搜索逻辑。

## 安全注意事项

- 不要将 `127.0.0.1:6000` 发布到公网。
- 保护 `/data/tg-provider/config.yaml` 和 `/data/tg-provider/sessions` 权限。
- 不要记录登录验证码、2FA 密码、`api_hash` 或 session 内容。
- 删除账号应使用 `DELETE /api/accounts/{id}`，不要只删除 SQLite 记录。
- 备份文件包含账号、频道、消息和链接元数据，移动或上传前需要按敏感数据处理。

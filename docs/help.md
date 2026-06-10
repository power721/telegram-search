# tg-search 帮助文档

本文面向部署和日常使用 tg-search 的管理员，覆盖安装、初始化、管理界面、运维和 API 入口。管理 API 的逐接口说明见 [api.md](api.md)，公开搜索 API 集成说明见 [public-api.md](public-api.md)。

## 1. 产品概览

tg-search 通过个人 Telegram 账号访问频道，把频道消息、链接、文件和媒体元数据写入本地 SQLite，并在本机或服务器上提供：

- Vue 管理界面：首次初始化、账号登录、频道控制、资源搜索、任务和日志管理。
- 本地索引搜索：消息、链接、文件、频道分组搜索。
- Telegram 资源库：把网盘、磁力、电驴、HTTP 链接和文件统一成资源条目。
- 公开搜索 API：给第三方系统用 API Key 查询资源库。
- 媒体代理：通过 `/v/:fileid` 和 `/i/:fileid` 代理 Telegram 视频/图片，支持 Range 请求和签名 URL。

核心数据都存放在本地数据目录中。远程 Telegram 搜索结果只用于展示，不会写入本地消息、链接、文件或 FTS 索引表。

## 2. 安装

### 2.1 Docker 一键安装

```bash
scripts/install-docker.sh
```

脚本会：

- 拉取 `haroldli/tg-search:latest`。
- 删除同名旧容器 `tg-search`。
- 使用 Docker volume `tg-search-data` 保存 `/data/tg-search`。
- 将宿主机端口映射到容器 `9900`。
- 尝试打开管理界面。

常用参数：

```bash
scripts/install-docker.sh --port 19900
scripts/install-docker.sh --no-open
```

### 2.2 Docker 手动安装

```bash
docker run -d \
  --name tg-search \
  --restart unless-stopped \
  -p 9900:9900 \
  -v tg-search-data:/data/tg-search \
  haroldli/tg-search:latest
```

查看状态：

```bash
docker ps --filter name=tg-search
docker logs -f tg-search
curl http://127.0.0.1:9900/api/ready
```

升级：

```bash
docker pull haroldli/tg-search:latest
docker rm -f tg-search
docker run -d \
  --name tg-search \
  --restart unless-stopped \
  -p 9900:9900 \
  -v tg-search-data:/data/tg-search \
  haroldli/tg-search:latest
```

### 2.3 Docker Compose 安装

仓库内的 `compose.yaml` 会拉取并运行远程镜像 `haroldli/tg-search:latest`，不会在本地构建镜像，并把 `./data` 映射为 `/data/tg-search`。

```bash
mkdir -p data
docker compose up -d
docker compose logs -f tg-search
```

升级远程镜像：

```bash
docker compose pull
docker compose up -d
```

停止：

```bash
docker compose down
```

### 2.4 二进制运行

```bash
go build -o /tmp/tg-search ./cmd/tg-search
/tmp/tg-search -config config.yaml
```

如果未指定 `-config`，程序会优先使用 `/data/tg-search/config.yaml`，再使用当前目录 `config.yaml`。如果配置文件不存在，会自动生成默认配置。

## 3. 配置文件

典型生产配置只需要启动期参数：

```yaml
server:
  host: 0.0.0.0
  port: 9900
storage:
  path: /data/tg-search
bot:
  enabled: false
  token: ""
  poll_interval: 3s
```

配置说明：

| 字段 | 说明 |
| --- | --- |
| `server.host` | HTTP 监听地址。Docker 内通常用 `0.0.0.0`，本地开发可用 `127.0.0.1`。 |
| `server.port` | HTTP 监听端口，默认 `9900`。 |
| `storage.path` | 运行时数据目录。 |
| `bot.enabled` | 是否启用 Telegram Bot 集成，默认关闭；数据库设置存在时作为首次默认值。 |
| `bot.token` | Telegram Bot Token；数据库设置存在时作为首次默认值。 |
| `bot.poll_interval` | Bot 轮询命令和投递通知的间隔，默认 `3s`；数据库设置存在时作为首次默认值。 |

以下内容在管理界面的「设置」页面维护并保存到数据库：

| 设置页 | 内容 |
| --- | --- |
| 账号与安全 | 管理员用户名/密码、API Key、Telegram `app_id` 和 `app_hash`。 |
| 存储 | SQLite 数据库预算、媒体缓存预算，低于 `100MB` 会被拒绝。 |
| 运行参数 | 历史同步 worker、批量大小、Telegram 请求间隔、代理、重连/拨号超时、请求限速、视频流式读取和媒体下载并发。 |
| 通知集成 | 搜索订阅、Webhook 事件推送、Telegram Bot Token 和轮询间隔。 |

服务启动时会先加载配置文件，再应用数据库里的运行时设置和 Bot 设置。旧版配置文件中的 `sync.*`、`storage.max_*`、`telegram.*` 和 `bot.*` 字段仍会被读取，可作为首次启动或尚未保存设置页参数时的默认值；新生成的默认配置不再写入运行时字段。

## 4. 数据目录

`storage.path` 下会创建：

| 路径 | 说明 |
| --- | --- |
| `tg-search.db` | SQLite 主数据库。 |
| `sessions/` | Telegram 账号 session。 |
| `logs/` | 应用、同步和 Telegram 日志。 |
| `backup/` | SQLite 备份文件。 |
| `uploads/` | 预留上传目录。 |
| `index/` | 本地索引目录。 |
| `thumbnails/` | 缩略图和媒体缓存。 |

图片代理会把 `/i/:fileid` 下载到 `thumbnails/image-proxy/`。命中缓存时直接读取本地文件，不再请求 Telegram。清理任务每小时检查一次：删除 7 天未访问的图片；当媒体缓存超过 `storage.max_media_cache` 时，按最近访问时间从旧到新淘汰到上限的 90%。视频代理仍按 Range 流式读取，不写入该图片缓存。

不要公开、提交或分享 `sessions/`、`tg-search.db`、日志、备份和配置中的敏感信息。

## 5. 首次初始化

打开管理界面后，系统会根据 `/api/setup/status` 自动跳转到当前步骤。

```text
管理员账号 -> API Key -> Telegram API -> Telegram 登录 -> 监听规则 -> 频道选择
```

### 5.1 管理员账号

创建第一个本地管理员。管理界面和管理 API 使用 `tg_search_session` Cookie 登录，不使用公开 API Key。

### 5.2 API Key

生成默认 API Key。它用于公开搜索 API 和媒体代理，不具备管理权限。完整 key 会在创建/查看设置时返回，后续可以在「设置」里重新生成。

### 5.3 Telegram API

填写 Telegram `app_id` 和 `app_hash`。`app_hash` 是写入型字段，管理 API 只返回是否已设置，不回显明文。

### 5.4 Telegram 登录

支持三种情况：

- 手机验证码登录。
- 两步验证密码。
- QR 登录。

账号登录成功后会保存 Telegram session，并触发频道元数据同步。该步骤只同步频道标题、用户名、成员数、描述、头像状态、同步/监听状态等元数据，不拉取完整历史消息。

### 5.5 监听规则

全局监听规则决定实时监听时哪些消息和链接会进入索引：

- `includes`：命中关键词才保留，留空表示不过滤。
- `excludes`：命中关键词则排除。
- `message_types`：允许的消息类型。
- `link_types`：允许的链接类型。
- `ignored_link_patterns`：忽略的链接模式。

每个频道还可以单独配置 Watch Rule。

### 5.6 频道选择

选择需要索引的频道并执行同步或监听操作：

| 操作/字段 | 说明 |
| --- | --- |
| 单频道同步 / 批量同步 | 拉取历史消息并写入本地索引。 |
| `listen_enabled` | 有效字段，控制是否实时监听新消息。 |
| `history_sync_enabled` | 废弃兼容字段，当前不要作为有效配置使用。 |
| `sync_profile` | 废弃兼容字段，当前不要作为有效配置使用。 |
| `remote_search_allowed` | 预留字段，当前不要作为公开能力开关依赖。 |

历史同步通过频道同步接口或管理界面的同步按钮触发；实时增量通过 `listen_enabled` 控制。

## 6. 管理界面

### 首页

展示服务状态、账号数量、频道数量、消息数、链接数和账号状态分布。

### 搜索

提供全局搜索和分类型搜索：

- 消息：基于本地 FTS5 的消息内容搜索。
- 链接：按链接类型、关键词、频道、时间过滤。
- 文件：按文件名、扩展名、分类过滤。
- 频道：按标题、用户名和描述搜索。

搜索结果中的媒体字段可能包含 `/i/:fileid` 或 `/v/:fileid` 地址。管理员会话访问时通常返回相对路径，公开 API 访问时返回带签名的地址。

### 频道

用于查看频道列表、按账号过滤、执行频道分析、检测 Telegram Web 访问能力、批量更新控制项、单频道或批量历史同步。

Web 访问检测只检查 `https://t.me/s/{username}` 是否可访问，不代表搜索引擎收录情况。

### 资源库

展示从消息中提取的资源条目，包含：

- `cloud_drive`
- `magnet`
- `ed2k`
- `http`
- `files`

支持关键词、分类、类型、扩展名、账号、频道、分页和排序过滤。管理员可以删除单个或批量资源条目。

### 账号

展示 Telegram 账号状态，支持重新同步频道元数据、退出 Telegram 登录和删除账号。退出或删除会停止该账号运行时监听，并移除本地 session。

### 任务

展示持久化任务，支持筛选、分页、查看详情、重试、取消、暂停、恢复、删除和批量删除。任务类型包括：

- `metadata_sync`
- `channel_analysis`
- `web_access_detection`
- `history_sync`
- `listener_recovery`
- `remote_search`
- `backup`
- `gap_recovery`

任务状态包括：

```text
queued -> running -> succeeded
queued -> running -> failed
failed -> queued
running -> canceling -> canceled
running -> paused -> running
running -> flood_wait -> queued
running -> reconnecting -> running
```

服务启动时会恢复未完成任务。未来的 `flood_wait` 保持 `next_run_at`，已过期或未调度的未完成任务回到 `queued`。

### 日志

支持按日志文件、级别、关键词、顺序、分页查看日志，并下载指定日志文件。

### 设置

设置页面包含：

- Telegram API：更新 `app_id` 和 `app_hash`。
- API Key：查看当前 key、重新生成 key。
- 管理员账号：修改用户名或密码。
- 运行时设置：同步、容量、Telegram 代理、限速、流式传输和媒体并发。
- 版本信息：查看当前版本，可选择检查 GitHub 最新 release。
- 系统信息：系统名、内核版本、CPU、Go 版本和主机名。

### API 帮助

管理界面内置公开 API 使用说明，方便复制 API Key 和请求示例。

## 7. 管理 API

管理 API 位于 `/api/*`，大多数接口需要管理员登录后的 `tg_search_session` Cookie。典型调用流程：

```bash
curl -i -c cookies.txt \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"secret123"}' \
  http://127.0.0.1:9900/api/auth/login

curl -b cookies.txt http://127.0.0.1:9900/api/status
```

详见 [管理 API 文档](api.md)。

## 8. 公开 API

公开接口用于外部系统集成，只允许：

- `GET /api/search`
- `POST /api/search`
- `GET /feeds/latest`
- `GET /feeds/search`
- `GET /feeds/saved/:id`
- `GET/HEAD /v/:fileid`
- `GET/HEAD /i/:fileid`

请求头：

```text
X-API-Key: <api-key>
```

示例：

```bash
curl -H "X-API-Key: $TG_SEARCH_API_KEY" \
  "http://127.0.0.1:9900/api/search?kw=电影&res=merge&cloud_types=quark,aliyun&include_image=1"
```

详见 [公开 API 集成文档](public-api.md)。

## 9. Telegram Bot

在管理后台「设置 -> 通知集成」配置并启用 Bot，重启服务后支持命令：

```text
/search <keyword>
/subscribe <keyword>
/unsubscribe <subscription_id>
/subscriptions
```

`/subscribe` 会创建启用 Telegram 通知的保存搜索。后续历史同步或实时监听写入新资源并匹配该保存搜索时，系统会通过 Bot 向订阅 chat 推送消息。

## 10. 运维

### 健康检查

```bash
curl http://127.0.0.1:9900/api/health
curl http://127.0.0.1:9900/api/ready
```

`/api/health` 返回服务是否可响应。`/api/ready` 会检查数据库和运行时目录是否可用。

### 备份

Compose 数据目录备份：

```bash
DATA_DIR=./data scripts/backup.sh
```

也可以在管理 API 中调用：

```bash
curl -b cookies.txt -X POST http://127.0.0.1:9900/api/maintenance/backup
```

### SQLite 优化

```bash
curl -b cookies.txt -X POST http://127.0.0.1:9900/api/maintenance/sqlite
```

### SSE 事件

管理界面通过 `/api/events` 接收实时事件。事件类型包括：

- `task.updated`
- `account.updated`
- `listener.updated`
- `activity.created`

## 10. 常见问题

### 打开页面后一直进入初始化

调用 `/api/setup/status` 查看 `current_step`。常见原因是尚未完成 API Key、Telegram 登录、监听规则或频道选择。

### Telegram 登录成功但没有消息

登录只同步频道元数据。需要在「频道」页面开启历史同步并执行同步，或开启实时监听等待新消息进入索引。

### 公开 API 返回 401

检查是否设置了 `X-API-Key` 或 `Authorization` 请求头。API Key 不支持 query 参数传递。

### 媒体地址无法打开

公开 API 返回的媒体 URL 默认带 `exp` 和 `sig`，有效期为 24 小时。过期后重新搜索获取新 URL。管理员会话也可以直接通过 Cookie 访问媒体地址。

### 历史同步被拒绝

当数据库容量超过设置页里的数据库容量上限时，系统可能拒绝新的高成本写入任务。可在「设置 > 存储」提高容量预算、清理资源或备份后维护数据库。

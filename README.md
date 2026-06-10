# tg-search

tg-search 是一个自托管的 Telegram 资源搜索服务。它使用个人 Telegram 账号读取频道消息，把消息、链接和文件索引到本地 SQLite，并提供网页管理界面、管理 API、公开搜索 API 和可签名的媒体代理地址。

适合的场景：

- 管理自己的 Telegram 频道资源库。
- 在本地搜索历史消息、网盘链接、磁力、电驴链接和视频文件。
- 给外部应用、机器人或站点接入统一的资源搜索 API。
- 用网页界面完成 Telegram 登录、频道同步、监听规则、任务和日志管理。

## 快速安装

推荐使用 Docker。脚本会拉取 `haroldli/tg-search:latest`，创建/复用 `tg-search-data` 数据卷，启动容器并尝试打开管理界面：

```bash
scripts/install-docker.sh
```

指定端口或不自动打开浏览器：

```bash
scripts/install-docker.sh --port 19900 --no-open
```

手动 Docker 运行：

```bash
docker run -d \
  --name tg-search \
  --restart unless-stopped \
  -p 9900:9900 \
  -v tg-search-data:/data/tg-search \
  haroldli/tg-search:latest
```

Docker Compose 运行：

```bash
mkdir -p data
docker compose up -d
```

启动后打开：

```text
http://127.0.0.1:9900
```

检查服务：

```bash
docker logs -f tg-search
curl http://127.0.0.1:9900/api/health
curl http://127.0.0.1:9900/api/ready
```

## 首次初始化

首次打开管理界面后，按向导完成：

```text
管理员账号 -> API Key -> Telegram API -> Telegram 登录 -> 监听规则 -> 频道选择
```

Telegram API 的 `app_id` 和 `app_hash` 需要从 Telegram 官方开发者后台获取。Telegram 登录支持验证码、两步验证密码和 QR 登录。账号上线后系统会先同步频道元数据，不会在登录步骤直接拉取完整历史消息。

完成初始化后，在「频道」页面选择需要管理的频道：

- 使用单频道或批量同步操作拉取历史消息。
- 开启 `listen_enabled` 后实时监听新消息。
- `history_sync_enabled` 和 `sync_profile` 是废弃兼容字段，当前不要作为有效配置使用。
- `remote_search_allowed` 是预留字段，当前不要作为公开能力开关依赖。

## 常用路径

```text
管理界面:       http://127.0.0.1:9900
健康检查:       GET /api/health
就绪检查:       GET /api/ready
管理 API:       /api/*
公开搜索 API:   GET/POST /api/search
视频代理:       GET/HEAD /v/:fileid
图片代理:       GET/HEAD /i/:fileid
```

管理 API 使用登录后的 `tg_search_session` Cookie。公开搜索 API 和媒体代理可使用 API Key：

```text
X-API-Key: <api-key>
```

## 数据目录

容器内默认数据目录为 `/data/tg-search`，包含配置、数据库、Telegram session、日志、备份、索引和缩略图。Compose 默认映射到本仓库的 `./data`，一键安装脚本默认使用 Docker volume `tg-search-data`。

备份和恢复 Compose 数据目录：

```bash
DATA_DIR=./data scripts/backup.sh
docker compose stop tg-search
DATA_DIR=./data scripts/restore.sh ./data/backup/tg-search-YYYYMMDDTHHMMSSZ.db
docker compose up -d
```

## 本地开发

后端：

```bash
go build -o /tmp/tg-search ./cmd/tg-search
/tmp/tg-search -config config.yaml
```

前端开发服务器：

```bash
npm install --prefix web
npm run web:dev
```

开发地址：

```text
Backend:  http://127.0.0.1:9900
Frontend: http://127.0.0.1:5173
```

常用检查：

```bash
GOCACHE=/tmp/go-build-cache go test ./...
npm run web:typecheck
npm run web:test
npm run web:build
```

## 文档

- 完整帮助文档：[docs/help.md](docs/help.md)
- 管理 API 文档：[docs/api.md](docs/api.md)
- 公开 API 集成文档：[docs/public-api.md](docs/public-api.md)
- API 响应约定：[docs/api-response-contract.md](docs/api-response-contract.md)
- 冒烟测试指南：[docs/smoke-test-guide.md](docs/smoke-test-guide.md)
- 生产部署检查清单：[docs/production-deployment-checklist.md](docs/production-deployment-checklist.md)

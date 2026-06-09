# TG Search

## 1. 项目定位

项目名称：`tg-search`

`tg-search` 是一个长期运行的个人 Telegram 搜索服务。

它允许用户登录自己的 Telegram 账号，并基于本地索引搜索自己有权限访问的 Telegram 内容。

核心能力包括：

* 搜索消息
* 搜索私密频道
* 搜索群组
* 搜索 Saved Messages
* 浏览频道历史消息
* 实时监听新消息
* 搜索网盘资源
* 搜索媒体资源

本项目不是：

* PanSou 替代品
* Telegram 公共搜索引擎
* Telegram 爬虫平台

本项目只索引用户自己有权限访问的 Telegram 内容。

---

## 2. 核心目标

构建用户自己的：

* Telegram 搜索引擎
* Telegram 索引库
* Telegram 资源库

支持索引：

* 私密频道
* 私密群组
* Saved Messages
* 已加入的公开频道

核心原则：

* 默认只拉取频道元数据
* 默认不自动同步所有频道历史消息
* 只同步用户明确勾选监听或同步的频道
* 避免首次登录时因频道数量过多导致同步数百万条消息
* 优先保证长期稳定运行和本地搜索体验

---

## 3. 技术栈

### Backend

* Go 1.24+
* Gin
* gotd/td
* GORM

### Frontend

* Vue 3
* TypeScript
* Vite
* Naive UI
* Pinia
* Vue Router
* UnoCSS

### Database

Phase 1：

* SQLite

Phase 2：

* MySQL
* PostgreSQL

### Search

Phase 1：

* SQLite FTS5

Phase 2：

* Bleve

Phase 3：

* ElasticSearch

### Cache

Phase 3：

* Redis

### Vector Search

Phase 4：

* Qdrant
* Milvus

---

## 4. 部署方式

项目独立部署、独立发布。

二进制名称：

```bash
tg-search
```

支持平台：

* linux-amd64
* linux-arm64
* linux-armv7

发布方式：

* GitHub Release
* Docker Hub

必须提供：

* Dockerfile
* Docker Compose

---

## 5. 数据目录

默认数据目录：

```bash
/data
```

目录结构：

```bash
/data
├── config.yaml
├── tg-search.db
├── sessions
├── logs
├── uploads
├── backup
├── index
└── thumbnails
```

---

## 6. 首次启动引导

首次启动进入 Setup Wizard。

### Step 1：创建管理员账号

字段：

* 用户名
* 密码

密码使用 bcrypt 存储。

---

### Step 2：创建 API Key

API Key 可跳过。

格式：

* UUID 去除短横线

API 调用认证方式：

```http
Authorization: <API_KEY>
```

---

### Step 3：配置 Telegram API

字段：

* App ID
* App Hash

允许跳过。

如果跳过，则使用项目内置默认配置。

后续允许在管理后台修改。

---

### Step 4：登录 Telegram

支持：

* 手机号
* 验证码
* 二步验证密码

登录成功后，后台异步拉取：

* Channels
* SuperGroups
* Saved Messages

注意：

首次登录只拉取元数据，不自动同步全部历史消息。

需要保存的元数据包括：

* 标题
* 用户名
* 类型
* 成员数
* 描述
* 基础统计信息

---

### Step 5：频道分析

展示：

* 标题
* 用户名
* 成员数
* 描述
* 图片数量
* 视频数量
* 文件数量
* 链接数量

异步执行：

```text
Channel Discoverability Detection
```

检测内容：

* 是否可 Web 搜索
* 是否已被公开索引

---

### Step 6：监听规则配置

支持规则：

* includes
* excludes

支持消息类型：

* text
* image
* video
* audio
* file
* link

支持链接类型：

* 网盘链接
* Magnet
* ED2K
* HTTP
* 其它

---

### Step 7：选择监听频道

用户选择频道后，可配置：

* 开启监听
* 不监听
* 是否同步历史消息

默认行为：

* 只监听用户勾选的频道
* 只同步用户勾选的频道
* 不自动同步全部频道历史消息

历史同步默认数量：

```text
1000 条
```

可选配置：

```text
100
1000
5000
10000
```

同步完成后：

* 写入数据库
* 提取链接
* 建立全文索引

---

### Step 8：进入首页

---

## 7. 首页

首页显示：

* 搜索框
* 统计卡片
* 同步状态
* 监听状态
* 最近活动

统计卡片包括：

* 账号数量
* 频道数量
* 消息数量
* 文件数量
* 图片数量
* 视频数量
* 网盘链接数量

---

## 8. 频道页

显示字段：

* 标题
* 用户名
* 类型
* 成员数
* 描述

同步状态：

* 已同步
* 同步中
* 未同步

监听状态：

* 已监听
* 未监听

可搜索性：

* 🔒 Private
* 🌐 Public
* ⚠ Partial
* ❓ Unknown

支持操作：

* 手动同步
* 开启监听
* 停止监听
* 修改同步数量
* 查看同步进度

---

## 9. 搜索页

支持关键词搜索。

过滤条件：

* 频道
* 账号
* 时间范围
* 消息类型
* 链接类型
* 文件类型

结果显示：

* 频道名称
* 消息内容
* 时间
* 发送人
* 文件信息
* 网盘链接

搜索能力：

* 全文搜索
* 高亮
* 分页
* 按时间排序
* 按相关性排序

---

## 10. 账号页

支持：

* 添加账号
* 删除账号
* 查看状态
* 重新登录
* 查看 Session 状态

---

## 11. Channel Discoverability Detection

同步频道元数据后自动执行。

目标：

识别频道是否已被公开搜索引擎或第三方平台索引。

状态：

```text
PRIVATE_ONLY
WEB_SEARCHABLE
PARTIALLY_SEARCHABLE
UNKNOWN
```

检测来源：

* Telegram Web
* Google
* Bing
* PanSou
* Telegram Search Bot

数据库字段：

```text
discoverability
discoverability_checked_at
discoverability_source
```

搜索排序策略：

```text
PRIVATE_ONLY 优先
PARTIALLY_SEARCHABLE 次之
WEB_SEARCHABLE 靠后
UNKNOWN 最后
```

---

## 12. 数据库设计

核心表：

* telegram_accounts
* telegram_channels
* telegram_messages
* telegram_links
* telegram_media
* telegram_files
* sync_tasks
* search_tasks

### telegram_accounts

保存 Telegram 账号信息和 Session 状态。

### telegram_channels

保存频道、群组、Saved Messages 的元数据。

包含字段：

* discoverability
* discoverability_checked_at
* discoverability_source
* sync_enabled
* listen_enabled
* last_sync_message_id
* last_sync_at

### telegram_messages

保存已同步消息。

### telegram_links

保存从消息中提取出的链接。

### telegram_media

媒体信息表。

Phase 1 只建表，不实现完整业务。

### telegram_files

文件信息表。

Phase 1 只建表，不实现完整业务。

### sync_tasks

保存历史同步任务状态。

### search_tasks

预留异步搜索任务。

---

## 13. 媒体架构预留

Phase 1：

* 仅建表
* 仅保存基础元数据
* 不实现代理业务

未来支持：

### Media Recognition

识别：

* 封面
* 标题
* 简介
* 类型

### Proxy

预留能力：

* Image Proxy
* Video Proxy
* File Proxy
* Download Proxy

---

## 14. 搜索架构

### Phase 1：SQLite FTS5

目标：

* 支持 100 万消息以内
* 搜索响应小于 200ms
* 支持高亮
* 支持分页
* 支持频道过滤
* 支持时间过滤

### Phase 2：Bleve

支持：

* 增量索引
* 更复杂查询
* 更好的中文分词扩展

### Phase 3：ElasticSearch

适用于：

* 企业部署
* 大规模数据
* 多账号大数据量搜索

---

## 15. 实时监听

使用：

```text
gotd Updates Engine
```

监听事件：

* 新消息
* 编辑消息
* 删除消息

要求：

* 自动恢复
* 自动重连
* FloodWait 处理
* Gap Recovery
* 消息零丢失
* 幂等写入
* 监听状态可观测

---

## 16. API

提供 REST API。

规范：

* OpenAPI 3.1

认证方式：

Phase 1：

* API Key

Phase 2：

* JWT

Phase 3：

* OAuth2

---

## 17. Phase 1 MVP

必须实现：

* Setup Wizard
* 管理员账号
* API Key
* Telegram 登录
* Session 管理
* 频道元数据拉取
* 手动选择监听频道
* 手动选择同步频道
* SQLite
* SQLite FTS5
* 历史同步
* 实时监听
* 搜索
* Vue 管理后台
* Channel Discoverability Detection
* Link Extractor
* Dockerfile
* Docker Compose

不要实现：

* ElasticSearch
* Redis
* 向量数据库
* 视频代理
* 图片代理
* 文件代理
* 下载代理

优先保证：

* 长期稳定运行
* Docker 友好
* NAS 友好
* 消息零丢失
* 搜索响应快
* 首次启动体验流畅
* 避免默认同步海量历史消息

---

## 18. 同步策略约束

必须遵守：

1. 首次登录只拉取频道、群组、Saved Messages 元数据。
2. 不得默认同步所有频道历史消息。
3. 只有用户明确勾选的频道才允许同步历史消息。
4. 只有用户明确勾选的频道才开启实时监听。
5. 默认历史同步数量为 1000 条。
6. 用户可手动修改同步数量。
7. 所有同步任务必须可暂停、可恢复、可重试。
8. 同步过程必须记录进度。
9. 同步写入必须幂等。
10. 监听过程中不得丢消息。

---

## 19. 参考项目

参考：

参考项目
* /home/harold/workspace/pansou
* /home/harold/workspace/teldrive

参考文档：

* docs/superpowers/plans/2026-06-07-pansou-gap-closure.md
* docs/teldrive-reference.md
* docs/tracking-helper-reference.md

不要直接复制实现。

仅借鉴：

* 架构设计
* 同步策略
* 索引设计
* 监听机制
* UI 交互设计

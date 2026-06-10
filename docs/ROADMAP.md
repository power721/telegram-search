# ROADMAP.md

# tg-search Product Roadmap

## Vision

tg-search 不只是一个 Telegram 搜索工具。

目标是构建：

> Self-hosted Telegram Resource Discovery Platform

帮助用户从 Telegram 频道中发现、整理、聚合、搜索和消费资源，并通过 API、Bot、RSS、媒体代理等方式对外提供统一服务。

---

# Current Status (v1.0)

已完成：

## Core Platform

* Telegram 多账号登录
* Session 持久化
* 频道元数据同步
* 历史消息同步
* 实时监听
* SQLite 存储
* FTS5 全文搜索

## Resource Extraction

* Cloud Drive
* Magnet
* ED2K
* HTTP Links
* Telegram Files

## Management

* Vue Admin UI
* Task Center
* Logs
* Runtime Settings
* Backup & Maintenance

## Public Services

* Public Search API
* Signed Media URLs
* Video Proxy
* Image Proxy

---

# v1.1 Resource Discovery

目标：

提升搜索结果质量。

## Resource Score

新增资源评分系统。

评分来源：

* 来源频道数量
* 出现次数
* 更新时间
* 文件完整度

新增字段：

```sql
resource_score
resource_popularity
```

搜索结果默认按评分排序。

---

## Resource Aggregation

多个频道中的同一资源自动聚合。

当前：

流浪地球2
流浪地球2
流浪地球2

优化后：

流浪地球2

来源：

* Channel A
* Channel B
* Channel C

资源：

* Quark
* Aliyun
* Magnet

---

## Resource Trends

新增热门资源排行榜。

支持：

* 今日热门
* 本周热门
* 本月热门

API：

GET /api/trending

---

## Search Analytics

统计：

* 热门关键词
* 零结果关键词
* API调用趋势

---

# v1.2 Subscription Platform

目标：

让用户持续回来。

## Saved Search

用户保存搜索条件。

示例：

哪吒3

自动跟踪新资源。

---

## Notification Center

支持：

* Telegram Bot
* Webhook
* RSS
* Email

事件：

* resource.created
* task.completed
* account.offline

---

## RSS Feeds

新增：

/feeds/latest

/feeds/movies

/feeds/software

/feeds/search?q=keyword

---

## Webhook Platform

支持：

POST /api/webhooks

配置：

* URL
* Secret
* Event Types

---

# v1.3 Telegram Ecosystem

目标：

成为 Telegram 内部资源搜索入口。

## Telegram Search Bot

支持：

/search keyword

/latest

/trending

/sub keyword

---

## Telegram Inline Mode

支持：

@tgsearch movie

直接返回搜索结果。

---

## Share Resource

支持：

资源分享到：

* Telegram
* RSS
* Webhook

---

# v1.4 Media Metadata

目标：

资源结构化。

## Metadata Extraction

自动提取：

* Title
* Year
* Season
* Episode
* Resolution
* Codec
* Size

---

## TMDB Integration

自动匹配：

* Movie
* TV Show

新增字段：

* tmdb_id
* imdb_id

---

## Resource Deduplication

统一：

流浪地球

流浪地球2

流浪地球导演版

形成作品实体。

---

## Artwork Support

自动获取：

* Poster
* Backdrop
* Logo

---

# v1.5 Content Library

目标：

从搜索引擎升级为资源库。

## Work Page

作品页：

流浪地球2

包含：

* 简介
* 海报
* 标签
* 所有资源

---

## Collections

合集：

* 漫威宇宙
* 哈利波特
* 刘慈欣作品

---

## Related Content

相关推荐：

看过：

流浪地球

推荐：

* 三体
* 球状闪电

---

# v2.0 Search Engine

目标：

Telegram Google。

## Channel Classification

自动分类频道：

* Movie
* TV
* Anime
* Music
* Software
* Books

---

## Channel Tags

自动标签：

4K
蓝光
夸克
阿里云

---

## Channel Ranking

频道评分：

* 更新频率
* 资源数量
* 活跃度

---

## Discovery

发现页面：

热门频道

热门资源

最新资源

---

# v2.1 AI Search

目标：

超越关键词搜索。

## Embedding Search

引入：

* Qdrant

支持：

语义搜索

---

## Similar Resource Search

搜索：

刘慈欣电影

返回：

* 流浪地球
* 三体
* 球状闪电

---

## AI Classification

自动识别：

* 影视
* 软件
* 音乐
* 电子书

---

## AI Tagging

自动生成：

* 类型
* 风格
* 标签

---

# v2.2 Ecosystem

目标：

开放平台。

## OpenAPI SDK

提供：

* Go SDK
* Python SDK
* TypeScript SDK

---

## TVBox Integration

专用接口：

/api/tvbox/search

---

## AList Integration

统一资源入口。

---

## Plugin System

支持：

* Parser Plugins
* Metadata Plugins
* Notification Plugins

---

# v3.0 Resource Network

长期目标。

## Multi-node Search

多个 tg-search 节点联合搜索。

---

## Federated Search

跨实例搜索。

---

## Resource Exchange

资源共享网络。

---

## Global Discovery

形成去中心化资源发现平台。

---

# Priority Order

P0

* Resource Score
* Resource Aggregation
* Saved Search

P1

* RSS
* Webhook
* Telegram Bot

P2

* Metadata Extraction
* TMDB Integration

P3

* AI Search
* Qdrant

P4

* Federated Search

---

# Non Goals

当前不计划：

* Telegram 消息发送
* Telegram 群管理
* Telegram CRM
* IM客户端替代
* 云端托管服务

tg-search 专注于：

资源发现
资源搜索
资源聚合
资源分发

# tg-search 公开 API 集成文档

公开 API 用于外部系统查询 tg-search 的本地资源库。它不会提供管理能力，也不会触发 Telegram 远程搜索。

## 1. 接入范围

公开可用接口：

```text
GET  /api/search
POST /api/search
GET  /v/:fileid
HEAD /v/:fileid
GET  /i/:fileid
HEAD /i/:fileid
```

`/api/search` 查询已经索引到本地资源库的链接和文件。资源来自历史同步或实时监听写入的本地数据，默认覆盖：

- 网盘资源：`cloud_drive`
- 磁力：`magnet`
- 电驴：`ed2k`
- Telegram 视频文件：`video`

## 2. 认证

公开 API 使用 API Key。推荐请求头：

```text
X-API-Key: <api-key>
```

也支持：

```text
Authorization: <api-key>
```

API Key 不支持放在 query 参数里。

示例：

```bash
export TG_SEARCH_API_KEY='0123abcd...'

curl -H "X-API-Key: $TG_SEARCH_API_KEY" \
  "http://127.0.0.1:9900/api/search?kw=电影"
```

## 3. 搜索接口

### `GET /api/search`

适合浏览器、简单脚本和只读查询。

```bash
curl -G "http://127.0.0.1:9900/api/search" \
  -H "X-API-Key: $TG_SEARCH_API_KEY" \
  --data-urlencode "kw=电影" \
  --data-urlencode "res=merge" \
  --data-urlencode "cloud_types=quark,aliyun" \
  --data-urlencode "include_image=1" \
  --data-urlencode "include_media_metadata=1" \
  --data-urlencode "limit=20" \
  --data-urlencode "offset=0"
```

### `POST /api/search`

适合服务端集成和复杂参数。

```bash
curl "http://127.0.0.1:9900/api/search" \
  -H "X-API-Key: $TG_SEARCH_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "kw": "电影",
    "res": "all",
    "cloud_types": ["quark", "aliyun"],
    "include_image": true,
    "include_media_metadata": true,
    "limit": 20,
    "offset": 0
  }'
```

## 4. 请求参数

| 参数 | GET | POST JSON | 说明 |
| --- | --- | --- | --- |
| 关键词 | `kw`、`q`、`keyword` | `kw` 或 `q` | 搜索关键词。可为空，表示按过滤条件返回最新资源。 |
| 响应类型 | `res` | `res` | `merge`、`merged_by_type`、`results`、`all`。默认 `merged_by_type`。 |
| 资源类型 | `cloud_types` | `cloud_types` | GET 为逗号分隔；POST 为字符串数组，也兼容带逗号的元素。 |
| 图片 | `include_image` | `include_image` | 是否返回图片代理 URL。 |
| 媒体元数据 | `include_media_metadata` 或 `media_metadata` | `include_media_metadata` 或 `media_metadata` | 是否返回标题、年份、季集、清晰度、大小等媒体字段。 |
| 数量 | `limit` | `limit` | 默认 `50`，最大 `3000`。 |
| 偏移 | `offset` | `offset` | 默认 `0`。 |

布尔值 GET 参数支持：

```text
1 true t yes y on
0 false f no n off
```

## 5. 响应类型

`res` 参数决定 `data` 中包含哪些字段：

| `res` 值 | 响应字段 | 说明 |
| --- | --- | --- |
| 空、`merge`、`merged_by_type` | `merged_by_type` | 按资源类型聚合链接，适合资源站/机器人直接消费。 |
| `results` | `results` | 按消息/资源条目返回，保留标题、内容、链接、图片和媒体信息。 |
| `all` | `results` 和 `merged_by_type` | 同时返回两种结构。 |

无论哪种类型，都会返回 `total`。

## 6. 成功响应

公开 API 使用兼容封装：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "total": 2,
    "merged_by_type": {
      "quark": [
        {
          "url": "https://pan.quark.cn/s/xxxx",
          "password": "abcd",
          "note": "资源标题",
          "datetime": "2026-06-08T02:00:00Z",
          "images": ["/i/123?exp=...&sig=..."],
          "media": {
            "title": "电影名",
            "year": "2026",
            "quality": "1080p",
            "size": "2.1GB",
            "tmdb_id": "12345",
            "category": "movie",
            "tags": "动作,悬疑"
          }
        }
      ],
      "magnet": []
    }
  }
}
```

`results` 结构：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "total": 1,
    "results": [
      {
        "unique_id": "link:https://pan.quark.cn/s/xxxx",
        "datetime": "2026-06-08T02:00:00Z",
        "title": "资源标题",
        "content": "",
        "links": [
          {
            "type": "quark",
            "url": "https://pan.quark.cn/s/xxxx",
            "password": "abcd",
            "datetime": "2026-06-08T02:00:00Z",
            "work_title": "资源标题",
            "media": {
              "title": "电影名",
              "year": "2026"
            }
          }
        ],
        "images": ["/i/123?exp=...&sig=..."],
        "media": {
          "title": "电影名",
          "year": "2026"
        }
      }
    ]
  }
}
```

字段说明：

| 字段 | 说明 |
| --- | --- |
| `total` | 命中过滤条件的总数。多个 `cloud_types` 会分别统计后合计。 |
| `unique_id` | tg-search 资源 ID，形如 `link:<url>` 或 `file:<id>`。 |
| `datetime` | 资源对应消息时间。 |
| `title` | 资源标题。启用媒体元数据时优先使用媒体标题。 |
| `links[].type` | 资源类型，例如 `quark`、`aliyun`、`magnet`、`ed2k`、`video`。 |
| `links[].url` | 外部链接或签名后的视频代理 URL。 |
| `links[].password` | 网盘提取码，可能为空。 |
| `images` | 图片代理 URL，仅 `include_image=true` 时返回。 |
| `media` | 媒体元数据，仅 `include_media_metadata=true` 且有数据时返回。 |

## 7. 资源类型过滤

`cloud_types` 为空时默认搜索：

```text
cloud_drive, magnet, ed2k, video
```

支持分组值：

| 值 | 说明 |
| --- | --- |
| `cloud_drive`、`pan`、`drive` | 所有网盘资源。 |
| `magnet` | 磁力链接。 |
| `ed2k` | 电驴链接。 |
| `video` | Telegram 视频文件。 |

支持网盘提供商：

```text
quark
baidu
aliyun
uc
xunlei
tianyi
115
mobile
pikpak
123
guangya
weiyun
lanzou
jianguoyun
```

中文和常见别名会被归一化：

| 输入别名 | 归一化 |
| --- | --- |
| `百度`、`百度云`、`百度网盘` | `baidu` |
| `阿里`、`阿里云盘`、`alipan`、`aliyundrive` | `aliyun` |
| `夸克`、`夸克网盘` | `quark` |
| `天翼云盘` | `tianyi` |
| `115网盘` | `115` |
| `迅雷云盘` | `xunlei` |
| `移动云盘`、`和彩云` | `mobile` |
| `UC网盘` | `uc` |
| `PikPak网盘` | `pikpak` |
| `123云盘`、`123pan` | `123` |
| `磁力` | `magnet` |
| `电驴` | `ed2k` |

示例：

```text
/api/search?kw=剧集&cloud_types=夸克,阿里云盘,磁力
```

等价于：

```text
/api/search?kw=剧集&cloud_types=quark,aliyun,magnet
```

## 8. 媒体 URL

当结果中有 Telegram 图片或视频时，公开 API 返回的 URL 会自动签名：

```text
/i/123?exp=1780912800&sig=...
/v/456?exp=1780912800&sig=...
```

规则：

- 默认有效期为 24 小时。
- 过期后重新调用 `/api/search` 获取新 URL。
- `/v/:fileid` 支持 `Range`，可用于 HTML5 视频播放器拖动进度。
- `/i/:fileid` 返回图片数据，带 `Cache-Control: public, max-age=86400`。
- 也可以直接使用 `X-API-Key` 请求 `/v/:fileid` 或 `/i/:fileid`。

视频 Range 示例：

```bash
curl -I \
  -H "Range: bytes=0-1048575" \
  "http://127.0.0.1:9900/v/456?exp=1780912800&sig=..."
```

## 9. 错误响应

公开 API 错误仍使用 `code/message` 封装，HTTP 状态码表示错误类型：

```json
{
  "code": 400,
  "message": "invalid res"
}
```

常见错误：

| HTTP | message | 原因 |
| --- | --- | --- |
| `400` | `invalid res` | `res` 不是 `merge`、`merged_by_type`、`results` 或 `all`。 |
| `400` | `limit must be a non-negative integer` | `limit` 不是非负整数。 |
| `400` | `offset must be a non-negative integer` | `offset` 不是非负整数。 |
| `400` | `include_image must be a boolean` | 布尔参数格式错误。 |
| `401` | `api key is required` | 未传 API Key。 |
| `401` | `invalid api key` | API Key 无效或已被重新生成。 |
| `503` | `resources are unavailable` | 资源服务不可用。 |

## 10. 集成建议

- 后端服务保存 API Key，不要把 API Key 暴露给不可信前端。
- 用 `res=merge` 获取按类型聚合的链接，适合资源站、机器人和插件。
- 用 `res=results` 获取完整条目，适合需要展示标题、图片和媒体元数据的应用。
- 打开 `include_image` 会为结果补充媒体代理 URL，数量较大时会增加服务端查询和签名开销。
- `limit` 最大为 `3000`，外部系统建议分页拉取。
- 媒体 URL 过期后不要重试旧 URL，重新搜索获取新签名。

## 11. 最小调用示例

Node.js：

```js
const baseURL = 'http://127.0.0.1:9900'
const apiKey = process.env.TG_SEARCH_API_KEY

async function search(keyword) {
  const url = new URL('/api/search', baseURL)
  url.searchParams.set('kw', keyword)
  url.searchParams.set('res', 'all')
  url.searchParams.set('cloud_types', 'quark,aliyun,magnet')
  url.searchParams.set('include_media_metadata', '1')

  const response = await fetch(url, {
    headers: { 'X-API-Key': apiKey }
  })
  const body = await response.json()
  if (!response.ok || body.code !== 0) {
    throw new Error(body.message || `HTTP ${response.status}`)
  }
  return body.data
}
```

Python：

```python
import os
import requests

base_url = "http://127.0.0.1:9900"
api_key = os.environ["TG_SEARCH_API_KEY"]

resp = requests.get(
    f"{base_url}/api/search",
    headers={"X-API-Key": api_key},
    params={
        "kw": "电影",
        "res": "merge",
        "cloud_types": "quark,aliyun,magnet",
        "include_image": "1",
    },
    timeout=10,
)
data = resp.json()
if resp.status_code != 200 or data.get("code") != 0:
    raise RuntimeError(data.get("message", resp.text))

print(data["data"]["merged_by_type"])
```

# Phase 4 Cloud Drive Link Parsing Design

## Goal

Implement Phase 4 tasks 054-070: provider-specific cloud drive link parsing, parser test corpus, continued history/update integration through the existing extractor, `link_type` search coverage, and enhanced `/api/links` filtering with date range support.

## Scope

This phase includes:

- A parser abstraction inside `internal/link` while preserving the existing public API: `link.NewExtractor().Extract(text) []model.Link`.
- Provider-specific link type detection for:
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
  - `guangya`
  - `magnet`
  - `ed2k`
  - fallback `url`
- Password/code extraction from provider URL captures, inline provider patterns, and nearby text.
- A realistic parser corpus covering positive cases and false positives.
- Tests proving history sync and update processing store provider-specific link types.
- Tests proving `GET /api/search?q=x&link_type=aliyun` filters by provider type.
- `GET /api/links` date range filters through `date_from` and `date_to`.

This phase does not include:

- New database tables or migrations.
- Link validity checks against cloud provider APIs.
- Performance optimization work from Phase 5.
- AList-TVBox response contract changes from Phase 6.

## Current Code Context

`internal/link.Extractor` already extracts generic HTTP URLs, magnet links, ED2K links, nearby passwords, and deduplicates links within one message.

Historical sync already calls the extractor in `internal/history.Service.storeBatch`. Realtime new/edit updates already call the extractor in `internal/update.Processor.storeMessage`. Search already accepts `link_type` and repository search already uses an `EXISTS` filter against `telegram_links.type`.

`GET /api/links` already supports:

- `type`
- `account_id`
- `channel_id`
- `keyword`
- `limit`
- `offset`

The main gaps are provider-specific `type` values, stronger parser tests, and date range support for links.

## Parser Architecture

Keep `Extractor` as the public entry point:

```go
type Extractor struct {
    parsers []Parser
}

func NewExtractor() *Extractor
func (e *Extractor) Extract(text string) []model.Link
```

Add an internal parser abstraction:

```go
type Parser interface {
    Extract(text string) []Candidate
}

type Candidate struct {
    Type        string
    URL         string
    Password    string
    MatchStart   int
    MatchEnd     int
}
```

The extractor will:

1. Run provider parsers first.
2. Run protocol parsers for magnet and ED2K.
3. Run a generic URL parser as fallback.
4. Normalize trailing punctuation.
5. Deduplicate by normalized URL, preserving the first provider-specific match.
6. Fill missing passwords from nearby text after the URL.

Provider-specific parsers must not depend on SQLite, Telegram, or HTTP.

## Provider Patterns

The Go implementation should use case-insensitive regular expressions equivalent to these user-provided reference patterns, adapted for Go syntax and message-boundary handling.

Provider parsers should preserve the full share URL after punctuation trimming, including non-password query parameters such as `?public=1`. Password query parameters are captured as `Password`, but they are not removed from the stored URL.

`115`:

```text
https://(?:115|115cdn|anxia).com/s/<share_id>[?password=<code>]
```

`xunlei`:

```text
https://pan.xunlei.com/s/<share_id>[?pwd=<code>]
```

`baidu`:

```text
https://pan.baidu.com/s/<share_id>[?pwd=<code>]
https://pan.baidu.com/(share|wap)/init?surl=<share_id>[&pwd=<code>]
```

`pikpak`:

```text
https://mypikpak.com/s/<share_id>[?pwd=<code>]
```

`tianyi`:

```text
https://cloud.189.cn/web/share?code=<share_id>
https://cloud.189.cn/t/<share_id>[（访问码：<code>）]
https://h5.cloud.189.cn/share.html#/t/<share_id>
```

`mobile`:

```text
https://caiyun.139.com/m/i?<share_id>
https://yun.139.com/shareweb/#/w/i/<share_id>
https://caiyun.139.com/w/i/<share_id>
```

`quark`:

```text
https://pan.quark.cn/s/<share_id>
```

`uc`:

```text
https://drive.uc.cn/s/<share_id>[?password=<code>]
https://fast.uc.cn/s/<share_id>[?password=<code>]
```

The UC parser must also accept share URLs with other query parameters, for example:

```text
https://drive.uc.cn/s/<share_id>?public=1
```

`aliyun`:

```text
https://www.alipan.com/s/<share_id>/folder/<folder_id>[?password=<code>]
https://www.aliyundrive.com/s/<share_id>/folder/<folder_id>[?password=<code>]
https://www.alipan.com/s/<share_id>[?password=<code>]
https://www.aliyundrive.com/s/<share_id>[?password=<code>]
```

`123`:

```text
https://123*.com/s/<share_id>提取码:<code>
https://123*.com/s/<share_id>.html[?提取码:<code>]
https://www.123*.com/s/<share_id>.html[?提取码:<code>]
```

The `123*.com` family follows the reference `123...\.com` shape. The implementation should accept three alphanumeric characters after `123`, such as `123pan.com`, while tests should include at least `123pan.com`.

`guangya`:

```text
https://guangyapan.com/s/<share_id>
https://www.guangyapan.com/s/<share_id>
```

`magnet` and `ed2k`:

```text
magnet:?...
ed2k://...
```

All remaining HTTP(S) URLs should still be extracted as fallback type `url`.

## Password Extraction

Provider URL captures take priority. If a provider regex captures a password/code, use it directly.

If the provider match does not contain a password, the extractor scans nearby text after the URL using this keyword set:

```text
密码
提取码
验证码
访问码
分享密码
密钥
pwd
password
code
share_pwd
pass_code
#
```

The accepted separator forms are:

```text
=
:
：
whitespace
```

Provider URL password captures follow the provider pattern, including `_` and `-` where the share pattern allows them. Nearby text password extraction follows the supplied generic password pattern and accepts 1-4 alphanumeric characters.

## URL Normalization

Normalize extracted URLs by trimming trailing punctuation that commonly appears in Telegram messages:

```text
.,;:!?)]}）】》"'，#
```

The trailing `#` trim is required for messages like `https://pan.xunlei.com/s/<id>?pwd=kewd#`, where `#` is a separator after the password rather than part of the share URL.

Do not rewrite domains, lower-case full URLs, remove query parameters, or validate share ids beyond the parser regex. The stored URL should remain the share URL as it appeared after punctuation trimming.

## Real Message Corpus

The parser test corpus must include a realistic multi-link Telegram message shaped like:

```text
海报
名称：2026年6月6日 短剧更新目录12

描述：1.本想当工具人，结果被圣女逼婚（80集）李进源＆索菲
2.百岁悟道仙途第二季（94集）动漫短剧
3.师叔的灵宝有点妖（34集）动漫短剧

链接：
🔗 夸克网盘：https://pan.quark.cn/s/8a16ab9c06b9
🔗 百度网盘：https://pan.baidu.com/s/1Zc_e4792cuvucfI-ZZts0Q?pwd=ruub
🔑 提取码：ruub
🔗 UC 网盘：https://drive.uc.cn/s/d5eaad53da684?public=1
🔗 迅雷云盘：https://pan.xunlei.com/s/VOuNMeKrMwroW9HmY21cZWfPA1?pwd=kewd#
🔑 提取码：kewd
🔗 阿里云盘：https://www.alipan.com/s/MHf34XusdVK

📁 大小：N
🏷 标签：#短剧 #最新短剧 #合集
📢 频道：https://t.me/+Djia5z2lVsI5ODRl
👥 群组：@Quark_Share_Group (https://t.me/Quark_Share_Group)
🤖 投稿：@QuarkRobot (https://t.me/QuarkRobot)
```

Expected extraction from this corpus:

- `quark`: `https://pan.quark.cn/s/8a16ab9c06b9`
- `baidu`: `https://pan.baidu.com/s/1Zc_e4792cuvucfI-ZZts0Q?pwd=ruub`, password `ruub`
- `uc`: `https://drive.uc.cn/s/d5eaad53da684?public=1`
- `xunlei`: `https://pan.xunlei.com/s/VOuNMeKrMwroW9HmY21cZWfPA1?pwd=kewd`, password `kewd`
- `aliyun`: `https://www.alipan.com/s/MHf34XusdVK`

The Telegram channel/group/bot URLs in the same message must not be classified as cloud-drive provider types. They may still be extracted as fallback `url` links to preserve the Phase 1 generic URL behavior, but they must not affect `link_type` searches for provider types.

## Search And Links API

`GET /api/search` keeps the existing `link_type` query parameter:

```text
GET /api/search?q=movie&link_type=aliyun
```

The repository already implements provider filtering using `telegram_links.type`; Phase 4 adds coverage with provider-specific extracted links.

`GET /api/links` keeps existing filters and adds:

```text
date_from
date_to
```

Date parsing rules:

- Accept `YYYY-MM-DD`.
- Accept RFC3339 timestamps.
- `date_from` is inclusive.
- `date_to` is inclusive for date-only input by internally using the next day as an exclusive upper bound.
- Invalid date values return HTTP 400.

Filters can be combined:

```text
GET /api/links?type=aliyun&account_id=1&channel_id=2&keyword=movie&date_from=2026-01-01&date_to=2026-01-31&limit=50&offset=0
```

Repository filtering should use message date (`telegram_messages.date`), not link creation time, because users search by message/resource date.

## Integration

No new integration point is needed for history or updates:

- `internal/history.Service` already calls `Extractor.Extract` before saving links.
- `internal/update.Processor` already calls `Extractor.Extract` for new and edited messages.
- `LinkRepository.ReplaceForMessageTx` already refreshes links on edit.

Phase 4 tests should update existing history/update tests to assert provider-specific types, proving the enhanced extractor flows through both paths.

## Error Handling

- Parser failures are not expected because regex parsing is local and deterministic.
- Invalid `/api/links` dates return HTTP 400 with an error message.
- Unknown `type` values are allowed and simply return no results.
- Empty `date_from` or `date_to` means the corresponding bound is absent.

## Testing

Add focused tests:

- Parser corpus with one or more positive samples for every supported provider type.
- A realistic multi-link Telegram message with Quark, Baidu, UC, Xunlei, Aliyun, hashtags, and Telegram channel/group/bot URLs.
- Password extraction for query-string passwords, inline Chinese access codes, and nearby keyword text.
- False positive corpus for non-share URLs and random text.
- Deduplication where provider parser and fallback URL parser both see the same URL.
- History sync stores provider-specific link types.
- Update processor creates provider-specific links on new messages and replaces them on edit.
- Search `link_type` filters by provider type.
- `/api/links` combines type/account/channel/keyword/date range/pagination filters.
- Invalid `/api/links` dates return 400.

## Acceptance Mapping

- Task 054: internal parser abstraction behind `Extractor`.
- Tasks 055-064: provider-specific parsers for the listed cloud drives.
- Task 065: magnet and ED2K remain supported and covered by corpus tests.
- Task 066: parser corpus with positives and false positives.
- Task 067: history sync integration covered through provider-specific type assertions.
- Task 068: update listener integration covered through processor tests.
- Task 069: `link_type` provider filter covered through search/API tests.
- Task 070: enhanced links API covered with combined filters, pagination, and date range.

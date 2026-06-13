# 频道头像本地文件缓存

## 现状
- 频道头像已使用 `ImageCache` (`storage.MediaCache`) 缓存
- 存储路径：`thumbnails/image-proxy`（与消息媒体代理共享）
- 无专门的头像缓存管理

## 问题
- 头像缓存与消息图片混在一起，难以单独管理
- 头像更稳定，应有不同的 TTL 和清理策略
- 需要独立的存储空间

## 方案
创建专用的 `AvatarCache`，独立于消息媒体缓存

### 变更文件
1. `internal/config/config.go` - 添加 `RuntimeDirs` 包含 `avatars` 目录
2. `internal/api/router.go` - 添加 `AvatarCache` 字段并初始化
3. `internal/api/channel_avatar.go` - 使用专用 `AvatarCache` 而非 `ImageCache`

### 实现细节
- 存储路径：`storage.Path/avatars`
- TTL: 30天（头像较稳定，比消息图片的7天更长）
- 复用 `storage.MediaCache` 实现，仅配置不同
- 缓存键保持：`ch-avatar:{channel_id}:{photo_id}`

### 测试
- 验证头像缓存目录创建
- 验证头像下载并缓存到正确目录
- 验证缓存命中和更新

## 未解决问题
- 是否需要单独的配置项控制头像缓存大小？（暂用 `MaxMediaCache`）
- 是否需要 API 端点清理头像缓存？（暂不需要）

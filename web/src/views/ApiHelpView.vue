<script setup lang="ts">
import { ref } from 'vue'

interface ParamRow {
  name: string
  type: string
  required: string
  description: string
}

interface FieldRow {
  name: string
  type: string
  description: string
}

const copiedKey = ref('')

const authOptions = [
  { label: '请求头', value: 'X-API-Key: YOUR_API_KEY' },
  { label: 'Bearer', value: 'Authorization: Bearer YOUR_API_KEY' },
  { label: '查询参数', value: 'api_key=YOUR_API_KEY' }
]

const searchParams: ParamRow[] = [
  { name: 'kw', type: 'string', required: '是', description: '搜索关键词。GET 也兼容 q、keyword。' },
  { name: 'q', type: 'string', required: '否', description: 'kw 的兼容别名。' },
  { name: 'res', type: 'string', required: '否', description: '返回结构：merge、results、all。默认 merge。' },
  { name: 'cloud_types', type: 'string[]', required: '否', description: '资源类型或网盘类型，GET 用逗号分隔。支持 cloud_drive、magnet、ed2k、video、quark、baidu、aliyun、uc、xunlei、tianyi、115、mobile、pikpak、123 等。' },
  { name: 'include_media_metadata', type: 'boolean', required: '否', description: '返回媒体元数据。GET 支持 true/false、1/0、yes/no。' },
  { name: 'media_metadata', type: 'boolean', required: '否', description: 'include_media_metadata 的兼容别名。' },
  { name: 'limit', type: 'number', required: '否', description: '分页数量，默认 50，最大 3000。' },
  { name: 'offset', type: 'number', required: '否', description: '分页偏移量，默认 0。' }
]

const searchFields: FieldRow[] = [
  { name: 'code', type: 'number', description: '0 表示成功。' },
  { name: 'message', type: 'string', description: '响应说明。' },
  { name: 'data.total', type: 'number', description: '命中的资源总数。' },
  { name: 'data.results', type: 'array', description: 'res=results 或 all 时返回的明细列表。' },
  { name: 'data.merged_by_type', type: 'object', description: 'res=merge 或 all 时返回的按类型聚合结果。' },
  { name: 'media', type: 'object', description: 'include_media_metadata=true 时返回标题、年份、季集、清晰度、大小、TMDB 等字段。' }
]

const healthFields: FieldRow[] = [
  { name: 'service', type: 'string', description: '服务状态。正常时为 ok。' }
]

const mediaParams: ParamRow[] = [
  { name: 'channel', type: 'string', required: '是', description: '频道用户名或频道 ID。路径中不需要 @ 前缀。' },
  { name: 'msgid', type: 'number', required: '是', description: 'Telegram 消息 ID，必须是正整数。' },
  { name: 'exp', type: 'string', required: '签名访问必填', description: '签名 URL 的过期时间戳，由搜索结果里的媒体 URL 自动携带。' },
  { name: 'sig', type: 'string', required: '签名访问必填', description: '媒体 URL 签名，由搜索结果里的媒体 URL 自动携带。' }
]

const getSearchExample = `curl -G 'http://localhost:9900/api/search' \\
  -H 'X-API-Key: YOUR_API_KEY' \\
  --data-urlencode 'kw=ubuntu' \\
  --data-urlencode 'res=merge' \\
  --data-urlencode 'cloud_types=quark,aliyun' \\
  --data-urlencode 'limit=50'`

const postSearchExample = `curl -X POST 'http://localhost:9900/api/search' \\
  -H 'Content-Type: application/json' \\
  -H 'Authorization: Bearer YOUR_API_KEY' \\
  -d '{
    "kw": "ubuntu",
    "res": "all",
    "cloud_types": ["quark", "aliyun"],
    "include_media_metadata": true,
    "limit": 50,
    "offset": 0
  }'`

const healthExample = `curl 'http://localhost:9900/api/health'`

const videoExample = `curl 'http://localhost:9900/v/media_channel/102' \\
  -H 'X-API-Key: YOUR_API_KEY' \\
  -H 'Range: bytes=0-'`

const imageExample = `curl 'http://localhost:9900/i/media_channel/101' \\
  -H 'X-API-Key: YOUR_API_KEY'`

const signedMediaExample = `<video
  src="/v/media_channel/102?exp=1735689600&sig=SIGNED_VALUE"
  controls
></video>

<img
  src="/i/media_channel/101?exp=1735689600&sig=SIGNED_VALUE"
  alt=""
/>`

async function copyCode(key: string, value: string) {
  try {
    await navigator.clipboard.writeText(value)
  } catch {
    const textarea = document.createElement('textarea')
    textarea.value = value
    textarea.style.position = 'fixed'
    textarea.style.opacity = '0'
    document.body.appendChild(textarea)
    textarea.select()
    document.execCommand('copy')
    document.body.removeChild(textarea)
  }
  copiedKey.value = key
  window.setTimeout(() => {
    if (copiedKey.value === key) copiedKey.value = ''
  }, 1800)
}
</script>

<template>
  <section class="page-section api-help-page">
    <div class="page-header">
      <div>
        <p class="page-kicker">API</p>
        <h1 class="page-title">公开 API 帮助</h1>
        <p class="page-subtitle">外部搜索、健康检查和 Telegram 媒体访问接口的调用说明。</p>
      </div>
    </div>

    <section class="panel summary-panel">
      <div class="summary-item">
        <span class="method method-get">GET</span>
        <strong>/api/health</strong>
        <small>服务健康检查</small>
      </div>
      <div class="summary-item">
        <span class="method method-mixed">GET/POST</span>
        <strong>/api/search</strong>
        <small>资源搜索</small>
      </div>
      <div class="summary-item">
        <span class="method method-get">GET</span>
        <strong>/v/:channel/:msgid</strong>
        <small>视频流</small>
      </div>
      <div class="summary-item">
        <span class="method method-get">GET</span>
        <strong>/i/:channel/:msgid</strong>
        <small>图片</small>
      </div>
    </section>

    <section class="panel">
      <div class="panel-heading">
        <div>
          <p class="eyebrow">Authentication</p>
          <h2>认证方式</h2>
        </div>
      </div>
      <p class="doc-text">
        <code>/api/search</code> 必须携带 API Key。<code>/v</code> 和 <code>/i</code> 可以携带 API Key，
        也可以直接使用搜索结果中返回的带 <code>exp</code> 与 <code>sig</code> 的签名媒体 URL。
      </p>
      <div class="auth-grid">
        <div v-for="item in authOptions" :key="item.label" class="auth-card">
          <span>{{ item.label }}</span>
          <code>{{ item.value }}</code>
        </div>
      </div>
    </section>

    <section class="panel api-section">
      <div class="endpoint-heading">
        <span class="method method-mixed">GET/POST</span>
        <div>
          <h2>/api/search</h2>
          <p>从本地资源索引中搜索网盘、磁力、ED2K 和视频资源。</p>
        </div>
      </div>

      <h3>请求参数</h3>
      <div class="table-panel">
        <table>
          <thead>
            <tr>
              <th>参数</th>
              <th>类型</th>
              <th>必填</th>
              <th>说明</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="param in searchParams" :key="param.name">
              <td><code>{{ param.name }}</code></td>
              <td>{{ param.type }}</td>
              <td>{{ param.required }}</td>
              <td>{{ param.description }}</td>
            </tr>
          </tbody>
        </table>
      </div>

      <div class="code-grid">
        <div class="code-card">
          <div class="code-title">
            <strong>GET 示例</strong>
            <button type="button" @click="copyCode('search-get', getSearchExample)">
              {{ copiedKey === 'search-get' ? '已复制' : '复制' }}
            </button>
          </div>
          <pre><code>{{ getSearchExample }}</code></pre>
        </div>
        <div class="code-card">
          <div class="code-title">
            <strong>POST 示例</strong>
            <button type="button" @click="copyCode('search-post', postSearchExample)">
              {{ copiedKey === 'search-post' ? '已复制' : '复制' }}
            </button>
          </div>
          <pre><code>{{ postSearchExample }}</code></pre>
        </div>
      </div>

      <h3>响应字段</h3>
      <div class="table-panel">
        <table>
          <thead>
            <tr>
              <th>字段</th>
              <th>类型</th>
              <th>说明</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="field in searchFields" :key="field.name">
              <td><code>{{ field.name }}</code></td>
              <td>{{ field.type }}</td>
              <td>{{ field.description }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>

    <section class="panel api-section">
      <div class="endpoint-heading">
        <span class="method method-get">GET</span>
        <div>
          <h2>/api/health</h2>
          <p>用于检查服务进程是否可访问。</p>
        </div>
      </div>

      <div class="code-card single-code">
        <div class="code-title">
          <strong>请求示例</strong>
          <button type="button" @click="copyCode('health', healthExample)">
            {{ copiedKey === 'health' ? '已复制' : '复制' }}
          </button>
        </div>
        <pre><code>{{ healthExample }}</code></pre>
      </div>

      <div class="table-panel">
        <table>
          <thead>
            <tr>
              <th>字段</th>
              <th>类型</th>
              <th>说明</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="field in healthFields" :key="field.name">
              <td><code>{{ field.name }}</code></td>
              <td>{{ field.type }}</td>
              <td>{{ field.description }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>

    <section class="panel api-section">
      <div class="endpoint-heading">
        <span class="method method-get">GET</span>
        <div>
          <h2>/v/:channel/:msgid</h2>
          <p>读取 Telegram 消息中的视频文件，支持浏览器 Range 分段请求。</p>
        </div>
      </div>

      <h3>路径与查询参数</h3>
      <div class="table-panel">
        <table>
          <thead>
            <tr>
              <th>参数</th>
              <th>类型</th>
              <th>必填</th>
              <th>说明</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="param in mediaParams" :key="`video-${param.name}`">
              <td><code>{{ param.name }}</code></td>
              <td>{{ param.type }}</td>
              <td>{{ param.required }}</td>
              <td>{{ param.description }}</td>
            </tr>
          </tbody>
        </table>
      </div>

      <div class="code-card single-code">
        <div class="code-title">
          <strong>请求示例</strong>
          <button type="button" @click="copyCode('video', videoExample)">
            {{ copiedKey === 'video' ? '已复制' : '复制' }}
          </button>
        </div>
        <pre><code>{{ videoExample }}</code></pre>
      </div>
    </section>

    <section class="panel api-section">
      <div class="endpoint-heading">
        <span class="method method-get">GET</span>
        <div>
          <h2>/i/:channel/:msgid</h2>
          <p>读取 Telegram 消息中的图片，适合在外部页面或结果卡片中展示缩略图。</p>
        </div>
      </div>

      <h3>路径与查询参数</h3>
      <div class="table-panel">
        <table>
          <thead>
            <tr>
              <th>参数</th>
              <th>类型</th>
              <th>必填</th>
              <th>说明</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="param in mediaParams" :key="`image-${param.name}`">
              <td><code>{{ param.name }}</code></td>
              <td>{{ param.type }}</td>
              <td>{{ param.required }}</td>
              <td>{{ param.description }}</td>
            </tr>
          </tbody>
        </table>
      </div>

      <div class="code-grid">
        <div class="code-card">
          <div class="code-title">
            <strong>请求示例</strong>
            <button type="button" @click="copyCode('image', imageExample)">
              {{ copiedKey === 'image' ? '已复制' : '复制' }}
            </button>
          </div>
          <pre><code>{{ imageExample }}</code></pre>
        </div>
        <div class="code-card">
          <div class="code-title">
            <strong>签名 URL 使用示例</strong>
            <button type="button" @click="copyCode('signed-media', signedMediaExample)">
              {{ copiedKey === 'signed-media' ? '已复制' : '复制' }}
            </button>
          </div>
          <pre><code>{{ signedMediaExample }}</code></pre>
        </div>
      </div>
    </section>
  </section>
</template>

<style scoped>
.api-help-page {
  gap: 18px;
}

.summary-panel {
  display: grid;
  gap: 10px;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
}

.summary-item {
  border: 1px solid var(--app-border-subtle);
  border-radius: var(--app-radius);
  display: grid;
  gap: 6px;
  min-width: 0;
  padding: 12px;
}

.summary-item strong {
  color: var(--app-heading);
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 15px;
}

.summary-item small {
  color: var(--app-text-muted);
  font-size: 13px;
}

.panel-heading,
.endpoint-heading {
  align-items: flex-start;
  display: flex;
  gap: 12px;
  justify-content: space-between;
}

.endpoint-heading {
  justify-content: flex-start;
  margin-bottom: 16px;
}

.endpoint-heading h2,
.panel-heading h2 {
  color: var(--app-heading);
  font-size: 17px;
  line-height: 1.35;
  margin: 0;
}

.endpoint-heading p,
.doc-text {
  color: var(--app-text-muted);
  line-height: 1.6;
  margin: 6px 0 0;
}

.doc-text {
  margin-bottom: 14px;
}

.method {
  align-items: center;
  border: 1px solid transparent;
  border-radius: var(--app-radius);
  display: inline-flex;
  flex: 0 0 auto;
  font-size: 12px;
  font-weight: 750;
  justify-content: center;
  line-height: 22px;
  min-width: 52px;
  padding: 0 7px;
}

.method-get {
  background: var(--app-accent-subtle);
  border-color: color-mix(in srgb, var(--app-accent) 30%, var(--app-border));
  color: var(--app-accent);
}

.method-mixed {
  background: var(--app-success-bg);
  border-color: color-mix(in srgb, var(--app-success) 30%, var(--app-border));
  color: var(--app-success);
}

.auth-grid,
.code-grid {
  display: grid;
  gap: 12px;
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
}

.auth-card,
.code-card {
  border: 1px solid var(--app-border-subtle);
  border-radius: var(--app-radius);
  min-width: 0;
}

.auth-card {
  display: grid;
  gap: 7px;
  padding: 12px;
}

.auth-card span {
  color: var(--app-text-muted);
  font-size: 13px;
  font-weight: 650;
}

code,
pre {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

code {
  color: var(--app-heading);
  font-size: 0.94em;
}

.api-section {
  display: grid;
  gap: 14px;
}

.api-section h3 {
  color: var(--app-heading);
  font-size: 15px;
  margin: 0;
}

.code-title {
  align-items: center;
  border-bottom: 1px solid var(--app-border-subtle);
  display: flex;
  gap: 8px;
  justify-content: space-between;
  padding: 9px 10px;
}

.code-title strong {
  color: var(--app-heading);
  font-size: 14px;
}

.code-title button {
  background: var(--app-surface);
  border: 1px solid var(--app-border);
  border-radius: var(--app-radius);
  color: var(--app-text);
  cursor: pointer;
  min-height: 28px;
  padding: 3px 9px;
}

.code-title button:hover {
  background: var(--app-surface-muted);
  border-color: var(--app-border-strong);
}

pre {
  color: var(--app-text);
  font-size: 13px;
  line-height: 1.55;
  margin: 0;
  overflow: auto;
  padding: 12px;
  white-space: pre;
}

.single-code {
  max-width: 760px;
}

.table-panel td {
  min-width: 120px;
}

.table-panel td:last-child {
  min-width: 320px;
}

@media (max-width: 720px) {
  .panel-heading,
  .endpoint-heading {
    display: grid;
  }

  .summary-panel,
  .auth-grid,
  .code-grid {
    grid-template-columns: 1fr;
  }
}
</style>

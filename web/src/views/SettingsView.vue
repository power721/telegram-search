<template>
  <section class="page-section">
    <p class="page-kicker">配置</p>
    <h1 class="page-title">设置</h1>
    <div class="settings-grid">
      <section class="panel">
        <h2>存储</h2>
        <dl>
          <div>
            <dt>最大数据库容量</dt>
            <dd>10 GB</dd>
          </div>
          <div>
            <dt>最大媒体缓存</dt>
            <dd>20 GB</dd>
          </div>
        </dl>
      </section>
      <section class="panel">
        <h2>管理员</h2>
        <p>管理员资料和 API 密钥将在这里管理。</p>
      </section>
    </div>
  </section>
</template>

<style scoped>
.page-kicker {
  color: #667085;
  margin: 0 0 4px;
}

.page-title {
  font-size: 24px;
  margin: 0 0 18px;
}

.settings-grid {
  display: grid;
  gap: 16px;
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.panel {
  background: #ffffff;
  border: 1px solid #d9dee7;
  border-radius: 8px;
  padding: 14px;
}

h2 {
  font-size: 16px;
  margin: 0 0 12px;
}

dl {
  margin: 0;
}

dl div {
  display: flex;
  justify-content: space-between;
  padding: 7px 0;
}

dd {
  font-weight: 600;
  margin: 0;
}

p {
  color: #667085;
  margin: 0;
}

@media (max-width: 840px) {
  .settings-grid {
    grid-template-columns: 1fr;
  }
}
</style>

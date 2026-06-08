<template>
  <section class="page-section">
    <p class="page-kicker">Configuration</p>
    <h1 class="page-title">Settings</h1>
    <div class="settings-grid">
      <section class="panel">
        <h2>Storage</h2>
        <dl>
          <div>
            <dt>Max DB Size</dt>
            <dd>10 GB</dd>
          </div>
          <div>
            <dt>Max Media Cache</dt>
            <dd>20 GB</dd>
          </div>
        </dl>
      </section>
      <section class="panel">
        <h2>Admin</h2>
        <p>Admin profile and API keys will be managed here.</p>
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

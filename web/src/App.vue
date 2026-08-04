<template>
  <router-view />
</template>

<style>
:root {
  --kk-radius: 10px;
}
* { margin: 0; padding: 0; box-sizing: border-box; }
html, body, #app { height: 100%; }
body {
  font-family: 'Helvetica Neue', Arial, 'PingFang SC', 'Microsoft YaHei', sans-serif;
  -webkit-font-smoothing: antialiased;
}

/* ---- 全局滚动条美化 ---- */
::-webkit-scrollbar { width: 8px; height: 8px; }
::-webkit-scrollbar-thumb { background: #c8cdd4; border-radius: 4px; }
::-webkit-scrollbar-thumb:hover { background: #a8afb8; }
::-webkit-scrollbar-track { background: transparent; }

/* ---- 统一卡片圆角 ---- */
.el-card { border-radius: var(--kk-radius); }

/* ---- 表格空状态美化 ---- */
.el-table__empty-text { color: #909399; font-size: 13px; line-height: 60px; }

/* ---- 路由切换动画 ---- */
.page-fade-enter-active { transition: opacity .25s ease, transform .25s ease; }
.page-fade-leave-active { transition: opacity .12s ease; }
.page-fade-enter-from { opacity: 0; transform: translateY(6px); }
.page-fade-leave-to { opacity: 0; }
</style>

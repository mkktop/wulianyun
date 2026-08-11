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

/* ---- 路由切换动画：交叉淡入（离开的页面绝对定位脱离文档流，新页面原位淡入，
   既无“从底部顶上来”的位移，也无 mode=out-in 的白屏间隙） ---- */
.router-wrap { position: relative; }
.page-fade-enter-active { transition: opacity .2s ease; }
.page-fade-leave-active { transition: opacity .15s ease; position: absolute; top: 0; left: 0; right: 0; }
.page-fade-enter-from { opacity: 0; }
.page-fade-leave-to { opacity: 0; }
</style>

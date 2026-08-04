<template>
  <span class="clock">{{ time }}</span>
</template>

<script setup lang="ts">
// 顶栏时钟：独立组件，每秒更新只重渲染自身，不触发壳层重渲染
import { onMounted, onUnmounted, ref } from 'vue'

const time = ref('')
let timer = 0
const pad = (n: number) => String(n).padStart(2, '0')

function tick() {
  const d = new Date()
  time.value = `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}

onMounted(() => {
  tick()
  timer = window.setInterval(tick, 1000)
})
onUnmounted(() => window.clearInterval(timer))
</script>

<style scoped>
.clock { color: #909399; font-size: 13px; font-variant-numeric: tabular-nums; }
</style>

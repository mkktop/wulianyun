<template>
  <div class="sp-header">
    <div>
      <div class="sp-header-title">
        <el-icon><component :is="icon" /></el-icon>
        <h2>{{ title }}</h2>
      </div>
      <div v-if="desc" class="sp-header-desc">{{ desc }}</div>
    </div>
    <div class="sp-header-actions"><slot /></div>
  </div>
</template>

<script setup lang="ts">
defineProps<{ title: string; desc?: string; icon?: string }>()
</script>

import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { visualizer } from 'rollup-plugin-visualizer'

export default defineConfig({
  plugins: [
    vue(),
    // 包体分析：产物 dist/stats.html（仅构建期，不进运行时）。
    // 注：传输压缩由 nginx 实时 gzip 承担（见 nginx.conf 的 gzip on），无需构建期预生成 .gz。
    visualizer({ open: false, filename: 'dist/stats.html', gzipSize: true, brotliSize: false }),
  ],
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true,
        ws: true
      }
    }
  },
  build: {
    chunkSizeWarningLimit: 1500,
    rollupOptions: {
      output: {
        // 第三方依赖分包：独立缓存 + 并行下载
        manualChunks: {
          vue: ['vue', 'vue-router', 'pinia'],
          'element-plus': ['element-plus', '@element-plus/icons-vue'],
          echarts: ['echarts'],
        },
      },
    },
  },
})

import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  base: './', // 確保資源使用相對路徑
  build: {
    outDir: '../docs', // 將編譯產物放在根目錄的 docs 資料夾
    emptyOutDir: true,
  }
})

import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

const base = process.env.VITE_BASE || '/'

export default defineConfig({
  plugins: [vue()],
  base,
  server: {
    proxy: {
      [base.replace(/\/$/, '') + '/api']: {
        target: 'http://localhost:3001',
        rewrite: (path) => path.replace(new RegExp('^' + base.replace(/\/$/, '')), '')
      }
    }
  }
})

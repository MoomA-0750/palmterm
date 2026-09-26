import { defineConfig } from "vite";

// 開発時は Go のサーバー（127.0.0.1:7681）へ /ws・/api・/auth を中継する。
// Go 側は -allow-origin で Vite の Origin を許しておく。
const backend = "http://127.0.0.1:7681";

export default defineConfig({
  server: {
    host: true,
    proxy: {
      "/ws": { target: backend, ws: true },
      "/api": backend,
      "/auth": backend,
    },
  },
});

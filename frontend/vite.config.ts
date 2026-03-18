import { defineConfig, loadEnv } from "vite";
import react from "@vitejs/plugin-react";
import path from "path";

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), "");
  const configuredBase = env.VITE_APP_BASE_PATH || "/";
  const base = configuredBase.endsWith("/") ? configuredBase : `${configuredBase}/`;

  return {
    base,
    plugins: [react()],
    resolve: {
      alias: { "@": path.resolve(__dirname, "./src") },
    },
    server: {
      host: "0.0.0.0",
      port: 5173,
      proxy: {
        "/api": { target: "http://localhost:8080", changeOrigin: true },
        "/ws":  { target: "ws://localhost:8080", ws: true, changeOrigin: true },
      },
    },
  };
});

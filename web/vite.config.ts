import { reactRouter } from "@react-router/dev/vite";
import tailwindcss from "@tailwindcss/vite";
import { defineConfig } from "vite";

export default defineConfig({
  plugins: [tailwindcss(), reactRouter()],
  resolve: {
    tsconfigPaths: true,
  },
  server: {
    // `bun run dev` serves the UI on :5173; RPCs go to a hub on :8080.
    proxy: {
      "/netwatch.v1.": "http://localhost:8080",
    },
  },
});

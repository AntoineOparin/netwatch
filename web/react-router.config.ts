import type { Config } from "@react-router/dev/config";

export default {
  // SPA mode: the Go hub serves the static build, there is no Node server.
  // `make web` copies build/client into internal/webui for embedding.
  ssr: false,
} satisfies Config;

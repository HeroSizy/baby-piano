import { defineConfig } from "vite";
import { cloudflare } from "@cloudflare/vite-plugin";

// public/ is copied verbatim (Vite's publicDir) as the Worker's static assets.
export default defineConfig({
  plugins: [cloudflare()],
});

import { bindings, defineConfig } from "cf/config";

// mashpiano.herosizy.dev: the static page in public/ (Vite copies it
// verbatim), and a few lines of Worker (src/worker.js) that send
// /download/<platform> to that file of the latest GitHub Release, so the
// page never needs a redeploy for a new version. Cloudflare's Git
// integration (Workers Builds, root directory site/) deploys it on every
// push to main that touches site/.
export default defineConfig({
  worker: {
    name: "mashpiano-site",
    compatibilityDate: "2026-09-01",
    entrypoint: "src/worker.js",
    workersDev: false,
    observability: { enabled: true },
    assets: {
      htmlHandling: "drop-trailing-slash",
      notFoundHandling: "404-page",
      runWorkerFirst: ["/download/*", "/api/*"],
    },
    domains: ["mashpiano.herosizy.dev"],
    env: { ASSETS: bindings.assets() },
  },
});

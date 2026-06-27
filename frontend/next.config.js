/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,

  // Produce a self-contained server bundle at .next/standalone/server.js so the edge
  // appliance can run `node server.js` with a minimal, pinned dependency tree
  // (no full node_modules / no `next start`). Assembled into artifacts\next by
  // laguna-escondida-edge/scripts/build-artifacts.ps1.
  output: "standalone",

  // Pin the output-file-tracing root to THIS project so server.js lands at
  // .next/standalone/server.js (not nested under a traced subpath) regardless of any
  // lockfiles in parent directories, and silence Next's multi-lockfile root warning.
  outputFileTracingRoot: __dirname,
}

module.exports = nextConfig

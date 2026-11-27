// The public pages take a card number, so they load and call only what they need: our origin,
// Google's sign-in and Wompi's API. The card goes to Wompi from the browser (connect-src).
// Next's own inline scripts need 'unsafe-inline'; its dev server also needs eval and a websocket.
const isDev = process.env.NODE_ENV !== 'production'
const publicPagesCsp = [
  "default-src 'self'",
  `script-src 'self' 'unsafe-inline'${isDev ? " 'unsafe-eval'" : ''} https://accounts.google.com`,
  "style-src 'self' 'unsafe-inline' https://accounts.google.com",
  "img-src 'self' data: https://*.googleusercontent.com",
  "font-src 'self' data:",
  `connect-src 'self' https://accounts.google.com https://production.wompi.co https://sandbox.wompi.co${isDev ? ' ws:' : ''}`,
  'frame-src https://accounts.google.com',
  "form-action 'self'",
  "base-uri 'self'",
  "object-src 'none'",
  "frame-ancestors 'none'",
].join('; ')
const publicPageHeaders = [
  { key: 'Content-Security-Policy', value: publicPagesCsp },
  { key: 'Referrer-Policy', value: 'strict-origin-when-cross-origin' },
]

/** @type {import('next').NextConfig} */
const nextConfig = {
  async headers() {
    return ['/signin', '/registro', '/registro/:path*', '/recuperar'].map((source) => ({ source, headers: publicPageHeaders }))
  },

  reactStrictMode: true,
  distDir: process.env.NEXT_DIST_DIR ?? '.next',

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

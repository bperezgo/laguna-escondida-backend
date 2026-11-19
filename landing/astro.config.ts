import tailwindcss from "@tailwindcss/vite";
import sitemap from "@astrojs/sitemap";
import { defineConfig } from "astro/config";
import icon from "astro-icon";

import { DEFAULT_SITE_URL } from "./src/config/defaults";

// The config runs before Vite reads .env, and the sitemap needs the site address now.
try {
  process.loadEnvFile(".env");
} catch {
  // No .env: the shell's environment is all there is.
}

export default defineConfig({
  site: process.env.PUBLIC_SITE_URL || DEFAULT_SITE_URL,
  output: "static",
  integrations: [sitemap(), icon()],
  vite: { plugins: [tailwindcss()] },
});

import { defineConfig } from "astro/config";
import UnoCSS from "unocss/astro";

import mdx from "@astrojs/mdx";

import sitemap from "@astrojs/sitemap";

// https://astro.build/config
export default defineConfig({
  site: "https://example.com",
  integrations: [
    UnoCSS({
      injectReset: false,
    }),
    mdx(),
    sitemap(),
  ],
  markdown: {
    shikiConfig: {
		theme: 'github-light',
	},
  },
});

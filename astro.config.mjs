import { defineConfig } from "astro/config";
import UnoCSS from "unocss/astro";
import mdx from "@astrojs/mdx";
import sitemap from "@astrojs/sitemap";

import react from "@astrojs/react";

// https://astro.build/config
export default defineConfig({
  site: "https://chaojie.fun",
  integrations: [UnoCSS({
    injectReset: false
  }), mdx(), sitemap(), react()],
  markdown: {
    shikiConfig: {
      theme: 'github-light'
    }
  }
});
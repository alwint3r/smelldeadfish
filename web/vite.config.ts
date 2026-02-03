import { defineConfig } from "vite";
import preact from "@preact/preset-vite";

export default defineConfig({
  plugins: [preact()],
  base: process.env.VITE_BASE ?? "/",
  server: {
    proxy: {
      "/api": "http://localhost:4318",
    },
  },
});

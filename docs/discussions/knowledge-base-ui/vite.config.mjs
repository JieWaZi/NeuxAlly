import { createRequire } from "node:module";
import path from "node:path";
import { fileURLToPath } from "node:url";

// 原型独立运行，沿用桌面前端已安装的依赖与组件。
const repo = fileURLToPath(new URL("../../../", import.meta.url));
const requireDesktop = createRequire(path.join(repo, "desktop/package.json"));
const { defineConfig } = await import(requireDesktop.resolve("vite"));
const { default: react } = await import(
  requireDesktop.resolve("@vitejs/plugin-react")
);
const { default: tailwindcss } = await import(
  requireDesktop.resolve("@tailwindcss/vite")
);

export default defineConfig({
  root: path.join(repo, "docs/discussions/knowledge-base-ui"),
  publicDir: path.join(repo, "desktop/public"),
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      "@": path.join(repo, "desktop/src"),
      react: path.join(repo, "desktop/node_modules/react"),
      "react-dom": path.join(repo, "desktop/node_modules/react-dom"),
      "lucide-react": path.join(repo, "desktop/node_modules/lucide-react"),
    },
  },
  server: {
    host: "127.0.0.1",
    port: 1421,
    strictPort: true,
    fs: { allow: [repo] },
  },
  build: {
    outDir: path.join(repo, ".scratch/knowledge-base-ui-dist"),
    emptyOutDir: true,
    rollupOptions: {
      input: {
        prototype: path.join(
          repo,
          "docs/discussions/knowledge-base-ui/index.html",
        ),
        gallery: path.join(
          repo,
          "docs/discussions/knowledge-base-ui/gallery.html",
        ),
      },
    },
  },
});

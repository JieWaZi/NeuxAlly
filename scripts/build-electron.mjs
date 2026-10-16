import { createRequire } from "node:module";
import path from "node:path";
import { fileURLToPath } from "node:url";

const desktop = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "../desktop",
);
const { build, context } = createRequire(path.join(desktop, "package.json"))(
  "esbuild",
);

function electronBuildOptions(onRebuild) {
  return {
    absWorkingDir: desktop,
    entryPoints: {
      main: "electron/main/index.ts",
      preload: "electron/preload/index.ts",
    },
    outdir: "dist-electron",
    outExtension: { ".js": ".cjs" },
    bundle: true,
    platform: "node",
    format: "cjs",
    target: "node22",
    external: ["electron", "node-pty"],
    sourcemap: true,
    plugins: onRebuild
      ? [
          {
            name: "electron-dev-rebuild",
            setup(build) {
              build.onEnd((result) => {
                if (!result.errors.length) onRebuild();
              });
            },
          },
        ]
      : [],
  };
}

export async function buildElectron() {
  await build(electronBuildOptions());
}

export async function watchElectron(onRebuild) {
  let initial = true;
  const ctx = await context(
    electronBuildOptions(() => {
      if (initial) {
        initial = false;
        return;
      }
      onRebuild();
    }),
  );
  await ctx.watch();
  return ctx;
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  await buildElectron();
}

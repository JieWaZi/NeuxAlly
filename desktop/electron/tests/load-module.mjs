import { buildSync } from "esbuild";
import { createRequire } from "node:module";
import path from "node:path";
import { fileURLToPath } from "node:url";
const desktop = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "../..",
);
const require = createRequire(path.join(desktop, "package.json"));
export function loadModule(entry, overrides = {}, window = {}) {
  const source = buildSync({
    entryPoints: [path.join(desktop, "electron", entry)],
    bundle: true,
    platform: "node",
    format: "cjs",
    packages: "external",
    write: false,
  }).outputFiles[0].text;
  const module = { exports: {} };
  new Function("require", "module", "exports", "window", source)(
    (name) => (name in overrides ? overrides[name] : require(name)),
    module,
    module.exports,
    window,
  );
  return module.exports;
}

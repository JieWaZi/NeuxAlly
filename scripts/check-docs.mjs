import { readdir, readFile, access } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const ignored = new Set([
  ".git",
  "node_modules",
  ".scratch",
  ".agents",
  "dist",
  "dist-electron",
  "release",
  "binaries",
  "coverage",
]);
async function markdownFiles(directory) {
  const files = [];
  for (const entry of await readdir(directory, { withFileTypes: true })) {
    if (ignored.has(entry.name)) continue;
    const target = path.join(directory, entry.name);
    if (entry.isDirectory()) files.push(...(await markdownFiles(target)));
    else if (entry.isFile() && entry.name.endsWith(".md")) files.push(target);
  }
  return files;
}
const failures = [];
for (const file of await markdownFiles(root)) {
  const text = await readFile(file, "utf8");
  const relative = path.relative(root, file);
  if (/\/Users\/[\w.-]+\//.test(text))
    failures.push(`${relative}: 包含个人目录`);
  const links = [
    ...text.matchAll(/!?\[[^\]]*\]\(([^\s)]+)(?:\s+[^)]*)?\)/g),
    ...text.matchAll(/<img\b[^>]*\bsrc="([^"]+)"/g),
  ];
  for (const match of links) {
    const target = match[1]?.replace(/^<|>$/g, "").split("#")[0];
    if (!target || /^[a-z][a-z\d+.-]*:/i.test(target)) continue;
    try {
      await access(
        path.resolve(path.dirname(file), decodeURIComponent(target)),
      );
    } catch {
      failures.push(`${relative}: 链接不可用 ${target}`);
    }
  }
}
if (failures.length) throw new Error(failures.join("\n"));
console.log("文档链接、图片与个人路径检查通过");

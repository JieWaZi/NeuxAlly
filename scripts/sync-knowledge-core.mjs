import { createHash } from "node:crypto";
import {
  access,
  mkdir,
  readFile,
  readdir,
  rename,
  rm,
  writeFile,
} from "node:fs/promises";
import path from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const defaultTarget = path.join(root, "third_party", "neux-knowledge");
const manifestName = "ALLY_SOURCE_SNAPSHOT.json";
const allowedRoots = new Set([
  "core",
  "internal",
  "rules",
  "remote",
  "httpapi",
  "cmd",
  "docs",
  "upstream",
  "licenses",
]);
const rootFiles = new Set([
  "go.mod",
  "go.sum",
  "LICENSE",
  "README.md",
  "README.zh-CN.md",
  "THIRD_PARTY_NOTICES.md",
  "AGENTS.md",
]);
const allowedExtensions = new Set([
  ".go",
  ".md",
  ".json",
  ".txt",
  ".yaml",
  ".yml",
]);
const sha256 = (bytes) => createHash("sha256").update(bytes).digest("hex");

async function walk(directory, relative = "", filter = false) {
  const files = [];
  for (const entry of await readdir(path.join(directory, relative), {
    withFileTypes: true,
  })) {
    if (filter && entry.name.startsWith(".")) continue;
    const name = path.posix.join(relative, entry.name);
    if (entry.isSymbolicLink()) throw new Error(`快照不接受符号链接：${name}`);
    if (entry.isDirectory()) {
      if (filter && !relative && !allowedRoots.has(entry.name)) continue;
      files.push(...(await walk(directory, name, filter)));
    } else if (entry.isFile()) {
      if (filter && !relative && !rootFiles.has(entry.name)) continue;
      if (
        filter &&
        relative &&
        !allowedExtensions.has(path.extname(entry.name)) &&
        !entry.name.toUpperCase().includes("LICENSE") &&
        !entry.name.toUpperCase().startsWith("COPYING")
      )
        continue;
      files.push(name);
    }
  }
  return files.sort();
}

export async function syncKnowledgeCore(source, target = defaultTarget) {
  source = path.resolve(source);
  target = path.resolve(target);
  if (
    source === target ||
    target.startsWith(source + path.sep) ||
    source.startsWith(target + path.sep)
  )
    throw new Error("来源与快照目录必须隔离");
  const module = await readFile(path.join(source, "go.mod"), "utf8");
  if (!/^module github\.com\/JieWaZi\/neux-knowledge\s*$/m.test(module))
    throw new Error("来源不是约定的独立知识核心");
  let exists = false;
  try {
    await access(target);
    exists = true;
  } catch (error) {
    if (error.code !== "ENOENT") throw error;
  }
  if (exists) {
    const old = JSON.parse(
      await readFile(path.join(target, manifestName), "utf8"),
    );
    if (old.generator !== "scripts/sync-knowledge-core.mjs")
      throw new Error("目标不是本脚本管理的快照");
  }
  const files = [];
  for (const name of await walk(source, "", true)) {
    const bytes = await readFile(path.join(source, name));
    files.push({ path: name, sha256: sha256(bytes), bytes });
  }
  const entries = files.map(({ path, sha256 }) => ({ path, sha256 }));
  const manifest = {
    generator: "scripts/sync-knowledge-core.mjs",
    module: "github.com/JieWaZi/neux-knowledge",
    schemaVersion: 1,
    digest: sha256(JSON.stringify(entries)),
    files: entries,
  };
  // 内容先完整写入临时目录，校验后替换受管快照，不复制 Git、数据库或本机配置。
  const temporary = target + ".tmp-" + process.pid;
  const backup = target + ".backup-" + process.pid;
  await mkdir(path.dirname(target), { recursive: true });
  await mkdir(temporary);
  try {
    for (const file of files) {
      const destination = path.join(temporary, file.path);
      await mkdir(path.dirname(destination), { recursive: true });
      await writeFile(destination, file.bytes);
    }
    await writeFile(
      path.join(temporary, manifestName),
      JSON.stringify(manifest, null, 2) + "\n",
    );
    await verifyKnowledgeCore(temporary);
    if (exists) await rename(target, backup);
    try {
      await rename(temporary, target);
    } catch (error) {
      if (exists) await rename(backup, target);
      throw error;
    }
    if (exists) await rm(backup, { recursive: true });
  } finally {
    await rm(temporary, { recursive: true, force: true });
  }
  return manifest;
}

export async function verifyKnowledgeCore(target = defaultTarget) {
  const manifest = JSON.parse(
    await readFile(path.join(target, manifestName), "utf8"),
  );
  if (
    manifest.schemaVersion !== 1 ||
    manifest.generator !== "scripts/sync-knowledge-core.mjs" ||
    !Array.isArray(manifest.files)
  )
    throw new Error("无效快照清单");
  if (sha256(JSON.stringify(manifest.files)) !== manifest.digest)
    throw new Error("快照清单摘要不匹配");
  const paths = new Set(manifest.files.map((file) => file.path));
  if (paths.size !== manifest.files.length)
    throw new Error("快照清单包含重复文件");
  for (const file of manifest.files) {
    if (path.isAbsolute(file.path) || file.path.split(/[\\/]/).includes(".."))
      throw new Error("快照清单路径越界");
    if (sha256(await readFile(path.join(target, file.path))) !== file.sha256)
      throw new Error(`快照内容不匹配：${file.path}`);
  }
  for (const file of await walk(target)) {
    if (file !== manifestName && !paths.has(file))
      throw new Error(`快照包含额外文件：${file}`);
  }
  return manifest;
}

if (
  process.argv[1] &&
  import.meta.url === pathToFileURL(path.resolve(process.argv[1])).href
) {
  const argument = process.argv[2];
  if (argument === "--check") {
    const result = await verifyKnowledgeCore();
    console.log(
      `知识核心快照校验通过：${result.files.length} 个文件，${result.digest}`,
    );
  } else {
    if (!argument || process.argv.length !== 3)
      throw new Error(
        "用法：node scripts/sync-knowledge-core.mjs <独立仓库路径> 或 --check",
      );
    const result = await syncKnowledgeCore(argument);
    console.log(
      `知识核心快照已生成：${result.files.length} 个文件，${result.digest}`,
    );
  }
}

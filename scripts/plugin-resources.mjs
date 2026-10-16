import { createHash } from "node:crypto";
import { execFileSync } from "node:child_process";
import { createRequire } from "node:module";
import {
  chmod,
  mkdir,
  mkdtemp,
  readFile,
  readdir,
  stat,
  rename,
  rm,
  writeFile,
} from "node:fs/promises";
import path from "node:path";
import os from "node:os";
import { fileURLToPath, pathToFileURL } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const sourceRoot = path.join(root, "plugin-sources");
const requireDesktop = createRequire(path.join(root, "desktop/package.json"));
const hash = (bytes) => createHash("sha256").update(bytes).digest("hex");

// 固定来源和每个平台资产均需有完整身份，运行时不解析远端分支。
export async function loadLock(input) {
  const lock =
    input ??
    JSON.parse(
      await readFile(path.join(sourceRoot, "source.lock.json"), "utf8"),
    );
  if (
    lock.schemaVersion !== 1 ||
    !/^[-a-z0-9.]+\.bundle$/.test(lock.source?.bundle ?? "") ||
    !/^[a-f0-9]{40}$/.test(lock.source?.commit ?? "") ||
    !/^[a-f0-9]{64}$/.test(lock.source?.sha256 ?? "") ||
    lock.provider?.name !== "feishu" ||
    lock.provider?.version !== "1.0.98" ||
    !/^[a-f0-9]{40}$/.test(lock.provider?.upstreamCommit ?? "")
  )
    throw new Error("插件固定来源缺失或无效");
  for (const target of [
    "darwin-amd64",
    "darwin-arm64",
    "windows-amd64",
    "windows-arm64",
    "linux-amd64",
    "linux-arm64",
  ]) {
    const asset = lock.provider.assets?.[target];
    if (
      !asset ||
      !/^[a-f0-9]{64}$/.test(asset.sha256) ||
      !/^lark-cli-1\.0\.98-[a-z0-9-]+\.(tar\.gz|zip)$/.test(asset.file) ||
      asset.url !==
        `https://github.com/larksuite/cli/releases/download/v1.0.98/${asset.file}`
    )
      throw new Error("插件程序固定来源无效");
  }
  return lock;
}
export function verifyDigest(bytes, expected) {
  if (hash(bytes) !== expected) throw new Error("插件资产摘要不一致");
  return bytes;
}
export function selectAsset(lock, target) {
  const aliases = {
    "darwin-x64": "darwin-amd64",
    "win32-x64": "windows-amd64",
    "win32-arm64": "windows-arm64",
    "linux-x64": "linux-amd64",
    "aarch64-apple-darwin": "darwin-arm64",
    "x86_64-apple-darwin": "darwin-amd64",
    "aarch64-pc-windows-msvc": "windows-arm64",
    "x86_64-pc-windows-msvc": "windows-amd64",
  };
  const key = aliases[target] ?? target;
  const asset = lock.provider.assets[key];
  if (!asset) throw new Error(`不支持的插件程序目标：${target}`);
  return { ...asset, target: key };
}

// 固定源打成标准 ZIP，保留 Unix 执行位和完整相对目录。
async function packageZIP(directory) {
  const files = {};
  async function collect(relative = "") {
    for (const entry of await readdir(path.join(directory, relative), {
      withFileTypes: true,
    })) {
      const name = relative ? `${relative}/${entry.name}` : entry.name;
      if (entry.isDirectory()) {
        await collect(name);
      } else if (entry.isFile()) {
        const location = path.join(directory, name);
        const mode = (await stat(location)).mode;
        files[name] = [
          new Uint8Array(await readFile(location)),
          { os: 3, attrs: (((mode & 0o777) | 0o100000) << 16) >>> 0 },
        ];
      } else {
        throw new Error("固定插件源不能包含符号链接或特殊文件");
      }
    }
  }
  await collect();
  return Buffer.from(requireDesktop("fflate").zipSync(files));
}

// Git bundle 是独立源仓库的固定提交快照，只读取内容，不执行源仓库脚本。
export async function prepareCatalog(output) {
  const lock = await loadLock();
  const bundle = path.join(sourceRoot, lock.source.bundle);
  verifyDigest(await readFile(bundle), lock.source.sha256);
  const temporary = await mkdtemp(
    path.join(os.tmpdir(), "neuxally-plugin-source-"),
  );
  try {
    const checkout = path.join(temporary, "source");
    execFileSync(
      "git",
      ["clone", "--quiet", "--no-checkout", bundle, checkout],
      { stdio: "pipe" },
    );
    const archive = execFileSync(
      "git",
      ["-C", checkout, "archive", "--format=tar", lock.source.commit],
      { maxBuffer: 32 * 1024 * 1024 },
    );
    const archivePath = path.join(temporary, "source.tar");
    await writeFile(archivePath, archive);
    const files = path.join(temporary, "files");
    await mkdir(files);
    execFileSync("tar", ["-xf", archivePath, "-C", files], { stdio: "pipe" });
    const source = JSON.parse(
      await readFile(path.join(files, "catalog.json"), "utf8"),
    );
    const metadata = JSON.parse(
      execFileSync(
        "go",
        [
          "run",
          "./cmd/plugin-package",
          ...source.map((entry) => path.join(files, entry.directory)),
        ],
        {
          cwd: path.join(root, "server"),
          encoding: "utf8",
          maxBuffer: 4 * 1024 * 1024,
        },
      ),
    );
    const seen = new Set();
    const entries = [];
    for (const entry of source) {
      if (
        !/^[a-z0-9-]+$/.test(entry.key) ||
        entry.directory !== `plugins/${entry.key}` ||
        seen.has(entry.key)
      )
        throw new Error("插件目录来源无效");
      seen.add(entry.key);
      const directory = path.join(files, entry.directory);
      const manifest = JSON.parse(
        await readFile(path.join(directory, "plugin.json"), "utf8"),
      );
      const provenance = JSON.parse(
        await readFile(path.join(directory, "provenance.json"), "utf8"),
      );
      if (
        manifest.$schema !==
          "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json" ||
        provenance.upstream?.commit !== lock.provider.upstreamCommit ||
        provenance.upstream?.license !== "MIT"
      )
        throw new Error("插件清单或来源声明不完整");
      await readFile(path.join(directory, "LICENSE.larksuite-cli"));
      const destination = `catalog/${entry.key}.zip`;
      const bytes = await packageZIP(directory);
      await mkdir(path.dirname(path.join(output, destination)), {
        recursive: true,
      });
      await writeFile(path.join(output, destination), bytes);
      entries.push({
        key: entry.key,
        directory: destination,
        sourceCommit: lock.source.commit,
        archiveSHA256: hash(bytes),
        package: metadata[entries.length],
      });
    }
    const catalog = {
      schemaVersion: 1,
      sourceCommit: lock.source.commit,
      entries,
    };
    await mkdir(output, { recursive: true });
    await writeFile(
      path.join(output, "catalog.json"),
      JSON.stringify(catalog, null, 2) + "\n",
    );
    return catalog;
  } finally {
    await rm(temporary, { recursive: true, force: true });
  }
}

// 下载只发生于开发或打包阶段；校验完整压缩资产后再提取单个原生程序。
export async function prepareProvider(output, target, options = {}) {
  const lock = await loadLock();
  const asset = selectAsset(lock, target);
  const cache = options.cache ?? path.join(root, ".scratch/plugin-assets");
  await mkdir(cache, { recursive: true });
  const archivePath = path.join(cache, asset.file);
  let bytes = await readFile(archivePath).catch(() => null);
  if (bytes) {
    verifyDigest(bytes, asset.sha256);
  } else {
    if (options.offline) throw new Error("离线模式缺少固定插件程序资产");
    const response = await fetch(asset.url);
    if (!response.ok) throw new Error(`获取插件资产失败：${response.status}`);
    bytes = Buffer.from(await response.arrayBuffer());
    verifyDigest(bytes, asset.sha256);
    await writeFile(archivePath, bytes);
  }
  const binaryName = asset.target.startsWith("windows-")
    ? "lark-cli.exe"
    : "lark-cli";
  let binary;
  let license;
  if (asset.file.endsWith(".zip")) {
    const { unzipSync } = requireDesktop("fflate");
    const files = unzipSync(bytes);
    const names = Object.keys(files).filter(
      (name) => path.posix.basename(name) === binaryName,
    );
    if (names.length !== 1) throw new Error("插件资产缺少唯一程序");
    binary = Buffer.from(files[names[0]]);
    const licenseName = Object.keys(files).find(
      (name) => path.posix.basename(name) === "LICENSE",
    );
    if (!licenseName) throw new Error("插件资产缺少许可证");
    license = Buffer.from(files[licenseName]);
  } else {
    const names = execFileSync("tar", ["-tzf", archivePath], {
      encoding: "utf8",
    })
      .trim()
      .split("\n")
      .filter((name) => path.posix.basename(name) === binaryName);
    if (names.length !== 1) throw new Error("插件资产缺少唯一程序");
    binary = execFileSync("tar", ["-xOf", archivePath, names[0]], {
      maxBuffer: 100 * 1024 * 1024,
    });
    const licenseName = execFileSync("tar", ["-tzf", archivePath], {
      encoding: "utf8",
    })
      .trim()
      .split("\n")
      .find((name) => path.posix.basename(name) === "LICENSE");
    if (!licenseName) throw new Error("插件资产缺少许可证");
    license = execFileSync("tar", ["-xOf", archivePath, licenseName]);
  }
  const destination = path.join(
    output,
    "providers",
    lock.provider.name,
    lock.provider.version,
    asset.target,
  );
  await mkdir(destination, { recursive: true });
  const staging = path.join(destination, `${binaryName}.tmp`);
  await writeFile(staging, binary);
  await chmod(staging, 0o755);
  await rename(staging, path.join(destination, binaryName));
  await writeFile(path.join(destination, "LICENSE"), license);
  await writeFile(
    path.join(destination, "provenance.json"),
    JSON.stringify(
      {
        upstreamCommit: lock.provider.upstreamCommit,
        url: asset.url,
        archiveSHA256: asset.sha256,
        binarySHA256: hash(binary),
      },
      null,
      2,
    ) + "\n",
  );
  return path.join(destination, binaryName);
}

export async function verifyPrepared(output, target) {
  const lock = await loadLock();
  const asset = selectAsset(lock, target);
  const directory = path.join(
    output,
    "providers",
    lock.provider.name,
    lock.provider.version,
    asset.target,
  );
  const provenance = JSON.parse(
    await readFile(path.join(directory, "provenance.json"), "utf8"),
  );
  if (
    provenance.archiveSHA256 !== asset.sha256 ||
    provenance.url !== asset.url ||
    provenance.upstreamCommit !== lock.provider.upstreamCommit
  )
    throw new Error("程序来源和固定清单不一致");
  verifyDigest(
    await readFile(
      path.join(
        directory,
        asset.target.startsWith("windows-") ? "lark-cli.exe" : "lark-cli",
      ),
    ),
    provenance.binarySHA256,
  );
}

if (
  process.argv[1] &&
  import.meta.url === pathToFileURL(path.resolve(process.argv[1])).href
) {
  const checking = process.argv.includes("--check");
  const output = checking
    ? await mkdtemp(path.join(os.tmpdir(), "plugin-check-"))
    : path.join(root, "desktop/resources/plugins");
  try {
    await prepareCatalog(output);
    if (!checking) {
      const target =
        process.argv.find(
          (value, index) => index > 1 && !value.startsWith("--"),
        ) ?? `${process.platform}-${process.arch}`;
      await prepareProvider(output, target, {
        offline: process.argv.includes("--offline"),
      });
      await verifyPrepared(output, target);
    }
    console.log(checking ? "固定插件源与目录检查通过" : "固定插件资源准备完成");
  } finally {
    if (checking) await rm(output, { recursive: true, force: true });
  }
}

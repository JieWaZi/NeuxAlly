import { buildGitHubProvider } from "./build-github-provider.mjs";
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
    lock.schemaVersion !== 2 ||
    !/^[-a-z0-9.]+\.bundle$/.test(lock.source?.bundle ?? "") ||
    !/^[a-f0-9]{40}$/.test(lock.source?.commit ?? "") ||
    !/^[a-f0-9]{64}$/.test(lock.source?.sha256 ?? "") ||
    !Array.isArray(lock.providers) ||
    lock.providers.length !== 2
  )
    throw new Error("插件固定来源缺失或无效");
  const expected = {
    feishu: {
      version: "1.0.98",
      repository: "larksuite/cli",
      binary: "lark-cli",
      licenseFile: "LICENSE.larksuite-cli",
    },
    github: {
      version: "2.102.0",
      repository: "cli/cli",
      binary: "gh",
      licenseFile: "LICENSE.github-cli",
    },
  };
  const seen = new Set();
  for (const provider of lock.providers) {
    const spec = expected[provider.name];
    if (
      !spec ||
      seen.has(provider.name) ||
      Object.entries(spec).some(([key, value]) => provider[key] !== value) ||
      !/^[a-f0-9]{40}$/.test(provider.upstreamCommit ?? "")
    )
      throw new Error("插件程序固定来源无效");
    seen.add(provider.name);
    if (provider.name === "github") {
      const archive = provider.source;
      const build = provider.build;
      if (
        !archive ||
        archive.url !==
          `https://github.com/cli/cli/archive/${provider.upstreamCommit}.tar.gz` ||
        archive.file !== `github-cli-${provider.upstreamCommit}.tar.gz` ||
        !/^[a-f0-9]{64}$/.test(archive.sha256) ||
        build?.toolchain !== "go1.27.1" ||
        build.version !== "2.102.0-neuxally.1" ||
        build.patch !== "patches/github-keyring-isolation.patch" ||
        build.verification !== "patches/github-keyring-isolation_test.go" ||
        !/^[a-f0-9]{64}$/.test(build.patchSHA256) ||
        !/^[a-f0-9]{64}$/.test(build.verificationSHA256)
      )
        throw new Error("GitHub 隔离构建固定来源无效");
      verifyDigest(
        await readFile(path.join(sourceRoot, build.patch)),
        build.patchSHA256,
      );
      verifyDigest(
        await readFile(path.join(sourceRoot, build.verification)),
        build.verificationSHA256,
      );
      continue;
    }
    for (const target of [
      "darwin-amd64",
      "darwin-arm64",
      "windows-amd64",
      "windows-arm64",
      "linux-amd64",
      "linux-arm64",
    ]) {
      const asset = provider.assets?.[target];
      const [platform, arch] = target.split("-");
      const file =
        provider.name === "feishu"
          ? `lark-cli-1.0.98-${target}.${platform === "windows" ? "zip" : "tar.gz"}`
          : `gh_2.102.0_${platform === "darwin" ? "macOS" : platform}_${arch}.${platform === "linux" ? "tar.gz" : "zip"}`;
      if (
        !asset ||
        !/^[a-f0-9]{64}$/.test(asset.sha256) ||
        asset.file !== file ||
        asset.url !==
          `https://github.com/${provider.repository}/releases/download/v${provider.version}/${file}`
      )
        throw new Error("插件程序固定来源无效");
    }
  }
  return lock;
}
export function verifyDigest(bytes, expected) {
  if (hash(bytes) !== expected) throw new Error("插件资产摘要不一致");
  return bytes;
}
export function selectAsset(lock, target, name = "feishu") {
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
  const provider = lock.providers.find((provider) => provider.name === name);
  const asset =
    name === "github" &&
    [
      "darwin-amd64",
      "darwin-arm64",
      "windows-amd64",
      "windows-arm64",
      "linux-amd64",
      "linux-arm64",
    ].includes(key)
      ? provider?.source
      : provider?.assets?.[key];
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
      const provider = lock.providers.find(
        (provider) => provider.name === entry.provider,
      );
      const provenance = JSON.parse(
        await readFile(path.join(directory, "provenance.json"), "utf8"),
      );
      if (
        manifest.$schema !==
          "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json" ||
        provenance.upstream?.commit !== provider?.upstreamCommit ||
        provenance.upstream?.license !== "MIT"
      )
        throw new Error("插件清单或来源声明不完整");
      await readFile(path.join(directory, provider.licenseFile));
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
  const programs = [];
  for (const provider of lock.providers) {
    const asset = selectAsset(lock, target, provider.name);
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
      ? `${provider.binary}.exe`
      : provider.binary;
    let binary;
    let license;
    if (provider.build) {
      const built = await buildGitHubProvider({
        archive: archivePath,
        provider,
        target: asset.target,
        sourceRoot,
        offline: options.offline,
      });
      binary = built.binary;
      license = built.license;
    } else if (asset.file.endsWith(".zip")) {
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
      provider.name,
      provider.version,
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
          upstreamCommit: provider.upstreamCommit,
          url: asset.url,
          archiveSHA256: asset.sha256,
          binarySHA256: hash(binary),
          ...(provider.build ? { build: provider.build } : {}),
        },
        null,
        2,
      ) + "\n",
    );
    programs.push(path.join(destination, binaryName));
  }
  return programs;
}

export async function verifyPrepared(output, target) {
  const lock = await loadLock();
  for (const provider of lock.providers) {
    const asset = selectAsset(lock, target, provider.name);
    const directory = path.join(
      output,
      "providers",
      provider.name,
      provider.version,
      asset.target,
    );
    const provenance = JSON.parse(
      await readFile(path.join(directory, "provenance.json"), "utf8"),
    );
    if (
      provenance.archiveSHA256 !== asset.sha256 ||
      provenance.url !== asset.url ||
      provenance.upstreamCommit !== provider.upstreamCommit ||
      (provider.build &&
        JSON.stringify(provenance.build) !== JSON.stringify(provider.build))
    )
      throw new Error("程序来源和固定清单不一致");
    verifyDigest(
      await readFile(
        path.join(
          directory,
          asset.target.startsWith("windows-")
            ? `${provider.binary}.exe`
            : provider.binary,
        ),
      ),
      provenance.binarySHA256,
    );
  }
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

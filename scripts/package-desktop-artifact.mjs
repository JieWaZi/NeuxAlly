import { execFileSync } from "node:child_process";
import {
  lstat,
  mkdtemp,
  readdir,
  readlink,
  rm,
  stat,
  symlink,
  writeFile,
} from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const release = path.join(root, "desktop/release");
const bundle = path.join(
  release,
  process.arch === "arm64" ? "mac-arm64" : "mac",
  "NeuxAlly.app",
);
const mib = 1024 * 1024;
const budgets = {
  application: 450 * mib,
  // 原有 Core/CLI 与前端预算外，增加约 47 MiB 的固定飞书程序和完整技能；保留约 16 MiB 余量。
  resources: 175 * mib,
  installer: 150 * mib,
};

async function sizeOf(file) {
  const info = await lstat(file);
  if (info.isSymbolicLink()) return 0;
  if (!info.isDirectory()) return info.size;
  const sizes = await Promise.all(
    (await readdir(file)).map((name) => sizeOf(path.join(file, name))),
  );
  return sizes.reduce((sum, size) => sum + size, 0);
}

function check(label, size, limit) {
  console.log(
    `${label}: ${(size / mib).toFixed(1)} MiB / ${(limit / mib).toFixed(0)} MiB`,
  );
  if (size > limit) throw new Error(`${label} exceeds release size budget`);
}

async function removeLegacyApplicationsLink(file) {
  try {
    const info = await lstat(file);
    if (!info.isSymbolicLink() || (await readlink(file)) !== "/Applications") {
      throw new Error(
        "Installer output contains an unexpected Applications entry",
      );
    }
    await rm(file);
  } catch (error) {
    if (error?.code !== "ENOENT") throw error;
  }
}

const report = path.join(release, `size-report-${process.arch}.json`);
await rm(report, { force: true });
const application = await sizeOf(bundle);
const resources = await sizeOf(path.join(bundle, "Contents/Resources"));
check("Installed application", application, budgets.application);
check("Application resources", resources, budgets.resources);
const source = path.dirname(bundle);
await removeLegacyApplicationsLink(path.join(source, "Applications"));
const entries = await readdir(source);
if (entries.some((name) => name !== "NeuxAlly.app")) {
  throw new Error("Installer source contains unexpected build artifacts");
}
const installer = path.join(release, `NeuxAlly-mac-${process.arch}.dmg`);
const staging = await mkdtemp(path.join(tmpdir(), `ally-dmg-${process.arch}-`));
try {
  const stagedBundle = path.join(staging, "NeuxAlly.app");
  // DMG 专用内容位于仓库外，避免 Applications 链接被 Vite 或其他工具遍历。
  execFileSync("/usr/bin/ditto", [bundle, stagedBundle], { stdio: "inherit" });
  execFileSync(
    "/usr/bin/codesign",
    ["--verify", "--deep", "--strict", stagedBundle],
    { stdio: "inherit" },
  );
  await symlink("/Applications", path.join(staging, "Applications"));
  // 在签名后生成分架构安装镜像，避免将不同架构及其他构建产物重复装入。
  execFileSync(
    "/usr/bin/hdiutil",
    [
      "create",
      "-volname",
      "NeuxAlly",
      "-srcfolder",
      staging,
      "-format",
      "ULMO",
      "-ov",
      installer,
    ],
    { stdio: "inherit" },
  );
} finally {
  await rm(staging, { recursive: true, force: true });
}
execFileSync("/usr/bin/hdiutil", ["verify", installer], { stdio: "inherit" });
const installerSize = (await stat(installer)).size;
check("Installer", installerSize, budgets.installer);
await writeFile(
  report,
  JSON.stringify(
    {
      architecture: process.arch,
      bytes: { application, resources, installer: installerSize },
      budgets,
    },
    null,
    2,
  ) + "\n",
);

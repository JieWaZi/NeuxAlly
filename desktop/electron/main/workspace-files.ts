import { promises as fs } from "node:fs";
import path from "node:path";
import { homedir } from "node:os";
import { execFile } from "node:child_process";
import { randomUUID } from "node:crypto";
import { promisify } from "node:util";
import type {
  FileReferenceRequest,
  FileReferenceResolution,
  WorkspaceFileContent,
  WorkspaceDirectoryEntry,
  WorkspaceFileEntry,
  ChatWorkspaceFiles,
  WorkspaceGitBranch,
  WorkspaceGitBranches,
  WorkspaceGitCheckoutPreview,
  WorkspaceGitCheckoutResult,
  WorkspaceGitChanges,
  WorkspaceGitRepositories,
} from "../shared/contracts";

const execute = promisify(execFile);
const textLimit = 10 * 1024 * 1024;
const imageLimit = 50 * 1024 * 1024;
const hidden = new Set([".DS_Store", "Thumbs.db"]);
const removedChatFilesName = "removed-chat-files.json";
const fileSearchMaxEntries = 50_000;
const fileSearchCacheLimit = 8;
const fileSearchExcludedDirectories = new Set([
  ".git",
  "node_modules",
  "dist",
  "build",
  "out",
  "coverage",
  "target",
  ".next",
  ".nuxt",
  ".turbo",
  ".venv",
  "__pycache__",
  ".pytest_cache",
  ".mypy_cache",
  ".gradle",
]);
type IndexedFile = Pick<WorkspaceFileEntry, "name" | "relativePath">;
type FileSearchIndex = {
  root: string;
  scopePath: string;
  settled?: { files: IndexedFile[]; startedAt: number };
  generation?: Promise<IndexedFile[]>;
  invalidations: number;
};
const fileSearchIndexes = new Map<string, FileSearchIndex>();

/** 文件路径变化后使相关索引失效；旧结果会在后台重建期间继续可用。 */
export function invalidateFileSearch(root: string): void {
  for (const index of fileSearchIndexes.values()) {
    if (index.root === root) index.invalidations++;
  }
}
async function removedChatFilesPath(
  root: string,
  create = false,
): Promise<string> {
  const canonicalRoot = await canonicalDirectory(root);
  if (create)
    await fs.mkdir(path.join(canonicalRoot, "attachments"), {
      recursive: true,
    });
  const directory = await workspacePath(root, "attachments");
  if (!within(canonicalRoot, directory))
    throw new Error("会话附件目录超出工作区");
  return path.join(directory, removedChatFilesName);
}
/** 账号配置由原生 CLI 消费，不作为可预览或附加到消息的工作区文件。 */
function isAccountConfiguration(target: string): boolean {
  return /(?:^|[\\/])runtime[\\/]accounts(?:[\\/]|$)/.test(target);
}
export function within(root: string, target: string): boolean {
  const relative = path.relative(root, target);
  return (
    relative === "" ||
    (!relative.startsWith(`..${path.sep}`) &&
      relative !== ".." &&
      !path.isAbsolute(relative))
  );
}
export async function canonicalDirectory(root: string): Promise<string> {
  const resolved = await fs.realpath(root);
  if (!(await fs.stat(resolved)).isDirectory())
    throw new Error("工作区路径不是目录");
  return resolved;
}
function relativeInput(value: string): string {
  if (
    path.isAbsolute(value) ||
    value.split(/[\\/]/).includes("..") ||
    value.includes("\0")
  )
    throw new Error("工作区相对路径不合法");
  return value;
}
async function resolveMissing(target: string): Promise<string> {
  try {
    return await fs.realpath(target);
  } catch (error) {
    if (
      !(error instanceof Error) ||
      !("code" in error) ||
      error.code !== "ENOENT"
    )
      throw error;
    const parent = path.dirname(target);
    if (parent === target) throw error;
    return path.join(await resolveMissing(parent), path.basename(target));
  }
}
/** 项目入口可以指向冻结的真实目录，入口内部的软链接仍须通过范围校验。 */
export async function workspacePath(
  root: string,
  relativePath: string,
  allowMissing = false,
): Promise<string> {
  const canonical = await canonicalDirectory(root);
  const relative = relativeInput(relativePath);
  const target = await (allowMissing
    ? resolveMissing(path.join(canonical, relative))
    : fs.realpath(path.join(canonical, relative)));
  if (
    isAccountConfiguration(path.join(canonical, relative)) ||
    isAccountConfiguration(target)
  )
    throw new Error("账号配置不属于工作区文件");
  if (within(canonical, target)) return target;
  const parts = relative.split(/[\\/]/).filter((p) => p && p !== ".");
  if (parts[0] === "projects") {
    const projects = await canonicalDirectory(path.join(canonical, "projects"));
    // 成员 cwd 通过相对软链接访问共享 projects/，解析后已越出 canonical。
    if (parts.length === 1) return projects;
    if (parts[1]) {
      const project = await canonicalDirectory(path.join(projects, parts[1]));
      if (within(project, target)) return target;
    }
  }
  throw new Error("工作区条目超出根目录");
}
export async function resolveFileReference(
  request: FileReferenceRequest,
): Promise<FileReferenceResolution> {
  const directory = (value: string) =>
    canonicalDirectory(value).catch(() => "");
  const workspaceRoot = await directory(request.workspaceRoot);
  const baseDirectory = await directory(request.baseDirectory);
  const sourceRoots = [
    ...new Set(
      (await Promise.all(request.sourceRoots.map(directory))).filter(Boolean),
    ),
  ];
  const input = request.path.trim().replace(/^~(?=$|[/\\])/, homedir());
  const existing = async (p: string) => {
    try {
      const r = await fs.realpath(p);
      const stat = await fs.stat(r);
      return stat.isFile() || stat.isDirectory() ? r : undefined;
    } catch {
      return undefined;
    }
  };
  let target = await existing(
    path.isAbsolute(input)
      ? input
      : path.join(baseDirectory || workspaceRoot, input),
  );
  if (!target && !path.isAbsolute(input)) {
    const candidates = [
      ...new Set(
        (
          await Promise.all(
            sourceRoots.map((root) => existing(path.join(root, input))),
          )
        ).filter((p): p is string => p !== undefined),
      ),
    ];
    if (candidates.length > 1) return { status: "ambiguous" };
    target = candidates[0];
  }
  if (!target) return { status: "missing" };
  if (isAccountConfiguration(target)) return { status: "outside_scope" };
  if (
    ![workspaceRoot, ...sourceRoots]
      .filter(Boolean)
      .some((root) => within(root, target))
  )
    return { status: "outside_scope" };
  return {
    status: (await fs.stat(target)).isDirectory() ? "directory" : "resolved",
    canonicalPath: target,
    baseDirectory,
    workspaceRoot,
    sourceRoots,
  };
}
export async function listDirectory(
  root: string,
  relativePath: string,
): Promise<WorkspaceDirectoryEntry[]> {
  const directory = await workspacePath(root, relativePath);
  const result: WorkspaceDirectoryEntry[] = [];
  for (const name of await fs.readdir(directory)) {
    if (hidden.has(name)) continue;
    const relative = path.posix.join(relativePath, name);
    try {
      const target = await workspacePath(root, relative);
      const stat = await fs.stat(target);
      if (stat.isDirectory() || stat.isFile())
        result.push({
          name,
          relativePath: relative,
          isDirectory: stat.isDirectory(),
          ...(stat.isFile() ? { size: stat.size } : {}),
        });
    } catch {
      /* 失效链接及范围外条目不进入工作区列表。 */
    }
  }
  return result.sort(
    (a, b) =>
      Number(b.isDirectory) - Number(a.isDirectory) ||
      Number(a.name.startsWith(".")) - Number(b.name.startsWith(".")) ||
      a.name.toLowerCase().localeCompare(b.name.toLowerCase()),
  );
}
export async function searchFiles(
  root: string,
  query: string,
  limit: number,
  scopePath = "",
): Promise<WorkspaceFileEntry[]> {
  if (
    scopePath
      .split(/[\\/]/)
      .some((part) => fileSearchExcludedDirectories.has(part))
  )
    return [];
  const normalizedQuery = query.trim().replaceAll("\\", "/");
  const resultLimit = Math.min(200, Math.max(1, limit));
  if (!normalizedQuery)
    return materializeFiles(
      root,
      (await scanFiles(root, scopePath, resultLimit, false)).sort((a, b) =>
        a.relativePath.localeCompare(b.relativePath),
      ),
    );
  const slash = normalizedQuery.lastIndexOf("/");
  if (slash >= 0) {
    const directory = path.posix.join(
      scopePath,
      normalizedQuery.slice(0, slash + 1),
    );
    if (
      normalizedQuery
        .slice(0, slash + 1)
        .split("/")
        .some(
          (part) => part === ".." || fileSearchExcludedDirectories.has(part),
        )
    )
      return [];
    try {
      const fragment = normalizedQuery.slice(slash + 1).toLowerCase();
      const entries = await listDirectory(root, directory);
      return materializeFiles(
        root,
        entries
          .filter(
            (entry) =>
              !entry.isDirectory &&
              (fragment.startsWith(".") || !entry.name.startsWith(".")) &&
              entry.name.toLowerCase().includes(fragment),
          )
          .slice(0, resultLimit),
      );
    } catch {
      return [];
    }
  }
  const index = fileSearchIndex(root, scopePath);
  const settled = index.settled;
  const files =
    settled && settled.startedAt === index.invalidations
      ? settled.files
      : await ensureFileSearchIndex(index);
  const needle = normalizedQuery.toLowerCase();
  return materializeFiles(
    root,
    files
      .flatMap((entry) => {
        if (
          !needle.startsWith(".") &&
          entry.relativePath.split("/").some((part) => part.startsWith("."))
        )
          return [];
        const score = fileSearchScore(entry, needle);
        return score === undefined ? [] : [{ entry, score }];
      })
      .sort(
        (a, b) =>
          b.score - a.score ||
          a.entry.relativePath.localeCompare(b.entry.relativePath),
      )
      .slice(0, resultLimit)
      .map(({ entry }) => entry),
  );
}

function fileSearchIndex(root: string, scopePath: string): FileSearchIndex {
  const key = `${root}\0${scopePath}`;
  let index = fileSearchIndexes.get(key);
  if (!index) {
    index = { root, scopePath, invalidations: 0 };
    if (fileSearchIndexes.size >= fileSearchCacheLimit) {
      const oldest = fileSearchIndexes.keys().next().value;
      if (oldest) fileSearchIndexes.delete(oldest);
    }
  } else {
    fileSearchIndexes.delete(key);
  }
  fileSearchIndexes.set(key, index);
  return index;
}

function ensureFileSearchIndex(index: FileSearchIndex): Promise<IndexedFile[]> {
  if (index.generation) return index.generation;
  const startedAt = index.invalidations;
  const generation = scanFiles(
    index.root,
    index.scopePath,
    fileSearchMaxEntries,
    true,
  );
  index.generation = generation;
  void generation.then(
    (files) => {
      if (index.generation !== generation) return;
      index.generation = undefined;
      index.settled = { files, startedAt };
    },
    () => {
      if (index.generation === generation) index.generation = undefined;
    },
  );
  return generation;
}

async function scanFiles(
  root: string,
  scopePath: string,
  limit: number,
  includeHidden: boolean,
): Promise<IndexedFile[]> {
  const pending = [
    {
      relative: scopePath,
      absolute: await workspacePath(root, scopePath),
    },
  ];
  const seen = new Set<string>();
  const files: IndexedFile[] = [];
  let entriesSeen = 0;
  for (let next = 0; next < pending.length && entriesSeen < limit; next++) {
    const current = pending[next];
    if (!current) break;
    const { relative, absolute } = current;
    try {
      if (seen.has(absolute)) continue;
      seen.add(absolute);
      const entries = await fs.readdir(absolute, { withFileTypes: true });
      entries.sort((a, b) => a.name.localeCompare(b.name));
      for (const entry of entries) {
        if (hidden.has(entry.name)) continue;
        if (!includeHidden && entry.name.startsWith(".")) continue;
        const entryPath = path.posix.join(relative, entry.name);
        if (isAccountConfiguration(entryPath)) continue;
        if (entry.isDirectory()) {
          if (fileSearchExcludedDirectories.has(entry.name)) continue;
          pending.push({
            relative: entryPath,
            absolute: path.join(absolute, entry.name),
          });
        } else if (entry.isFile()) {
          files.push({ name: entry.name, relativePath: entryPath });
        } else if (entry.isSymbolicLink()) {
          try {
            const canonical = await workspacePath(root, entryPath);
            const status = await fs.stat(canonical);
            if (status.isDirectory()) {
              if (!fileSearchExcludedDirectories.has(entry.name))
                pending.push({ relative: entryPath, absolute: canonical });
            } else if (status.isFile()) {
              files.push({ name: entry.name, relativePath: entryPath });
            }
          } catch {
            // 范围外或失效的软链接不进入搜索索引。
          }
        }
        entriesSeen++;
        if (entriesSeen >= limit) break;
      }
    } catch (error) {
      if (next === 0) throw error;
    }
  }
  return files;
}

function fileSearchScore(
  entry: IndexedFile,
  query: string,
): number | undefined {
  const name = entry.name.toLowerCase();
  const relativePath = entry.relativePath.toLowerCase();
  if (name === query) return 1_000;
  if (name.startsWith(query)) return 900;
  if (name.includes(query)) return 700;
  if (relativePath.includes(query)) return 500;
  let cursor = 0;
  let gap = 0;
  for (const character of query) {
    const found = relativePath.indexOf(character, cursor);
    if (found < 0) return undefined;
    gap += found - cursor;
    cursor = found + 1;
  }
  return 300 + Math.max(0, 100 - gap);
}

async function materializeFiles(
  root: string,
  files: readonly IndexedFile[],
): Promise<WorkspaceFileEntry[]> {
  const results = await Promise.all(
    files.map(async (entry) => {
      try {
        return {
          ...entry,
          path: await workspacePath(root, entry.relativePath),
        };
      } catch {
        return undefined;
      }
    }),
  );
  return results.filter((entry): entry is WorkspaceFileEntry => !!entry);
}

async function removedChatFiles(root: string): Promise<string[]> {
  try {
    const manifest = await removedChatFilesPath(root);
    if ((await fs.lstat(manifest)).isSymbolicLink())
      throw new Error("会话文件清单不能是软链接");
    const value: unknown = JSON.parse(await fs.readFile(manifest, "utf8"));
    return Array.isArray(value)
      ? value.filter((item): item is string => typeof item === "string")
      : [];
  } catch (error) {
    if (error instanceof Error && "code" in error && error.code === "ENOENT")
      return [];
    throw error;
  }
}

async function generatedChatFiles(root: string): Promise<WorkspaceFileEntry[]> {
  const scopePath = "attachments/generated";
  const directory = await workspacePath(root, scopePath, true);
  try {
    await fs.stat(directory);
  } catch (error) {
    // 会话尚未生成文件时此目录不会创建；根目录和范围仍须通过上面的校验。
    if (error instanceof Error && "code" in error && error.code === "ENOENT")
      return [];
    throw error;
  }
  return searchFiles(root, "", 200, scopePath);
}

export async function listChatWorkspaceFiles(
  root: string,
): Promise<ChatWorkspaceFiles> {
  const [generated, removed] = await Promise.all([
    generatedChatFiles(root),
    removedChatFiles(root),
  ]);
  const removedPaths = await Promise.all(
    removed.map(async (relativePath) => {
      if (!relativePath.startsWith("attachments/")) return relativePath;
      try {
        await workspacePath(root, relativePath);
        return undefined;
      } catch {
        return relativePath;
      }
    }),
  );
  return {
    generated,
    removedPaths: removedPaths.filter((value): value is string =>
      Boolean(value),
    ),
  };
}

export async function chatWorkspaceFilePath(
  root: string,
  relativePath: string,
): Promise<string> {
  if (!relativePath.trim()) throw new Error("工作区相对路径不合法");
  const target = await workspacePath(root, relativePath);
  if (!(await fs.stat(target)).isFile()) throw new Error("工作区条目不是文件");
  return target;
}

export async function deleteChatWorkspaceFile(
  root: string,
  relativePath: string,
): Promise<boolean> {
  const target = await chatWorkspaceFilePath(root, relativePath);
  const canonicalRoot = await canonicalDirectory(root);
  const normalizedPath = relativePath.replace(/\\/g, "/");
  const ownedAttachment = normalizedPath.startsWith("attachments/");
  if (ownedAttachment) {
    const attachmentsRoot = path.join(canonicalRoot, "attachments");
    if (!within(attachmentsRoot, target))
      throw new Error("会话文件超出附件目录");
    const source = path.join(canonicalRoot, relativePath);
    if ((await fs.lstat(source)).isSymbolicLink())
      throw new Error("不能删除软链接会话文件");
  }
  const removed = new Set(await removedChatFiles(root));
  removed.add(relativePath);
  const manifest = await removedChatFilesPath(root, true);
  const temporary = `${manifest}.${randomUUID()}.tmp`;
  try {
    await fs.writeFile(temporary, JSON.stringify([...removed]), { flag: "wx" });
    await fs.rename(temporary, manifest);
  } finally {
    await fs.rm(temporary, { force: true });
  }
  if (ownedAttachment) await fs.unlink(target);
  return true;
}
const imageTypes: Record<string, string> = {
  ".png": "image/png",
  ".jpg": "image/jpeg",
  ".jpeg": "image/jpeg",
  ".gif": "image/gif",
  ".webp": "image/webp",
  ".svg": "image/svg+xml",
  ".bmp": "image/bmp",
  ".ico": "image/x-icon",
};
export async function readFile(
  root: string,
  relativePath: string,
): Promise<WorkspaceFileContent> {
  const target = await workspacePath(root, relativePath);
  const stat = await fs.stat(target);
  if (!stat.isFile()) throw new Error("工作区条目不是文件");
  const mediaType = imageTypes[path.extname(target).toLowerCase()];
  const base = {
    name: path.basename(target),
    relativePath,
    size: stat.size,
    mediaType,
  };
  if (stat.size > (mediaType ? imageLimit : textLimit))
    return { ...base, kind: "too_large" };
  const bytes = await fs.readFile(target);
  if (mediaType)
    return {
      ...base,
      kind: "image",
      dataUrl: `data:${mediaType};base64,${bytes.toString("base64")}`,
    };
  try {
    return {
      ...base,
      kind: "text",
      mediaType: "text/plain; charset=utf-8",
      content: new TextDecoder("utf-8", { fatal: true }).decode(bytes),
    };
  } catch {
    return { ...base, kind: "binary" };
  }
}
export async function writeFile(
  root: string,
  relativePath: string,
  content: string,
): Promise<boolean> {
  const target = await workspacePath(root, relativePath);
  if (!(await fs.stat(target)).isFile()) throw new Error("工作区条目不是文件");
  if (Buffer.byteLength(content) > textLimit)
    throw new Error("文件内容超过 10 MiB 写入上限");
  await fs.writeFile(target, content);
  return true;
}
const gitEnv = {
  ...process.env,
  GIT_TERMINAL_PROMPT: "0",
  GIT_OPTIONAL_LOCKS: "0",
};
const gitOptions = {
  env: gitEnv,
  maxBuffer: 20 * 1024 * 1024,
  timeout: 15000,
};

async function git(root: string, args: string[]): Promise<string | undefined> {
  try {
    return (await execute("git", ["-C", root, ...args], gitOptions)).stdout;
  } catch (error) {
    if (
      error instanceof Error &&
      "code" in error &&
      typeof error.code === "number"
    )
      return undefined;
    throw error;
  }
}

async function gitRequired(root: string, args: string[]): Promise<string> {
  try {
    return (await execute("git", ["-C", root, ...args], gitOptions)).stdout;
  } catch (error) {
    const message =
      error instanceof Error &&
      "stderr" in error &&
      typeof error.stderr === "string" &&
      error.stderr.trim()
        ? error.stderr.trim()
        : error instanceof Error
          ? error.message
          : "Git 命令执行失败";
    throw new Error(message, { cause: error });
  }
}

async function resolveGitRoot(root: string): Promise<string | undefined> {
  const canonical = await canonicalDirectory(root);
  return (await git(canonical, ["rev-parse", "--show-toplevel"]))?.trim();
}

async function lineStats(
  gitRoot: string,
  files: readonly string[],
): Promise<Map<string, [number, number]>> {
  const counts = new Map<string, [number, number]>();
  if (files.length === 0) return counts;
  const output = await git(gitRoot, [
    "diff",
    "HEAD",
    "--no-renames",
    "--numstat",
    "-z",
    "--",
    ...files,
  ]);
  for (const record of output?.split("\0") ?? []) {
    const match = /^(\d+|-)\t(\d+|-)\t([\s\S]+)$/.exec(record);
    if (match?.[3])
      counts.set(match[3], [Number(match[1]) || 0, Number(match[2]) || 0]);
  }
  for (const file of files) {
    if (counts.has(file)) continue;
    const stat = await fs.stat(path.join(gitRoot, file)).catch(() => undefined);
    counts.set(file, [stat && stat.size > 0 ? 1 : 0, 0]);
  }
  return counts;
}
export async function gitChanges(
  root: string,
  visited = new Set<string>(),
): Promise<WorkspaceGitChanges> {
  const canonical = await canonicalDirectory(root);
  const result: WorkspaceGitChanges = {
    isGitRepo: false,
    files: [],
    totalAddedLines: 0,
    totalDeletedLines: 0,
  };
  if (visited.has(canonical)) return result;
  visited.add(canonical);
  const gitRoot = (
    await git(canonical, ["rev-parse", "--show-toplevel"])
  )?.trim();
  if (!gitRoot) {
    const projects = await fs
      .readdir(path.join(canonical, "projects"))
      .catch(() => []);
    for (const name of projects) {
      const child = await gitChanges(
        path.join(canonical, "projects", name),
        visited,
      ).catch(() => undefined);
      if (!child) continue;
      result.isGitRepo ||= child.isGitRepo;
      result.files.push(
        ...child.files.map((file) => ({
          ...file,
          path: `projects/${name}/${file.path}`,
        })),
      );
    }
  } else {
    result.isGitRepo = true;
    const branchName = (
      await git(gitRoot, ["rev-parse", "--abbrev-ref", "HEAD"])
    )?.trim();
    if (branchName && branchName !== "HEAD") {
      result.branch = branchName;
    } else {
      const commit = (
        await git(gitRoot, ["rev-parse", "--short", "HEAD"])
      )?.trim();
      if (commit) result.branch = commit;
    }
    const prefix = path.relative(gitRoot, canonical);
    const spec = prefix || ".";
    const hasHead =
      (await git(gitRoot, ["rev-parse", "--verify", "HEAD"])) !== undefined;
    const status =
      (
        await git(gitRoot, [
          "diff",
          ...(hasHead ? ["HEAD"] : ["--cached"]),
          "--no-renames",
          "--name-status",
          "-z",
          "--",
          spec,
        ])
      )?.split("\0") ?? [];
    const counts = new Map<string, [number, number]>();
    for (const record of (
      await git(gitRoot, [
        "diff",
        ...(hasHead ? ["HEAD"] : ["--cached"]),
        "--no-renames",
        "--numstat",
        "-z",
        "--",
        spec,
      ])
    )?.split("\0") ?? []) {
      const match = /^(\d+|-)\t(\d+|-)\t([\s\S]+)$/.exec(record);
      if (match?.[3])
        counts.set(match[3], [Number(match[1]) || 0, Number(match[2]) || 0]);
    }
    for (let i = 0; i + 1 < status.length; i += 2) {
      const file = status[i + 1];
      if (!file) continue;
      const code = status[i];
      const relative = path.relative(canonical, path.join(gitRoot, file));
      const [addedLines, deletedLines] = counts.get(file) ?? [0, 0];
      result.files.push({
        path: relative,
        name: path.basename(file),
        gitRoot,
        status: code === "A" ? "added" : code === "D" ? "deleted" : "modified",
        addedLines,
        deletedLines,
      });
    }
    for (const file of (
      await git(gitRoot, [
        "ls-files",
        "--others",
        "--exclude-standard",
        "-z",
        "--",
        spec,
      ])
    )?.split("\0") ?? []) {
      if (!file) continue;
      const relative = path.relative(canonical, path.join(gitRoot, file));
      const size =
        (await fs.stat(path.join(gitRoot, file)).catch(() => undefined))
          ?.size ?? 0;
      result.files.push({
        path: relative,
        name: path.basename(file),
        gitRoot,
        status: "untracked",
        addedLines: size > 0 ? 1 : 0,
        deletedLines: 0,
      });
    }
  }
  result.files.sort((a, b) => a.path.localeCompare(b.path));
  result.totalAddedLines = result.files.reduce(
    (sum, f) => sum + f.addedLines,
    0,
  );
  result.totalDeletedLines = result.files.reduce(
    (sum, f) => sum + f.deletedLines,
    0,
  );
  return result;
}

export async function listGitBranches(
  root: string,
): Promise<WorkspaceGitBranches> {
  const gitRoot = await resolveGitRoot(root);
  const result: WorkspaceGitBranches = { isGitRepo: false, branches: [] };
  if (!gitRoot) return result;
  result.isGitRepo = true;
  result.gitRoot = gitRoot;
  const output = await git(gitRoot, [
    "for-each-ref",
    "--sort=-committerdate",
    "--format=%(refname:short)\t%(refname)\t%(HEAD)",
    "refs/heads",
    "refs/remotes",
  ]);
  if (!output?.trim()) return result;
  const branches: WorkspaceGitBranch[] = [];
  for (const line of output.trim().split("\n")) {
    const [name, refName, head] = line.split("\t");
    if (!name || !refName || name === "origin/HEAD") continue;
    branches.push({
      name,
      isCurrent: head === "*",
      isRemote: refName.startsWith("refs/remotes/"),
    });
  }
  result.branches = branches.sort((left, right) => {
    if (left.isCurrent !== right.isCurrent) return left.isCurrent ? -1 : 1;
    if (left.isRemote !== right.isRemote) return left.isRemote ? 1 : -1;
    return left.name.localeCompare(right.name);
  });
  return result;
}

export async function previewGitCheckout(
  root: string,
  branch: string,
): Promise<WorkspaceGitCheckoutPreview> {
  const gitRoot = await resolveGitRoot(root);
  if (!gitRoot) return { canCheckout: false, conflicts: [] };
  if (!(await git(gitRoot, ["rev-parse", "--verify", branch]))) {
    throw new Error("分支不存在");
  }
  const currentCommit = (await git(gitRoot, ["rev-parse", "HEAD"]))?.trim();
  const targetCommit = (await git(gitRoot, ["rev-parse", branch]))?.trim();
  if (currentCommit && targetCommit && currentCommit === targetCommit) {
    return { canCheckout: true, conflicts: [] };
  }
  const localChanged = [
    ...((await git(gitRoot, ["diff", "--name-only", "-z", "HEAD"]))
      ?.split("\0")
      .filter(Boolean) ?? []),
    ...((
      await git(gitRoot, ["ls-files", "--others", "--exclude-standard", "-z"])
    )
      ?.split("\0")
      .filter(Boolean) ?? []),
  ];
  const uniqueLocal = [...new Set(localChanged)];
  if (uniqueLocal.length === 0) return { canCheckout: true, conflicts: [] };
  const checkoutChanges = new Set(
    (await git(gitRoot, ["diff", "--name-only", "-z", "HEAD", branch]))
      ?.split("\0")
      .filter(Boolean) ?? [],
  );
  const conflictingPaths: string[] = [];
  for (const file of uniqueLocal) {
    if (checkoutChanges.has(file)) {
      conflictingPaths.push(file);
      continue;
    }
    if (
      (await git(gitRoot, ["cat-file", "-e", `${branch}:${file}`])) !==
      undefined
    ) {
      conflictingPaths.push(file);
    }
  }
  if (conflictingPaths.length === 0) {
    return { canCheckout: true, conflicts: [] };
  }
  const stats = await lineStats(gitRoot, conflictingPaths);
  return {
    canCheckout: false,
    conflicts: conflictingPaths.sort().map((filePath) => ({
      path: filePath,
      addedLines: stats.get(filePath)?.[0] ?? 0,
      deletedLines: stats.get(filePath)?.[1] ?? 0,
    })),
  };
}

export async function listGitRepositories(
  roots: readonly string[],
): Promise<WorkspaceGitRepositories> {
  const repositories: WorkspaceGitRepositories["repositories"] = [];
  const seen = new Set<string>();
  for (const root of roots) {
    if (!root.trim()) continue;
    let directory: string;
    try {
      directory = await canonicalDirectory(root);
    } catch {
      continue;
    }
    const gitRoot = await resolveGitRoot(directory);
    if (!gitRoot || seen.has(gitRoot)) continue;
    seen.add(gitRoot);
    const changes = await gitChanges(directory);
    repositories.push({
      directory,
      gitRoot,
      label: path.basename(directory),
      branch: changes.branch,
    });
  }
  return { repositories };
}

export async function checkoutGitBranch(
  root: string,
  branch: string,
  commitMessage?: string,
): Promise<WorkspaceGitCheckoutResult> {
  const gitRoot = await resolveGitRoot(root);
  if (!gitRoot) throw new Error("当前目录不是 Git 仓库");
  if (!(await git(gitRoot, ["rev-parse", "--verify", branch]))) {
    throw new Error("分支不存在");
  }
  if (commitMessage?.trim()) {
    await gitRequired(gitRoot, ["add", "-A"]);
    await gitRequired(gitRoot, ["commit", "-m", commitMessage.trim()]);
  } else {
    const preview = await previewGitCheckout(root, branch);
    if (!preview.canCheckout) {
      throw new Error("存在本地更改，无法切换分支");
    }
  }
  const switched =
    (await git(gitRoot, ["switch", "--quiet", branch])) !== undefined;
  if (!switched) {
    const remoteRef = (
      await git(gitRoot, ["show-ref", "--verify", `refs/remotes/${branch}`])
    )?.trim();
    if (remoteRef) {
      const localName = branch.replace(/^[^/]+\//, "");
      const localExists = await git(gitRoot, [
        "show-ref",
        "--verify",
        `refs/heads/${localName}`,
      ]);
      if (localExists) {
        await gitRequired(gitRoot, ["switch", "--quiet", localName]);
      } else {
        await gitRequired(gitRoot, ["switch", "--quiet", "--track", branch]);
      }
    } else {
      await gitRequired(gitRoot, ["switch", "--quiet", branch]);
    }
  }
  const branchName =
    (await git(gitRoot, ["rev-parse", "--abbrev-ref", "HEAD"]))?.trim() ??
    branch;
  return { branch: branchName };
}

export async function readDiff(root: string, relativePath: string) {
  const target = await workspacePath(root, relativePath, true);
  let directory = path.dirname(target);
  while (!(await fs.stat(directory).catch(() => undefined))?.isDirectory()) {
    const parent = path.dirname(directory);
    if (parent === directory) throw new Error("找不到文件父目录");
    directory = parent;
  }
  const gitRoot = (
    await git(directory, ["rev-parse", "--show-toplevel"])
  )?.trim();
  if (!gitRoot || !within(gitRoot, target))
    throw new Error("文件不属于 Git 仓库");
  const stat = await fs.stat(target).catch(() => undefined);
  if (stat && stat.size > textLimit) throw new Error("文件超过 Diff 读取上限");
  const oldContent =
    (await git(gitRoot, ["show", `HEAD:${path.relative(gitRoot, target)}`])) ??
    "";
  const newContent = stat ? await fs.readFile(target, "utf8") : "";
  return { name: path.basename(target), relativePath, oldContent, newContent };
}

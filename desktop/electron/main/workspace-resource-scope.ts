import { constants, type BigIntStats } from "node:fs";
import { WorkspaceResourceWatchHub } from "./workspace-resource-watch";
import { promises as fs } from "node:fs";
import { randomUUID } from "node:crypto";
import path from "node:path";

import type {
  FileReferenceResolution,
  WorkspaceDirectoryEntry,
  WorkspaceFileEntry,
  WorkspaceResourceImport,
  WorkspaceResourceChangeEvent,
  WorkspaceResourceRead,
  WorkspaceResourceScope,
  WorkspaceResourceStat,
  WorkspaceResourceWrite,
} from "../shared/contracts";
import type { CoreSupervisor } from "./core-supervisor";
import { workspaceSingleFileBytesLimit } from "../shared/workspace-byte-limits";
import {
  canonicalDirectory,
  invalidateFileSearch,
  listDirectory,
  resolveFileReference,
  searchFiles,
  within,
  workspacePath,
} from "./workspace-files";

const readLimit = workspaceSingleFileBytesLimit;
const writeLimit = 10 * 1024 * 1024;
const mediaTypes: Record<string, string> = {
  ".csv": "text/csv",
  ".doc": "application/msword",
  ".docx":
    "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
  ".gif": "image/gif",
  ".jpeg": "image/jpeg",
  ".jpg": "image/jpeg",
  ".md": "text/markdown",
  ".pdf": "application/pdf",
  ".png": "image/png",
  ".ppt": "application/vnd.ms-powerpoint",
  ".pptx":
    "application/vnd.openxmlformats-officedocument.presentationml.presentation",
  ".svg": "image/svg+xml",
  ".tsv": "text/tab-separated-values",
  ".txt": "text/plain",
  ".webp": "image/webp",
  ".xls": "application/vnd.ms-excel",
  ".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
};

type Reader = { scopeId: string; controller: AbortController };
type Subscription = { scopeId: string; close: () => void; active: boolean };

function versionOf(stat: BigIntStats): string {
  return [stat.dev, stat.ino, stat.size, stat.mtimeNs, stat.ctimeNs].join(":");
}

function metadata(
  target: string,
  relativePath: string,
  stat: BigIntStats,
): WorkspaceResourceStat {
  if (!stat.isFile()) throw new Error("工作区条目不是文件");
  if (stat.size > BigInt(Number.MAX_SAFE_INTEGER))
    throw new Error("文件大小超出可表示范围");
  return {
    relativePath,
    size: Number(stat.size),
    version: versionOf(stat),
    modifiedAt: Number(stat.mtimeMs),
    mediaType:
      mediaTypes[path.extname(target).toLowerCase()] ??
      "application/octet-stream",
  };
}

function isMissing(error: unknown): boolean {
  return (
    error instanceof Error &&
    "code" in error &&
    (error.code === "ENOENT" || error.code === "ENOTDIR")
  );
}

/**
 * Electron Main 持有 Chat 授权、读取取消与文件观察者。Renderer 只收到不透明 scopeId，
 * 文件路径仍逐次经过 workspacePath 的真实路径和软链接检查。
 */
export class WorkspaceResourceScopes {
  private readonly scopes = new Map<string, WorkspaceResourceScope>();
  private readonly chatScopes = new Map<string, string>();
  private readonly scopeRoots = new Map<string, Set<string>>();
  private readonly opening = new Map<string, Promise<WorkspaceResourceScope>>();
  private readonly readers = new Map<string, Reader>();
  private readonly writeQueues = new Map<string, Promise<void>>();
  private readonly subscriptions = new Map<string, Subscription>();
  private readonly watchHub = new WorkspaceResourceWatchHub();
  private generation = 0;
  private coreIdentity?: string;

  constructor(
    private readonly core: Pick<CoreSupervisor, "connection">,
    private readonly emit: (event: WorkspaceResourceChangeEvent) => void,
    private readonly fetcher: typeof fetch = fetch,
  ) {}

  async open(chatId: string): Promise<WorkspaceResourceScope> {
    const connection = await this.core.connection();
    const identity = `${connection.baseUrl}\0${connection.token}`;
    if (this.coreIdentity !== undefined && this.coreIdentity !== identity)
      this.dispose();
    this.coreIdentity = identity;
    const existing = this.chatScopes.get(chatId);
    if (existing) return this.requireScope(existing);
    const pending = this.opening.get(chatId);
    if (pending) return pending;
    const generation = this.generation;
    const opening = this.createScope(chatId, generation, connection).finally(
      () => {
        this.opening.delete(chatId);
      },
    );
    this.opening.set(chatId, opening);
    return opening;
  }

  private async createScope(
    chatId: string,
    generation: number,
    connection: Awaited<ReturnType<CoreSupervisor["connection"]>>,
  ): Promise<WorkspaceResourceScope> {
    const response = await this.fetcher(
      `${connection.baseUrl}/api/v1/chats/${encodeURIComponent(chatId)}`,
      {
        headers: { Authorization: `Bearer ${connection.token}` },
        signal: AbortSignal.timeout(5000),
      },
    );
    if (!response.ok) {
      await response.body?.cancel();
      throw new Error("无法验证 Chat 工作区范围");
    }
    const value: unknown = await response.json();
    const chat =
      typeof value === "object" && value !== null && "chat" in value
        ? value.chat
        : undefined;
    if (
      typeof chat !== "object" ||
      chat === null ||
      !("id" in chat) ||
      chat.id !== chatId ||
      !("workspaceDirectory" in chat) ||
      typeof chat.workspaceDirectory !== "string" ||
      !chat.workspaceDirectory
    )
      throw new Error("Core 未返回匹配的 Chat 工作区");
    const scope: WorkspaceResourceScope = {
      id: randomUUID(),
      root: await canonicalDirectory(chat.workspaceDirectory),
    };
    const sources =
      "additionalWorkingDirectories" in chat &&
      Array.isArray(chat.additionalWorkingDirectories)
        ? chat.additionalWorkingDirectories.filter(
            (value): value is string => typeof value === "string",
          )
        : [];
    const allowedRoots = new Set([scope.root]);
    for (const source of sources) {
      try {
        allowedRoots.add(await canonicalDirectory(source));
      } catch {
        // 已失效的旧来源不授予文件权限，其余有效目录仍可使用。
      }
    }
    try {
      const projects = await listDirectory(scope.root, "projects");
      for (const project of projects) {
        if (!project.isDirectory) continue;
        allowedRoots.add(await workspacePath(scope.root, project.relativePath));
      }
    } catch {
      // 没有关联项目的 Chat 仍保留自己的工作区范围。
    }
    if (generation !== this.generation) throw new Error("文件范围已关闭");
    this.scopes.set(scope.id, scope);
    this.chatScopes.set(chatId, scope.id);
    this.scopeRoots.set(scope.id, allowedRoots);
    return scope;
  }

  private requireScope(scopeId: string): WorkspaceResourceScope {
    const scope = this.scopes.get(scopeId);
    if (!scope) throw new Error("文件范围无效或已关闭");
    return scope;
  }

  private async authorizedRoot(
    scopeId: string,
    requestedRoot?: string,
  ): Promise<string> {
    const scope = this.requireScope(scopeId);
    if (!requestedRoot) return scope.root;
    const canonical = await canonicalDirectory(requestedRoot);
    if (!this.scopeRoots.get(scopeId)?.has(canonical))
      throw new Error("文件根目录不属于此 Chat 范围");
    return canonical;
  }

  /** 浏览器等 Main 资源只可使用 Core 为此 Chat 授权的精确目录。 */
  async verifyRoot(scopeId: string, requestedRoot: string): Promise<string> {
    return this.authorizedRoot(scopeId, requestedRoot);
  }

  async resolveReference(
    scopeId: string,
    inputPath: string,
    baseDirectory?: string,
  ): Promise<FileReferenceResolution> {
    const scope = this.requireScope(scopeId);
    const roots = [...(this.scopeRoots.get(scopeId) ?? new Set([scope.root]))];
    const base = baseDirectory
      ? await canonicalDirectory(baseDirectory)
      : scope.root;
    if (!roots.some((root) => within(root, base)))
      throw new Error("文件引用基准目录不属于此 Chat 范围");
    return resolveFileReference({
      path: inputPath,
      baseDirectory: base,
      workspaceRoot: scope.root,
      sourceRoots: roots.filter((root) => root !== scope.root),
    });
  }

  async list(
    scopeId: string,
    relativePath: string,
    requestedRoot?: string,
  ): Promise<WorkspaceDirectoryEntry[]> {
    return listDirectory(
      await this.authorizedRoot(scopeId, requestedRoot),
      relativePath,
    );
  }

  async search(
    scopeId: string,
    query: string,
    limit: number,
    scopePath: string,
    requestedRoot?: string,
  ): Promise<WorkspaceFileEntry[]> {
    return searchFiles(
      await this.authorizedRoot(scopeId, requestedRoot),
      query,
      limit,
      scopePath,
    );
  }

  /** 另存始终在 Main 复制文件，不受预览读取大小限制。目标路径只能来自宿主保存对话框。 */
  async copyTo(
    scopeId: string,
    relativePath: string,
    destination: string,
    requestedRoot?: string,
  ): Promise<void> {
    const root = await this.authorizedRoot(scopeId, requestedRoot);
    const source = await workspacePath(root, relativePath);
    if (path.resolve(destination) === source)
      throw new Error("另存目标不能是原文件");
    const before = await fs.stat(source, { bigint: true });
    metadata(source, relativePath, before);
    const temporary = path.join(
      path.dirname(destination),
      `file-save-${randomUUID()}-${path.basename(destination)}.tmp`,
    );
    try {
      await fs.copyFile(source, temporary);
      const current = await fs.stat(await workspacePath(root, relativePath), {
        bigint: true,
      });
      if (versionOf(current) !== versionOf(before))
        throw new Error("文件在另存期间已变化，请重试");
      this.requireScope(scopeId);
      await fs.rename(temporary, destination);
    } finally {
      await fs.rm(temporary, { force: true });
    }
  }

  async rename(
    scopeId: string,
    relativePath: string,
    nextName: string,
    expectedVersion: string,
    requestedRoot?: string,
  ): Promise<WorkspaceResourceStat> {
    if (
      !nextName ||
      nextName === "." ||
      nextName === ".." ||
      /[\\/\0]/.test(nextName)
    )
      throw new Error("文件名不合法");
    const root = await this.authorizedRoot(scopeId, requestedRoot);
    const source = await workspacePath(root, relativePath);
    if ((await fs.lstat(path.join(root, relativePath))).isSymbolicLink())
      throw new Error("不能重命名软链接入口");
    const before = await fs.stat(source, { bigint: true });
    metadata(source, relativePath, before);
    if (versionOf(before) !== expectedVersion)
      throw new Error("文件已变化，请刷新后重命名");
    const destinationRelative = path.posix.join(
      path.posix.dirname(relativePath),
      nextName,
    );
    const destination = await workspacePath(root, destinationRelative, true);
    if (path.dirname(source) !== path.dirname(destination))
      throw new Error("重命名不能移动文件到其他目录");
    if (source === destination) return metadata(source, relativePath, before);
    try {
      await fs.lstat(destination);
      throw new Error("目标文件名已存在");
    } catch (error) {
      if (!isMissing(error)) throw error;
    }
    const current = await fs.stat(await workspacePath(root, relativePath), {
      bigint: true,
    });
    if (versionOf(current) !== expectedVersion)
      throw new Error("文件已变化，请刷新后重命名");
    this.requireScope(scopeId);
    await fs.rename(source, destination);
    invalidateFileSearch(root);
    return metadata(
      destination,
      destinationRelative,
      await fs.stat(destination, { bigint: true }),
    );
  }

  /** 目录操作限制在 Chat 授权根内部，不经共享 projects 软链接跨根改名。 */
  async directoryVersion(
    scopeId: string,
    relativePath: string,
    requestedRoot?: string,
  ): Promise<string> {
    const root = await this.authorizedRoot(scopeId, requestedRoot);
    const directory = await this.directoryPath(root, relativePath);
    const info = await fs.stat(directory, { bigint: true });
    if (!info.isDirectory()) throw new Error("工作区条目不是目录");
    return versionOf(info);
  }

  async renameDirectory(
    scopeId: string,
    relativePath: string,
    nextName: string,
    expectedVersion: string,
    requestedRoot?: string,
  ): Promise<string> {
    if (
      !nextName ||
      nextName === "." ||
      nextName === ".." ||
      /[\\/\0]/.test(nextName)
    )
      throw new Error("目录名不合法");
    const root = await this.authorizedRoot(scopeId, requestedRoot);
    const source = await this.directoryPath(root, relativePath);
    if (versionOf(await fs.stat(source, { bigint: true })) !== expectedVersion)
      throw new Error("目录已变化，请刷新后重命名");
    const destinationRelative = path.posix.join(
      path.posix.dirname(relativePath),
      nextName,
    );
    const destination = await workspacePath(root, destinationRelative, true);
    if (
      !within(root, destination) ||
      path.dirname(source) !== path.dirname(destination)
    )
      throw new Error("重命名不能移动目录到其他位置");
    if (destination === source) return relativePath;
    try {
      await fs.lstat(destination);
      throw new Error("目标目录名已存在");
    } catch (error) {
      if (!isMissing(error)) throw error;
    }
    const current = await this.directoryPath(root, relativePath);
    if (
      current !== source ||
      versionOf(await fs.stat(current, { bigint: true })) !== expectedVersion
    )
      throw new Error("目录已变化，请刷新后重命名");
    this.requireScope(scopeId);
    await fs.rename(source, destination);
    invalidateFileSearch(root);
    return destinationRelative;
  }

  async removeDirectory(
    scopeId: string,
    relativePath: string,
    expectedVersion: string,
    requestedRoot?: string,
  ): Promise<void> {
    const root = await this.authorizedRoot(scopeId, requestedRoot);
    const source = await this.directoryPath(root, relativePath);
    if (versionOf(await fs.stat(source, { bigint: true })) !== expectedVersion)
      throw new Error("目录已变化，请刷新后删除");
    const current = await this.directoryPath(root, relativePath);
    if (
      current !== source ||
      versionOf(await fs.stat(current, { bigint: true })) !== expectedVersion
    )
      throw new Error("目录已变化，请刷新后删除");
    this.requireScope(scopeId);
    await fs.rm(source, { recursive: true, force: false });
    invalidateFileSearch(root);
  }

  private async directoryPath(root: string, relativePath: string) {
    if (!relativePath || relativePath === ".")
      throw new Error("不能操作工作区根目录");
    const directory = await workspacePath(root, relativePath);
    if (!within(root, directory) || directory === root)
      throw new Error("目录超出授权根范围");
    if ((await fs.lstat(path.join(root, relativePath))).isSymbolicLink())
      throw new Error("不能操作软链接目录入口");
    if (!(await fs.stat(directory)).isDirectory())
      throw new Error("工作区条目不是目录");
    return directory;
  }

  async removeFile(
    scopeId: string,
    relativePath: string,
    expectedVersion: string,
    requestedRoot?: string,
  ): Promise<void> {
    const root = await this.authorizedRoot(scopeId, requestedRoot);
    if (!relativePath || relativePath === ".")
      throw new Error("不能删除工作区根目录");
    const source = await workspacePath(root, relativePath);
    if ((await fs.lstat(path.join(root, relativePath))).isSymbolicLink())
      throw new Error("不能删除软链接入口");
    const before = await fs.stat(source, { bigint: true });
    metadata(source, relativePath, before);
    if (versionOf(before) !== expectedVersion)
      throw new Error("文件已变化，请刷新后删除");
    const current = await fs.stat(await workspacePath(root, relativePath), {
      bigint: true,
    });
    if (versionOf(current) !== expectedVersion)
      throw new Error("文件已变化，请刷新后删除");
    this.requireScope(scopeId);
    await fs.unlink(source);
    invalidateFileSearch(root);
  }

  /** 输入源仅由 Main 的打开文件对话框提供，目标必须位于 Chat 授权目录。 */
  async importFiles(
    scopeId: string,
    directoryPath: string,
    sourcePaths: string[],
    requestedRoot?: string,
  ): Promise<WorkspaceResourceImport> {
    const root = await this.authorizedRoot(scopeId, requestedRoot);
    if (sourcePaths.length > 32) throw new Error("一次最多导入 32 个文件");
    const directory = await workspacePath(root, directoryPath);
    if (!(await fs.stat(directory)).isDirectory())
      throw new Error("导入目标不是目录");
    const imported: string[] = [];
    const failed: { name: string; message: string }[] = [];
    for (const source of sourcePaths) {
      const name = path.basename(source);
      try {
        this.requireScope(scopeId);
        if (!(await fs.stat(source)).isFile())
          throw new Error("只能导入普通文件");
        const relativePath = path.posix.join(directoryPath, name);
        const target = await workspacePath(root, relativePath, true);
        if (path.dirname(target) !== directory)
          throw new Error("导入目标超出所选目录");
        await fs.copyFile(source, target, constants.COPYFILE_EXCL);
        imported.push(relativePath);
      } catch (cause) {
        failed.push({
          name,
          message: cause instanceof Error ? cause.message : "导入失败",
        });
      }
    }
    if (imported.length) invalidateFileSearch(root);
    return { imported, failed };
  }

  /** 拖入的浏览器 File 字节不携带任意本机路径，仅以独占链接落入授权目录。 */
  async importBytes(
    scopeId: string,
    directoryPath: string,
    name: string,
    bytes: Uint8Array,
    requestedRoot?: string,
  ): Promise<string> {
    const root = await this.authorizedRoot(scopeId, requestedRoot);
    if (
      !name ||
      name === "." ||
      name === ".." ||
      name.includes("/") ||
      name.includes("\\") ||
      name.includes("\0")
    )
      throw new Error("导入文件名无效");
    if (bytes.byteLength > workspaceSingleFileBytesLimit)
      throw new Error("拖入文件超过 64 MiB，请使用导入按钮");
    const directory = await workspacePath(root, directoryPath);
    if (!(await fs.stat(directory)).isDirectory())
      throw new Error("导入目标不是目录");
    const relativePath = path.posix.join(directoryPath, name);
    const target = await workspacePath(root, relativePath, true);
    if (path.dirname(target) !== directory)
      throw new Error("导入目标超出所选目录");
    const temporary = path.join(directory, `resource-import-${randomUUID()}`);
    try {
      await fs.writeFile(temporary, bytes, { flag: "wx" });
      this.requireScope(scopeId);
      await fs.link(temporary, target);
      invalidateFileSearch(root);
      return relativePath;
    } finally {
      await fs.unlink(temporary).catch(() => {});
    }
  }

  async stat(
    scopeId: string,
    relativePath: string,
    requestedRoot?: string,
  ): Promise<WorkspaceResourceStat | null> {
    const root = await this.authorizedRoot(scopeId, requestedRoot);
    try {
      const target = await workspacePath(root, relativePath);
      return metadata(
        target,
        relativePath,
        await fs.stat(target, { bigint: true }),
      );
    } catch (error) {
      if (isMissing(error)) return null;
      throw error;
    }
  }

  /** 仅供 Main 调用系统默认应用；真实路径不返回给 Renderer。 */
  async filePath(
    scopeId: string,
    relativePath: string,
    requestedRoot?: string,
  ): Promise<string> {
    const root = await this.authorizedRoot(scopeId, requestedRoot);
    const target = await workspacePath(root, relativePath);
    metadata(target, relativePath, await fs.stat(target, { bigint: true }));
    return target;
  }

  async read(
    scopeId: string,
    relativePath: string,
    requestId: string,
    requestedRoot?: string,
  ): Promise<WorkspaceResourceRead> {
    const root = await this.authorizedRoot(scopeId, requestedRoot);
    if (this.readers.has(requestId)) throw new Error("重复的文件读取请求");
    const controller = new AbortController();
    this.readers.set(requestId, { scopeId, controller });
    try {
      const target = await workspacePath(root, relativePath);
      const handle = await fs.open(target, "r");
      try {
        const before = await handle.stat({ bigint: true });
        const info = metadata(target, relativePath, before);
        if (info.size > readLimit) throw new Error("文件超过 64 MiB 读取上限");
        const bytes = await handle.readFile({ signal: controller.signal });
        const after = await handle.stat({ bigint: true });
        const current = await fs.stat(await workspacePath(root, relativePath), {
          bigint: true,
        });
        if (
          versionOf(before) !== versionOf(after) ||
          versionOf(before) !== versionOf(current)
        )
          throw new Error("文件读取期间已变化");
        if (controller.signal.aborted) throw new Error("文件读取已取消");
        return { ...info, bytes: new Uint8Array(bytes) };
      } finally {
        await handle.close();
      }
    } finally {
      this.readers.delete(requestId);
    }
  }

  cancel(scopeId: string, requestId: string): boolean {
    this.requireScope(scopeId);
    const reader = this.readers.get(requestId);
    if (!reader) return false;
    if (reader.scopeId !== scopeId) throw new Error("文件请求不属于此范围");
    reader.controller.abort();
    return true;
  }

  async write(
    scopeId: string,
    relativePath: string,
    content: string,
    expectedVersion: string,
    saveRevision: number,
    requestedRoot?: string,
  ): Promise<WorkspaceResourceWrite> {
    const root = await this.authorizedRoot(scopeId, requestedRoot);
    if (Buffer.byteLength(content, "utf8") > writeLimit)
      throw new Error("文件超过 10 MiB 写入上限");
    const target = await workspacePath(root, relativePath);
    const generation = this.generation;
    const previous = this.writeQueues.get(target) ?? Promise.resolve();
    let release!: () => void;
    const queued = new Promise<void>((resolve) => {
      release = resolve;
    });
    this.writeQueues.set(target, queued);
    await previous;
    const temporary = path.join(
      path.dirname(target),
      `file-save-${randomUUID()}-${path.basename(target)}.tmp`,
    );
    try {
      const before = await fs.stat(target, { bigint: true });
      metadata(target, relativePath, before);
      if (versionOf(before) !== expectedVersion)
        throw new Error("文件已被外部修改，请重新载入后保存");
      const handle = await fs.open(
        temporary,
        "wx",
        Number(before.mode & 0o777n),
      );
      try {
        await handle.writeFile(content, { encoding: "utf8" });
        await handle.sync();
      } finally {
        await handle.close();
      }
      await fs.chmod(temporary, Number(before.mode & 0o777n));
      const current = await fs.stat(await workspacePath(root, relativePath), {
        bigint: true,
      });
      if (versionOf(current) !== expectedVersion)
        throw new Error("文件已被外部修改，请重新载入后保存");
      if (generation !== this.generation) throw new Error("文件范围已关闭");
      await fs.rename(temporary, target);
      return {
        resource: metadata(
          target,
          relativePath,
          await fs.stat(target, { bigint: true }),
        ),
        saveRevision,
      };
    } finally {
      try {
        await fs.rm(temporary, { force: true });
      } finally {
        release();
        if (this.writeQueues.get(target) === queued)
          this.writeQueues.delete(target);
      }
    }
  }

  async watch(
    scopeId: string,
    relativePath: string,
    requestedRoot?: string,
    directory = false,
  ): Promise<WorkspaceResourceChangeEvent> {
    const root = await this.authorizedRoot(scopeId, requestedRoot);
    if (this.subscriptions.size >= 64)
      throw new Error("文件观察者数量达到上限");
    const target = await workspacePath(root, relativePath, true);
    if (directory && !(await fs.stat(target)).isDirectory())
      throw new Error("观察目标不是目录");
    this.requireScope(scopeId);
    const subscriptionId = randomUUID();
    let sequence = 0;
    let lastFileVersion: string | null | undefined;
    const subscription: Subscription = {
      scopeId,
      close: () => {},
      active: true,
    };
    this.subscriptions.set(subscriptionId, subscription);
    const roots =
      root === this.requireScope(scopeId).root
        ? [...(this.scopeRoots.get(scopeId) ?? new Set([root]))]
        : [root];
    const stops: (() => void)[] = [];
    subscription.close = () => {
      for (const stop of stops) stop();
    };
    try {
      for (const observedRoot of roots) {
        const stop = await this.watchHub.subscribe(observedRoot, (state) => {
          if (!subscription.active) return;
          invalidateFileSearch(root);
          invalidateFileSearch(observedRoot);
          const next = ++sequence;
          if (directory) {
            this.emit({
              subscriptionId,
              sequence: next,
              resource: null,
              state,
            });
            return;
          }
          void this.stat(scopeId, relativePath, root)
            .then((resource) => {
              const version = resource?.version ?? null;
              if (
                next < sequence ||
                (state === "changed" &&
                  lastFileVersion !== undefined &&
                  lastFileVersion === version)
              )
                return;
              lastFileVersion = version;
              if (subscription.active)
                this.emit({ subscriptionId, sequence: next, resource, state });
            })
            .catch(() => {
              if (subscription.active)
                this.emit({
                  subscriptionId,
                  sequence: next,
                  resource: null,
                  state,
                });
            });
        });
        stops.push(stop);
        if (!subscription.active) stop();
      }
    } catch (error) {
      this.unwatch(scopeId, subscriptionId);
      throw error;
    }
    try {
      const resource = directory
        ? null
        : await this.stat(scopeId, relativePath, root);
      if (!directory && lastFileVersion === undefined)
        lastFileVersion = resource?.version ?? null;
      return { subscriptionId, sequence: 0, resource };
    } catch (error) {
      this.unwatch(scopeId, subscriptionId);
      throw error;
    }
  }

  unwatch(scopeId: string, subscriptionId: string): boolean {
    this.requireScope(scopeId);
    const subscription = this.subscriptions.get(subscriptionId);
    if (!subscription) return false;
    if (subscription.scopeId !== scopeId)
      throw new Error("文件观察者不属于此范围");
    subscription.active = false;
    subscription.close();
    this.subscriptions.delete(subscriptionId);
    return true;
  }

  dispose(): void {
    this.generation += 1;
    this.coreIdentity = undefined;
    this.opening.clear();
    for (const reader of this.readers.values()) reader.controller.abort();
    this.readers.clear();
    for (const subscription of this.subscriptions.values()) {
      subscription.active = false;
      subscription.close();
    }
    this.subscriptions.clear();
    this.watchHub.close();
    this.scopes.clear();
    this.chatScopes.clear();
    this.scopeRoots.clear();
  }
}

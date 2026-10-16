import path from "node:path";

import type { WorkspaceResourceScopes } from "./workspace-resource-scope";
import { canonicalDirectory, within } from "./workspace-files";
import * as git from "./source-control-git";

type ResourceAccess = Pick<WorkspaceResourceScopes, "open" | "verifyRoot">;

/** Git 查询先绑定 Core 授权的 Chat 根目录，再选择其中的仓库。 */
export class ScopedGit {
  constructor(private readonly resources: ResourceAccess) {}

  private async repository(chatId: string, root: string, selected?: string) {
    const scope = await this.resources.open(chatId);
    const authorized = await this.resources.verifyRoot(scope.id, root);
    const discovered = await git.repoRoots(authorized);
    const repositories: string[] = [];
    for (const candidate of discovered) {
      try {
        const canonical = await canonicalDirectory(candidate);
        if (within(authorized, canonical) && !repositories.includes(canonical))
          repositories.push(canonical);
      } catch {
        // 已消失或越界的工作树不进入可选仓库列表。
      }
    }
    if (selected) {
      const canonical = await canonicalDirectory(selected);
      if (!repositories.includes(canonical))
        throw new Error("Git 仓库不属于此 Chat 授权目录");
      return { repository: canonical, repositories };
    }
    return { repository: repositories[0], repositories };
  }

  private relativeFile(value: string): string {
    if (
      !value ||
      path.posix.isAbsolute(value) ||
      path.win32.isAbsolute(value) ||
      value.includes("\\") ||
      value
        .split("/")
        .some((part) => part === "" || part === "." || part === "..")
    )
      throw new Error("Git 文件路径无效");
    return value;
  }

  async status(chatId: string, root: string, selected?: string) {
    const { repository, repositories } = await this.repository(
      chatId,
      root,
      selected,
    );
    if (!repository)
      return {
        isRepo: false,
        entries: [],
        repositories,
      } as git.GitStatusResult;
    const result = await git.status(repository, repository);
    return { ...result, root: repository, repositories };
  }

  async diff(
    chatId: string,
    root: string,
    selected: string,
    relativePath: string,
    staged: boolean,
  ) {
    const { repository } = await this.repository(chatId, root, selected);
    if (!repository) throw new Error("没有授权的 Git 仓库");
    return git.diff(
      repository,
      this.relativeFile(relativePath),
      staged,
      repository,
    );
  }

  async log(
    chatId: string,
    root: string,
    selected: string,
    count: number,
    skip: number,
  ) {
    const { repository } = await this.repository(chatId, root, selected);
    if (!repository) throw new Error("没有授权的 Git 仓库");
    return git.log(repository, count, skip, repository);
  }

  async show(
    chatId: string,
    root: string,
    selected: string,
    revision: string,
    relativePath: string,
  ) {
    const { repository } = await this.repository(chatId, root, selected);
    if (!repository) throw new Error("没有授权的 Git 仓库");
    if (revision !== "HEAD" && !/^[0-9a-f]{7,64}$/i.test(revision))
      throw new Error("Git 版本无效");
    return git.show(
      repository,
      revision,
      this.relativeFile(relativePath),
      repository,
    );
  }

  async commitDiff(
    chatId: string,
    root: string,
    selected: string,
    hash: string,
  ) {
    const { repository } = await this.repository(chatId, root, selected);
    if (!repository) throw new Error("没有授权的 Git 仓库");
    if (!/^[0-9a-f]{7,64}$/i.test(hash)) throw new Error("Git 提交 ID 无效");
    return git.commitDiff(repository, hash, repository);
  }

  private async change(
    chatId: string,
    root: string,
    selected: string,
    relativePath: string,
    side: "staged" | "unstaged",
  ) {
    const { repository } = await this.repository(chatId, root, selected);
    if (!repository) throw new Error("没有授权的 Git 仓库");
    const validPath = this.relativeFile(relativePath);
    const snapshot = await git.status(repository, repository);
    const entry = snapshot.entries.find((item) => item.path === validPath);
    if (!entry) throw new Error("Git 文件状态已改变，请刷新后重试");
    const staged = entry.xy[0] !== " " && entry.xy[0] !== "?";
    const unstaged =
      entry.xy === "??" || (entry.xy[1] !== " " && entry.xy[1] !== "?");
    if ((side === "staged" && !staged) || (side === "unstaged" && !unstaged))
      throw new Error("Git 文件状态已改变，请刷新后重试");
    return { repository, path: validPath, entry };
  }

  async stage(
    chatId: string,
    root: string,
    selected: string,
    relativePath: string,
  ) {
    const target = await this.change(
      chatId,
      root,
      selected,
      relativePath,
      "unstaged",
    );
    await git.stage(target.repository, target.path, target.repository);
  }

  async unstage(
    chatId: string,
    root: string,
    selected: string,
    relativePath: string,
  ) {
    const target = await this.change(
      chatId,
      root,
      selected,
      relativePath,
      "staged",
    );
    await git.unstage(target.repository, target.path, target.repository);
  }

  async commit(
    chatId: string,
    root: string,
    selected: string,
    message: string,
  ) {
    const { repository } = await this.repository(chatId, root, selected);
    if (!repository) throw new Error("没有授权的 Git 仓库");
    const trimmed = message.trim();
    if (!trimmed || trimmed.length > 4096)
      throw new Error("提交说明需为 1 到 4096 个字符");
    const snapshot = await git.status(repository, repository);
    if (
      !snapshot.entries.some(
        (entry) => entry.xy[0] !== " " && entry.xy[0] !== "?",
      )
    )
      throw new Error("没有已暂存的更改");
    await git.commit(repository, trimmed, repository);
  }

  async discard(
    chatId: string,
    root: string,
    selected: string,
    relativePath: string,
  ) {
    const target = await this.change(
      chatId,
      root,
      selected,
      relativePath,
      "unstaged",
    );
    if (target.entry.xy === "??")
      throw new Error("未跟踪文件不能通过 Git 丢弃");
    await git.discard(target.repository, target.path, target.repository);
  }
}

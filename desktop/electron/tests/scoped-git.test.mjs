import { test } from "node:test";
import assert from "node:assert/strict";
import {
  mkdtemp,
  mkdir,
  readFile,
  realpath,
  rm,
  writeFile,
} from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import { execFileSync } from "node:child_process";
import { DiffFile } from "@git-diff-view/file";
import { loadModule } from "./load-module.mjs";

const { ScopedGit } = loadModule("main/scoped-git.ts");
const chatId = "a".repeat(32);

function statusEntry(
  path,
  xy,
  stagedAddedLines,
  stagedDeletedLines,
  unstagedAddedLines,
  unstagedDeletedLines,
) {
  return {
    path,
    xy,
    stagedAddedLines,
    stagedDeletedLines,
    unstagedAddedLines,
    unstagedDeletedLines,
  };
}

function git(cwd, ...args) {
  return execFileSync("git", args, { cwd, encoding: "utf8" });
}

test("Git 查询只能选当前 Chat 授权目录内的仓库", async (t) => {
  const parent = await realpath(
    await mkdtemp(path.join(tmpdir(), "ally-scoped-git-")),
  );
  t.after(() => rm(parent, { recursive: true, force: true }));
  const root = path.join(parent, "chat");
  const repo = path.join(root, "project");
  const outside = path.join(parent, "outside");
  await mkdir(repo, { recursive: true });
  await mkdir(outside);
  git(repo, "init", "-q");
  git(outside, "init", "-q");
  git(repo, "config", "user.name", "NeuxAlly Test");
  git(repo, "config", "user.email", "ally@example.test");
  await writeFile(path.join(repo, "a.txt"), "before\n");
  git(repo, "add", "a.txt");
  git(repo, "commit", "-qm", "initial");
  await writeFile(path.join(repo, "a.txt"), "after\n");

  const service = new ScopedGit({
    async open(id) {
      assert.equal(id, chatId);
      return { id: "scope", root };
    },
    async verifyRoot(id, requested) {
      assert.equal(id, "scope");
      if (requested !== root) throw new Error("未授权的目录");
      return root;
    },
  });
  const status = await service.status(chatId, root);
  assert.equal(status.isRepo, true);
  assert.deepEqual(status.repositories, [repo]);
  assert.deepEqual(status.entries, [statusEntry("a.txt", " M", 0, 0, 1, 1)]);
  const patch = await service.diff(chatId, root, repo, "a.txt", false);
  assert.match(patch, /after/);
  for (const mode of ["unified", "split"]) {
    const preview = new DiffFile("a.txt", "", "a.txt", "", [patch]);
    preview.initTheme("light");
    preview.init();
    if (mode === "split") preview.buildSplitDiffLines();
    else preview.buildUnifiedDiffLines();
    assert.ok(preview.diffLineLength > 0);
  }
  assert.equal(
    (await service.log(chatId, root, repo, 20, 0))[0].subject,
    "initial",
  );
  assert.equal(
    await service.show(chatId, root, repo, "HEAD", "a.txt"),
    "before\n",
  );
  await assert.rejects(service.status(chatId, root, outside), /不属于/);
  await assert.rejects(service.status(chatId, outside), /未授权/);
  await assert.rejects(
    service.diff(chatId, root, repo, "../outside/a.txt", false),
    /路径无效/,
  );
  await assert.rejects(
    service.show(chatId, root, repo, "--all", "a.txt"),
    /版本无效/,
  );
  assert.equal(await readFile(path.join(repo, "a.txt"), "utf8"), "after\n");
  await service.stage(chatId, root, repo, "a.txt");
  assert.deepEqual((await service.status(chatId, root, repo)).entries, [
    statusEntry("a.txt", "M ", 1, 1, 0, 0),
  ]);
  await service.unstage(chatId, root, repo, "a.txt");
  assert.deepEqual((await service.status(chatId, root, repo)).entries, [
    statusEntry("a.txt", " M", 0, 0, 1, 1),
  ]);
  await assert.rejects(service.commit(chatId, root, repo, "empty"), /已暂存/);
  await service.stage(chatId, root, repo, "a.txt");
  await service.commit(chatId, root, repo, "second");
  assert.equal(
    (await service.log(chatId, root, repo, 1, 0))[0].subject,
    "second",
  );
  await writeFile(path.join(repo, "a.txt"), "discard me\n");
  await service.discard(chatId, root, repo, "a.txt");
  assert.equal(await readFile(path.join(repo, "a.txt"), "utf8"), "after\n");
  await writeFile(path.join(repo, "a[1].txt"), "literal\n");
  await writeFile(path.join(repo, "a1.txt"), "different\n");
  await service.stage(chatId, root, repo, "a[1].txt");
  assert.deepEqual(
    (await service.status(chatId, root, repo)).entries.sort((a, b) =>
      a.path.localeCompare(b.path),
    ),
    [
      statusEntry("a[1].txt", "A ", 1, 0, 0, 0),
      statusEntry("a1.txt", "??", 0, 0, 1, 0),
    ].sort((a, b) => a.path.localeCompare(b.path)),
  );
  const untrackedPatch = await service.diff(
    chatId,
    root,
    repo,
    "a1.txt",
    false,
  );
  assert.match(untrackedPatch, /\+different/);
  const untrackedPreview = new DiffFile("a1.txt", "", "a1.txt", "", [
    untrackedPatch,
  ]);
  untrackedPreview.initTheme("light");
  untrackedPreview.init();
  untrackedPreview.buildUnifiedDiffLines();
  assert.ok(untrackedPreview.diffLineLength > 0);
  await assert.rejects(service.stage(chatId, root, outside, "a.txt"), /不属于/);
});

test("嵌套在未授权上层仓库的聊天目录不读取上层 Git 状态", async (t) => {
  const parent = await realpath(
    await mkdtemp(path.join(tmpdir(), "ally-scoped-git-parent-")),
  );
  t.after(() => rm(parent, { recursive: true, force: true }));
  git(parent, "init", "-q");
  const root = path.join(parent, "chat");
  await mkdir(root);
  const service = new ScopedGit({
    open: async () => ({ id: "scope", root }),
    verifyRoot: async () => root,
  });
  assert.deepEqual(await service.status(chatId, root), {
    isRepo: false,
    entries: [],
    repositories: [],
  });
});

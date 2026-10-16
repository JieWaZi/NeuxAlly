import { test } from "node:test";
import assert from "node:assert/strict";
import {
  mkdtemp,
  mkdir,
  readFile,
  readdir,
  realpath,
  rm,
  symlink,
  stat,
  truncate,
  writeFile,
} from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";

import { loadModule } from "./load-module.mjs";

const { WorkspaceResourceScopes } = loadModule(
  "main/workspace-resource-scope.ts",
);
const chatId = "a".repeat(32);

async function fixture(t) {
  const top = await realpath(
    await mkdtemp(path.join(tmpdir(), "ally-resource-")),
  );
  const root = path.join(top, "workspace");
  await mkdir(root);
  t.after(() => rm(top, { recursive: true, force: true }));
  return { top, root };
}

function manager(root, events = [], additionalWorkingDirectories = []) {
  const calls = [];
  const resources = new WorkspaceResourceScopes(
    {
      connection: async () => ({
        baseUrl: "http://127.0.0.1:8432",
        token: "secret",
      }),
    },
    (event) => events.push(event),
    async (url, options) => {
      calls.push({ url, options });
      return Response.json({
        chat: {
          id: chatId,
          workspaceDirectory: root,
          additionalWorkingDirectories,
        },
      });
    },
  );
  return { resources, calls };
}

test("拖入文件字节仅写入授权目录且不会覆盖现有文件", async (t) => {
  const { top, root } = await fixture(t);
  await mkdir(path.join(root, "docs"));
  const { resources } = manager(root);
  t.after(() => resources.dispose());
  const scope = await resources.open(chatId);
  const bytes = new TextEncoder().encode("hello");

  assert.equal(
    await resources.importBytes(scope.id, "docs", "报告 #1.txt", bytes),
    "docs/报告 #1.txt",
  );
  assert.equal(
    await readFile(path.join(root, "docs", "报告 #1.txt"), "utf8"),
    "hello",
  );
  await assert.rejects(
    resources.importBytes(scope.id, "docs", "报告 #1.txt", bytes),
    /EEXIST/,
  );
  await assert.rejects(
    resources.importBytes(scope.id, "docs", "../outside.txt", bytes),
    /文件名无效/,
  );
  await assert.rejects(
    resources.importBytes(scope.id, "docs", "nul\0name", bytes),
    /文件名无效/,
  );
  await assert.rejects(
    resources.importBytes(scope.id, "../", "outside.txt", bytes),
    /不合法/,
  );
  await assert.rejects(
    resources.importBytes(
      scope.id,
      "docs",
      "too-large.txt",
      new Uint8Array(64 * 1024 * 1024 + 1),
    ),
    /64 MiB/,
  );
  await assert.rejects(
    resources.importBytes(scope.id, "docs", "outside.txt", bytes, top),
    /不属于此 Chat 范围/,
  );
  await writeFile(path.join(top, "private.txt"), "private");
  await symlink(
    path.join(top, "private.txt"),
    path.join(root, "docs", "escape.txt"),
  );
  await assert.rejects(
    resources.importBytes(scope.id, "docs", "escape.txt", bytes),
    /超出/,
  );
  assert.equal(
    await readFile(path.join(top, "private.txt"), "utf8"),
    "private",
  );
  assert.deepEqual(
    (await readdir(path.join(root, "docs"))).sort(),
    ["escape.txt", "报告 #1.txt"].sort(),
  );
});

test("项目软链接中的文件与文件树使用同一引用范围", async (t) => {
  const { top, root } = await fixture(t);
  const project = path.join(top, "project");
  await mkdir(project);
  await writeFile(path.join(project, "AGENTS.md"), "instructions");
  await mkdir(path.join(root, "projects"));
  await symlink(project, path.join(root, "projects", "ally"));
  const { resources } = manager(root);
  t.after(() => resources.dispose());
  const scope = await resources.open(chatId);

  const [entry] = await resources.search(scope.id, "AGENTS", 20, "projects");
  assert.equal(entry?.relativePath, "projects/ally/AGENTS.md");
  const reference = await resources.resolveReference(
    scope.id,
    entry.relativePath,
    root,
  );
  assert.equal(reference.status, "resolved");
  assert.equal(reference.canonicalPath, path.join(project, "AGENTS.md"));
  assert.equal(reference.sourceRoots.includes(project), true);
  assert.equal(
    (
      await resources.resolveReference(
        scope.id,
        path.join(project, "AGENTS.md"),
        root,
      )
    ).status,
    "resolved",
  );
});

test("空文件查询优先返回浅层文件而不遍历整个依赖目录", async (t) => {
  const { root } = await fixture(t);
  await writeFile(path.join(root, "z.txt"), "root");
  await mkdir(path.join(root, "node_modules"));
  await writeFile(path.join(root, "node_modules", "a.txt"), "dependency");
  const { resources } = manager(root);
  t.after(() => resources.dispose());
  const scope = await resources.open(chatId);
  const result = await resources.search(scope.id, "", 1, "");
  assert.deepEqual(
    result.map((entry) => entry.relativePath),
    ["z.txt"],
  );
  assert.deepEqual(await resources.search(scope.id, "a.txt", 20, ""), []);
});

test("文件搜索复用索引，并在失效后返回最新文件", async (t) => {
  const { root } = await fixture(t);
  await mkdir(path.join(root, "docs"));
  const { resources } = manager(root);
  t.after(() => resources.dispose());
  const scope = await resources.open(chatId);
  assert.deepEqual(await resources.search(scope.id, "fresh", 20, ""), []);
  await writeFile(path.join(root, "fresh-external.txt"), "external");
  assert.deepEqual(await resources.search(scope.id, "fresh", 20, ""), []);
  await resources.importBytes(
    scope.id,
    "docs",
    "fresh-imported.txt",
    new TextEncoder().encode("imported"),
  );
  const result = await resources.search(scope.id, "fresh", 20, "");
  assert.deepEqual(result.map((entry) => entry.relativePath).sort(), [
    "docs/fresh-imported.txt",
    "fresh-external.txt",
  ]);
});

test("文件搜索支持 模糊匹配与目录查询", async (t) => {
  const { root } = await fixture(t);
  await mkdir(path.join(root, "src"));
  await writeFile(
    path.join(root, "src", "workspace-file-browser.tsx"),
    "source",
  );
  await writeFile(path.join(root, "src", "unrelated.ts"), "source");
  const { resources } = manager(root);
  t.after(() => resources.dispose());
  const scope = await resources.open(chatId);
  assert.deepEqual(
    (await resources.search(scope.id, "wfb", 20, "")).map(
      (entry) => entry.relativePath,
    ),
    ["src/workspace-file-browser.tsx"],
  );
  assert.deepEqual(
    (await resources.search(scope.id, "src/workspace", 20, "")).map(
      (entry) => entry.relativePath,
    ),
    ["src/workspace-file-browser.tsx"],
  );
});

test("Main 从 Core 的 Chat 身份颁发范围，读取保留版本与完整 bytes", async (t) => {
  const { top, root } = await fixture(t);
  await writeFile(path.join(root, "报告 #1.pdf"), "first");
  const project = path.join(top, "project");
  await mkdir(project);
  await writeFile(path.join(project, "source.txt"), "source");
  const { resources, calls } = manager(root, [], [project]);
  t.after(() => resources.dispose());
  const [scope, concurrent] = await Promise.all([
    resources.open(chatId),
    resources.open(chatId),
  ]);
  assert.equal(concurrent.id, scope.id);
  assert.equal(scope.root, root);
  assert.equal(await resources.verifyRoot(scope.id, project), project);
  await assert.rejects(
    resources.verifyRoot(scope.id, top),
    /不属于此 Chat 范围/,
  );
  assert.equal(calls[0].url, `http://127.0.0.1:8432/api/v1/chats/${chatId}`);
  assert.equal(calls[0].options.headers.Authorization, "Bearer secret");
  assert.equal((await resources.open(chatId)).id, scope.id);
  assert.equal(calls.length, 1);

  const before = await resources.stat(scope.id, "报告 #1.pdf");
  assert.equal(before.mediaType, "application/pdf");
  assert.equal(
    await resources.filePath(scope.id, "报告 #1.pdf"),
    path.join(root, "报告 #1.pdf"),
  );
  const first = await resources.read(
    scope.id,
    "报告 #1.pdf",
    crypto.randomUUID(),
  );
  assert.equal(Buffer.from(first.bytes).toString(), "first");
  assert.equal(first.version, before.version);
  await writeFile(path.join(root, "报告 #1.pdf"), "second version");
  const after = await resources.stat(scope.id, "报告 #1.pdf");
  assert.notEqual(after.version, before.version);
  await assert.rejects(
    resources.write(scope.id, "报告 #1.pdf", "stale", before.version, 1),
    /已被外部修改/,
  );
  const saved = await resources.write(
    scope.id,
    "报告 #1.pdf",
    "saved",
    after.version,
    7,
  );
  assert.equal(saved.saveRevision, 7);
  assert.notEqual(saved.resource.version, after.version);
  assert.equal(await readFile(path.join(root, "报告 #1.pdf"), "utf8"), "saved");
  const parallel = await Promise.allSettled([
    resources.write(
      scope.id,
      "报告 #1.pdf",
      "winner",
      saved.resource.version,
      8,
    ),
    resources.write(
      scope.id,
      "报告 #1.pdf",
      "loser",
      saved.resource.version,
      9,
    ),
  ]);
  assert.equal(
    parallel.filter((result) => result.status === "fulfilled").length,
    1,
  );
  assert.equal(
    parallel.filter((result) => result.status === "rejected").length,
    1,
  );
  const accepted = parallel.find((result) => result.status === "fulfilled");
  assert.equal(
    await readFile(path.join(root, "报告 #1.pdf"), "utf8"),
    accepted?.value.saveRevision === 8 ? "winner" : "loser",
  );
  const projectRead = await resources.read(
    scope.id,
    "source.txt",
    crypto.randomUUID(),
    project,
  );
  assert.equal(Buffer.from(projectRead.bytes).toString(), "source");
  const projectReference = await resources.resolveReference(
    scope.id,
    "source.txt",
    project,
  );
  assert.equal(projectReference.status, "resolved");
  assert.equal(
    projectReference.canonicalPath,
    path.join(project, "source.txt"),
  );
  await assert.rejects(
    resources.resolveReference(scope.id, "source.txt", top),
    /不属于此 Chat 范围/,
  );
  await assert.rejects(
    resources.read(scope.id, "source.txt", crypto.randomUUID(), top),
    /不属于此 Chat 范围/,
  );
  const projectBefore = await resources.stat(scope.id, "source.txt", project);
  const renamed = await resources.rename(
    scope.id,
    "source.txt",
    "来源 #1.txt",
    projectBefore.version,
    project,
  );
  assert.equal(renamed.relativePath, "来源 #1.txt");
  assert.equal(
    await readFile(path.join(project, "来源 #1.txt"), "utf8"),
    "source",
  );
  await assert.rejects(
    resources.removeFile(
      scope.id,
      "来源 #1.txt",
      projectBefore.version,
      project,
    ),
    /已变化/,
  );
  await resources.removeFile(scope.id, "来源 #1.txt", renamed.version, project);
  await assert.rejects(stat(path.join(project, "来源 #1.txt")), /ENOENT/);

  await symlink(path.join(top, "outside.txt"), path.join(root, "escape"));
  await writeFile(path.join(top, "outside.txt"), "private");
  await assert.rejects(
    resources.read(scope.id, "escape", crypto.randomUUID()),
    /超出/,
  );
  await assert.rejects(resources.filePath(scope.id, "escape"), /超出/);
  assert.deepEqual(
    (await resources.list(scope.id, "")).map((entry) => entry.relativePath),
    ["报告 #1.pdf"],
  );
  assert.deepEqual(
    (await resources.search(scope.id, "报告", 20, "")).map(
      (entry) => entry.relativePath,
    ),
    ["报告 #1.pdf"],
  );
  assert.deepEqual(
    (await resources.list(scope.id, "", project)).map(
      (entry) => entry.relativePath,
    ),
    [],
  );
  await assert.rejects(resources.list(scope.id, "", top), /不属于此 Chat 范围/);
  await assert.rejects(
    resources.search(scope.id, "", 20, "../", project),
    /不合法/,
  );
  await assert.rejects(resources.stat(scope.id, "../outside.txt"), /不合法/);
  await symlink(path.join(root, "报告 #1.pdf"), path.join(root, "inside-link"));
  await assert.rejects(
    resources.rename(
      scope.id,
      "inside-link",
      "new-link",
      (await resources.stat(scope.id, "inside-link")).version,
    ),
    /软链接/,
  );
  assert.equal(
    await readFile(path.join(top, "outside.txt"), "utf8"),
    "private",
  );
  const large = path.join(root, "large.pdf");
  await writeFile(large, "");
  await truncate(large, 64 * 1024 * 1024 + 1);
  await assert.rejects(
    resources.read(scope.id, "large.pdf", crypto.randomUUID()),
    /64 MiB/,
  );
  const exported = path.join(top, "large exported.pdf");
  await resources.copyTo(scope.id, "large.pdf", exported);
  assert.equal((await stat(exported)).size, 64 * 1024 * 1024 + 1);
  const imported = await resources.importFiles(
    scope.id,
    "",
    [exported],
    project,
  );
  assert.deepEqual(imported, {
    imported: ["large exported.pdf"],
    failed: [],
  });
  assert.equal(
    (await stat(path.join(project, "large exported.pdf"))).size,
    64 * 1024 * 1024 + 1,
  );
  const duplicate = await resources.importFiles(
    scope.id,
    "",
    [exported],
    project,
  );
  assert.equal(duplicate.imported.length, 0);
  assert.equal(duplicate.failed[0]?.name, "large exported.pdf");
  await assert.rejects(
    resources.importFiles(scope.id, "", [exported], top),
    /不属于此 Chat 范围/,
  );
  await assert.rejects(
    resources.copyTo(scope.id, "escape", path.join(top, "leak.txt")),
    /超出/,
  );
});

test("目录重命名和递归删除只作用于授权根，检测版本冲突和软链接", async (t) => {
  const { top, root } = await fixture(t);
  const outside = path.join(top, "outside");
  await mkdir(outside);
  await mkdir(path.join(root, "资料"));
  await mkdir(path.join(root, "资料", "内层"));
  await writeFile(path.join(root, "资料", "内层", "a.md"), "draft");
  await symlink(outside, path.join(root, "外部链接"));
  const { resources } = manager(root);
  t.after(() => resources.dispose());
  const scope = await resources.open(chatId);
  await assert.rejects(resources.directoryVersion(scope.id, ""), /根目录/);
  await assert.rejects(
    resources.directoryVersion(scope.id, "../outside"),
    /不合法/,
  );
  await assert.rejects(
    resources.directoryVersion(scope.id, "外部链接"),
    /超出|软链接/,
  );
  await assert.rejects(
    resources.directoryVersion(scope.id, "资料", outside),
    /不属于此 Chat 范围/,
  );
  const version = await resources.directoryVersion(scope.id, "资料");
  await assert.rejects(
    resources.renameDirectory(scope.id, "资料", "../bad", version),
    /不合法/,
  );
  await assert.rejects(
    resources.renameDirectory(scope.id, "资料", "新资料", "stale"),
    /已变化/,
  );
  const renamed = await resources.renameDirectory(
    scope.id,
    "资料",
    "新资料",
    version,
  );
  assert.equal(renamed, "新资料");
  assert.equal(
    await readFile(path.join(root, "新资料", "内层", "a.md"), "utf8"),
    "draft",
  );
  await assert.rejects(
    resources.removeDirectory(scope.id, "新资料", version),
    /已变化/,
  );
  await resources.removeDirectory(
    scope.id,
    "新资料",
    await resources.directoryVersion(scope.id, "新资料"),
  );
  await assert.rejects(stat(path.join(root, "新资料")), /ENOENT/);
  assert.ok((await stat(outside)).isDirectory());
});

test("观察者从 ready 开始递增序号，关闭后不再交付", async (t) => {
  const { root } = await fixture(t);
  await writeFile(path.join(root, "notes.txt"), "before");
  const events = [];
  const { resources } = manager(root, events);
  t.after(() => resources.dispose());
  const scope = await resources.open(chatId);
  const ready = await resources.watch(scope.id, "notes.txt");
  assert.equal(ready.sequence, 0);
  assert.equal(ready.resource.size, 6);
  await writeFile(path.join(root, "notes.txt"), "after change");
  await new Promise((resolve, reject) => {
    const deadline = setTimeout(
      () => reject(new Error("watcher 未收到变化")),
      2000,
    );
    const poll = () => {
      if (
        events.some((event) => event.subscriptionId === ready.subscriptionId)
      ) {
        clearTimeout(deadline);
        resolve();
      } else setTimeout(poll, 10);
    };
    poll();
  });
  assert.ok(events.some((event) => event.sequence > ready.sequence));
  assert.equal(resources.unwatch(scope.id, ready.subscriptionId), true);
  const count = events.length;
  await writeFile(path.join(root, "notes.txt"), "more");
  await new Promise((resolve) => setTimeout(resolve, 50));
  assert.equal(events.length, count);

  const directoryReady = await resources.watch(scope.id, "", undefined, true);
  assert.equal(directoryReady.sequence, 0);
  assert.equal(directoryReady.resource, null);
  await writeFile(path.join(root, "新建 #1.txt"), "hello");
  await new Promise((resolve, reject) => {
    const deadline = setTimeout(
      () => reject(new Error("目录观察者未收到变化")),
      2000,
    );
    const poll = () => {
      if (
        events.some(
          (event) =>
            event.subscriptionId === directoryReady.subscriptionId &&
            event.sequence > 0,
        )
      ) {
        clearTimeout(deadline);
        resolve();
      } else setTimeout(poll, 10);
    };
    poll();
  });
  assert.equal(
    resources.unwatch(scope.id, directoryReady.subscriptionId),
    true,
  );
});

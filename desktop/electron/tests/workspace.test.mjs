import { test } from "node:test";
import assert from "node:assert/strict";
import {
  mkdtemp,
  mkdir,
  writeFile,
  symlink,
  rm,
  readFile,
  realpath,
} from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import { execFileSync } from "node:child_process";
import { loadModule } from "./load-module.mjs";
const files = loadModule("main/workspace-files.ts");
const fixture = async (t) => {
  const root = await realpath(
    await mkdtemp(path.join(tmpdir(), "ally-files-")),
  );
  t.after(() => rm(root, { recursive: true, force: true }));
  return root;
};
test("没有生成文件目录的会话返回空列表，并可随后发现生成文件", async (t) => {
  const root = await fixture(t);
  const empty = { generated: [], removedPaths: [] };
  assert.deepEqual(await files.listChatWorkspaceFiles(root), empty);

  const uploaded = path.join(root, "attachments", "message-1");
  await mkdir(uploaded, { recursive: true });
  await writeFile(path.join(uploaded, "upload.txt"), "upload");
  assert.deepEqual(await files.listChatWorkspaceFiles(root), empty);

  const generated = path.join(root, "attachments", "generated");
  await mkdir(generated);
  await writeFile(path.join(generated, "report.md"), "report");
  assert.deepEqual(
    (await files.listChatWorkspaceFiles(root)).generated.map(
      (entry) => entry.relativePath,
    ),
    ["attachments/generated/report.md"],
  );
});
test("会话列表仍拒绝失效工作区与越界生成目录", async (t) => {
  const top = await fixture(t);
  await assert.rejects(files.listChatWorkspaceFiles(path.join(top, "missing")));
  const root = path.join(top, "workspace");
  const outside = path.join(top, "outside");
  await mkdir(path.join(root, "attachments"), { recursive: true });
  await mkdir(outside);
  await symlink(outside, path.join(root, "attachments", "generated"));
  await assert.rejects(files.listChatWorkspaceFiles(root), /超出/);
});
test("真实路径隔离越界软链接并支持受管项目入口", async (t) => {
  const top = await fixture(t);
  const root = path.join(top, "work");
  const project = path.join(top, "project");
  await mkdir(path.join(root, "projects"), { recursive: true });
  await mkdir(project);
  await writeFile(path.join(project, "a.txt"), "hello");
  await symlink(project, path.join(root, "projects", "repo"));
  await symlink(project, path.join(root, "escape"));
  assert.equal(
    await files.workspacePath(root, "projects/repo/a.txt"),
    path.join(project, "a.txt"),
  );
  await assert.rejects(files.workspacePath(root, "escape/a.txt"), /超出/);
  await assert.rejects(files.workspacePath(root, "../project/a.txt"), /不合法/);
  await symlink(top, path.join(project, "outside"));
  await assert.rejects(
    files.workspacePath(root, "projects/repo/outside"),
    /超出/,
  );
  await files.writeFile(project, "a.txt", "updated");
  assert.equal(await readFile(path.join(project, "a.txt"), "utf8"), "updated");
});
test("会话文件删除只移除受管文件，项目原文件保留且列表记住移除状态", async (t) => {
  const top = await fixture(t);
  const root = path.join(top, "work");
  const project = path.join(top, "project");
  const generated = path.join(root, "attachments", "generated");
  const uploaded = path.join(root, "attachments", "message-1");
  await Promise.all([
    mkdir(generated, { recursive: true }),
    mkdir(uploaded, { recursive: true }),
    mkdir(path.join(root, "projects"), { recursive: true }),
    mkdir(project),
  ]);
  await writeFile(path.join(generated, "report.md"), "report");
  await writeFile(path.join(uploaded, "upload.txt"), "upload");
  await writeFile(path.join(project, "source.txt"), "source");
  await symlink(project, path.join(root, "projects", "repo"));

  assert.deepEqual(
    (await files.listChatWorkspaceFiles(root)).generated.map(
      (entry) => entry.relativePath,
    ),
    ["attachments/generated/report.md"],
  );
  await files.deleteChatWorkspaceFile(root, "attachments/message-1/upload.txt");
  await assert.rejects(readFile(path.join(uploaded, "upload.txt")));
  await files.deleteChatWorkspaceFile(root, "projects/repo/source.txt");
  assert.equal(
    await readFile(path.join(project, "source.txt"), "utf8"),
    "source",
  );
  assert.deepEqual((await files.listChatWorkspaceFiles(root)).removedPaths, [
    "attachments/message-1/upload.txt",
    "projects/repo/source.txt",
  ]);
  await files.deleteChatWorkspaceFile(root, "attachments/generated/report.md");
  assert.deepEqual((await files.listChatWorkspaceFiles(root)).generated, []);
  await writeFile(path.join(uploaded, "upload.txt"), "restored");
  assert.deepEqual((await files.listChatWorkspaceFiles(root)).removedPaths, [
    "projects/repo/source.txt",
    "attachments/generated/report.md",
  ]);
  await symlink(
    path.join(project, "source.txt"),
    path.join(uploaded, "escape"),
  );
  await assert.rejects(
    files.deleteChatWorkspaceFile(root, "attachments/message-1/escape"),
    /超出|附件目录/,
  );
  const manifest = path.join(root, "attachments", "removed-chat-files.json");
  const outsideManifest = path.join(top, "outside.json");
  await writeFile(outsideManifest, "[]");
  await rm(manifest);
  await symlink(outsideManifest, manifest);
  await assert.rejects(files.listChatWorkspaceFiles(root), /清单不能是软链接/);
  await assert.rejects(
    files.deleteChatWorkspaceFile(root, "attachments/message-1/upload.txt"),
    /清单不能是软链接/,
  );
  assert.equal(
    await readFile(path.join(uploaded, "upload.txt"), "utf8"),
    "restored",
  );
});
test("团队成员工作区可以列出共享项目命名空间", async (t) => {
  const top = await fixture(t);
  const workspace = path.join(top, "workspace");
  const projects = path.join(workspace, "projects");
  const member = path.join(workspace, "members", "waker-1");
  const project = path.join(top, "project");
  await Promise.all([
    mkdir(project),
    mkdir(projects, { recursive: true }),
    mkdir(member, { recursive: true }),
  ]);
  await writeFile(path.join(project, "a.txt"), "hello");
  await symlink(project, path.join(projects, "repo"));
  await symlink(path.relative(member, projects), path.join(member, "projects"));
  assert.equal(await files.workspacePath(member, "projects"), projects);
  assert.deepEqual(await files.listDirectory(member, "projects"), [
    {
      name: "repo",
      relativePath: "projects/repo",
      isDirectory: true,
    },
  ]);
  assert.equal(
    await files.workspacePath(member, "projects/repo/a.txt"),
    path.join(project, "a.txt"),
  );
  await symlink(top, path.join(project, "outside"));
  await assert.rejects(
    files.workspacePath(member, "projects/repo/outside"),
    /超出/,
  );
});
test("文件引用按真实身份去重并区分范围外与歧义", async (t) => {
  const root = await fixture(t);
  const first = path.join(root, "first"),
    second = path.join(root, "second"),
    work = path.join(root, "work");
  await Promise.all([mkdir(first), mkdir(second), mkdir(work)]);
  await writeFile(path.join(first, "a.txt"), "a");
  await writeFile(path.join(second, "a.txt"), "b");
  const request = {
    path: "a.txt",
    baseDirectory: work,
    workspaceRoot: work,
    sourceRoots: [first, second],
  };
  assert.equal((await files.resolveFileReference(request)).status, "ambiguous");
  const resolved = await files.resolveFileReference({
    ...request,
    sourceRoots: [first, first],
  });
  assert.equal(resolved.status, "resolved");
  assert.equal(resolved.canonicalPath, path.join(first, "a.txt"));
  assert.equal(
    (
      await files.resolveFileReference({
        ...request,
        path: path.join(second, "a.txt"),
        sourceRoots: [first],
      })
    ).status,
    "outside_scope",
  );
});
test("文件搜索不会在循环项目链接中重复遍历", async (t) => {
  const root = await fixture(t);
  await mkdir(path.join(root, "projects"));
  await writeFile(path.join(root, "note.txt"), "note");
  await symlink(root, path.join(root, "projects", "loop"));
  const matches = await files.searchFiles(root, "note", 200);
  assert.equal(matches.length, 1);
  assert.equal((await files.readFile(root, "note.txt")).content, "note");
  await writeFile(path.join(root, "binary"), Buffer.from([255, 254, 0]));
  assert.equal((await files.readFile(root, "binary")).kind, "binary");
});
test("Git 变更与 Diff 保留空格、删除和未跟踪文件", async (t) => {
  const root = await fixture(t);
  const git = (...args) =>
    execFileSync("git", ["-C", root, ...args], { stdio: "pipe" });
  git("init", "-b", "main");
  git("config", "user.name", "NeuxAlly Test");
  git("config", "user.email", "test@example.invalid");
  await writeFile(path.join(root, "a file.txt"), "before\n");
  await writeFile(path.join(root, "deleted.txt"), "deleted\n");
  git("add", ".");
  git("commit", "-m", "initial");
  await writeFile(path.join(root, "a file.txt"), "after\n");
  await rm(path.join(root, "deleted.txt"));
  await writeFile(path.join(root, "new.txt"), "new");
  const changes = await files.gitChanges(root);
  assert.equal(changes.branch, "main");
  assert.equal(changes.files.length, 3);
  assert.equal(
    changes.files.find((f) => f.path === "deleted.txt").status,
    "deleted",
  );
  const diff = await files.readDiff(root, "a file.txt");
  assert.equal(diff.oldContent, "before\n");
  assert.equal(diff.newContent, "after\n");
  assert.equal((await files.readDiff(root, "deleted.txt")).newContent, "");
});
test("Git 分支列表、冲突预览与切换", async (t) => {
  const root = await fixture(t);
  const git = (...args) =>
    execFileSync("git", ["-C", root, ...args], { stdio: "pipe" });
  git("init", "-b", "main");
  git("config", "user.name", "NeuxAlly Test");
  git("config", "user.email", "test@example.invalid");
  await writeFile(path.join(root, "README.md"), "main\n");
  git("add", ".");
  git("commit", "-m", "initial");
  git("branch", "feature/login");
  git("checkout", "feature/login");
  await writeFile(path.join(root, "README.md"), "feature\n");
  git("add", "README.md");
  git("commit", "-m", "feature");
  git("checkout", "main");

  const branches = await files.listGitBranches(root);
  assert.equal(branches.isGitRepo, true);
  assert.ok(branches.branches.some((branch) => branch.name === "main"));
  assert.ok(
    branches.branches.some((branch) => branch.name === "feature/login"),
  );

  await writeFile(path.join(root, "README.md"), "dirty\n");
  const preview = await files.previewGitCheckout(root, "feature/login");
  assert.equal(preview.canCheckout, false);
  assert.deepEqual(
    preview.conflicts.map((file) => file.path),
    ["README.md"],
  );

  await files.checkoutGitBranch(
    root,
    "feature/login",
    "保存本地更改后切换分支",
  );
  assert.equal((await files.gitChanges(root)).branch, "feature/login");
  assert.equal(
    await readFile(path.join(root, "README.md"), "utf8"),
    "feature\n",
  );
});
test("多目录项目按 Git 根去重并分别返回分支", async (t) => {
  const top = await fixture(t);
  const frontend = path.join(top, "frontend");
  const backend = path.join(top, "backend");
  await mkdir(frontend, { recursive: true });
  await mkdir(backend, { recursive: true });
  const initRepo = async (dir, content) => {
    const git = (...args) =>
      execFileSync("git", ["-C", dir, ...args], { stdio: "pipe" });
    git("init", "-b", "main");
    git("config", "user.name", "NeuxAlly Test");
    git("config", "user.email", "test@example.invalid");
    await writeFile(path.join(dir, "README.md"), content);
    git("add", ".");
    git("commit", "-m", "initial");
  };
  await initRepo(frontend, "frontend\n");
  await initRepo(backend, "backend\n");
  execFileSync("git", ["-C", backend, "checkout", "-b", "develop"], {
    stdio: "pipe",
  });

  const repositories = await files.listGitRepositories([frontend, backend]);
  assert.equal(repositories.repositories.length, 2);
  assert.deepEqual(repositories.repositories.map((repo) => repo.label).sort(), [
    "backend",
    "frontend",
  ]);
  assert.equal(
    repositories.repositories.find((repo) => repo.label === "frontend")?.branch,
    "main",
  );
  assert.equal(
    repositories.repositories.find((repo) => repo.label === "backend")?.branch,
    "develop",
  );

  const monorepo = path.join(top, "mono");
  await mkdir(path.join(monorepo, "apps", "api"), { recursive: true });
  await initRepo(monorepo, "mono\n");
  const deduped = await files.listGitRepositories([
    monorepo,
    path.join(monorepo, "apps", "api"),
  ]);
  assert.equal(deduped.repositories.length, 1);
  assert.equal(deduped.repositories[0]?.label, "mono");
});
test("界面协议拒绝越界资源并只授权实际首帧脚本", async (t) => {
  const { rendererFile, rendererCSP } = loadModule("main/renderer-protocol.ts");
  const root = await fixture(t);
  const dist = path.join(root, "dist");
  await mkdir(dist);
  await writeFile(path.join(root, "secret"), "private");
  await writeFile(
    path.join(dist, "index.html"),
    "<script>document.title='NeuxAlly'</script>",
  );
  await symlink(path.join(root, "secret"), path.join(dist, "escape"));
  assert.equal(
    await rendererFile(dist, "ally://app/index.html"),
    path.join(dist, "index.html"),
  );
  await assert.rejects(rendererFile(dist, "ally://app/escape"), /超出/);
  const csp = await rendererCSP(path.join(dist, "index.html"));
  assert.match(csp, /script-src 'self' 'sha256-/);
  assert.doesNotMatch(csp, /script-src[^;]*unsafe-inline/);
  assert.match(csp, /frame-src blob: http: https:/);
  assert.match(csp, /object-src 'none'/);
});

test("账号原生凭据不进入文件列表、预览或消息引用", async (t) => {
  const root = await fixture(t);
  const home = path.join(root, "runtime", "accounts", "codex");
  await mkdir(home, { recursive: true });
  await writeFile(path.join(home, "auth.json"), "private");
  await symlink(home, path.join(root, "account-alias"));
  assert.deepEqual(await files.listDirectory(root, "runtime"), []);
  assert.deepEqual(await files.searchFiles(root, "auth", 200), []);
  await assert.rejects(
    files.workspacePath(root, "runtime/accounts/codex/auth.json"),
    /账号配置/,
  );
  await assert.rejects(
    files.workspacePath(root, "account-alias/auth.json"),
    /账号配置/,
  );
  assert.equal(
    (
      await files.resolveFileReference({
        workspaceRoot: root,
        baseDirectory: root,
        sourceRoots: [],
        path: "account-alias/auth.json",
      })
    ).status,
    "outside_scope",
  );
});

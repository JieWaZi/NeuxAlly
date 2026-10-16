import { test } from "node:test";
import assert from "node:assert/strict";
import {
  mkdtemp,
  mkdir,
  writeFile,
  realpath,
  rm,
  rename,
} from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import { execFileSync } from "node:child_process";
import { loadModule } from "./load-module.mjs";

const { WorkspaceResourceWatchHub } = loadModule(
  "main/workspace-resource-watch.ts",
);
const { WorkspaceResourceScopes } = loadModule(
  "main/workspace-resource-scope.ts",
);
const chatId = "b".repeat(32);
const git = (root, ...args) =>
  execFileSync("git", ["-C", root, ...args], { stdio: "pipe" });
async function fixture(t) {
  const top = await realpath(await mkdtemp(path.join(tmpdir(), "ally-watch-")));
  t.after(() => rm(top, { recursive: true, force: true }));
  const root = path.join(top, "workspace");
  await mkdir(root);
  return { root, top };
}
async function waitFor(predicate, message = "原生文件事件未到达") {
  const deadline = Date.now() + 4000;
  while (!predicate()) {
    if (Date.now() >= deadline) throw new Error(message);
    await new Promise((resolve) => setTimeout(resolve, 15));
  }
}

test("同根消费者共享观察，深层文件和新目录更新后最后退订释放", async (t) => {
  const { root } = await fixture(t);
  await mkdir(path.join(root, "nested", "deep"), { recursive: true });
  const hub = new WorkspaceResourceWatchHub();
  t.after(() => hub.close());
  let first = 0;
  let second = 0;
  const stopA = await hub.subscribe(root, (state) => {
    if (state === "changed") first++;
  });
  const stopB = await hub.subscribe(root, (state) => {
    if (state === "changed") second++;
  });
  assert.equal(hub.trees.size, 1);
  const before = first;
  await writeFile(
    path.join(root, "nested", "deep", "external.txt"),
    "external",
  );
  await waitFor(() => first > before && second > 0);
  const after = first;
  stopA();
  await mkdir(path.join(root, "new-directory"));
  const beforeSecond = second;
  await writeFile(path.join(root, "new-directory", "new.txt"), "created");
  await waitFor(() => second > beforeSecond);
  assert.equal(first, after);
  stopB();
  assert.equal(hub.trees.size, 0);
});

test("授权根被删除替换后由父目录真实事件重建，恢复深层修改", async (t) => {
  const { root, top } = await fixture(t);
  const hub = new WorkspaceResourceWatchHub();
  t.after(() => hub.close());
  const states = [];
  await hub.subscribe(root, (state) => states.push(state));
  await rename(root, path.join(top, "previous"));
  await waitFor(() => states.includes("recovering"));
  await mkdir(root);
  await mkdir(path.join(root, "replacement"));
  await waitFor(() => states.at(-1) === "changed");
  const before = states.length;
  await writeFile(path.join(root, "replacement", "restored.txt"), "restored");
  await waitFor(() => states.length > before && states.at(-1) === "changed");
});

test("linked worktree 的外部 HEAD/index 和共同 refs 都会失效，不依靠文件轮询", async (t) => {
  const { root, top } = await fixture(t);
  const repo = path.join(top, "repository");
  await mkdir(repo);
  git(repo, "init", "--quiet", "-b", "main");
  git(repo, "config", "user.email", "test@example.com");
  git(repo, "config", "user.name", "Test");
  await writeFile(path.join(repo, "tracked.txt"), "tracked\n");
  git(repo, "add", ".");
  git(repo, "commit", "--quiet", "-m", "fixture");
  await rm(root, { recursive: true });
  git(repo, "worktree", "add", "--quiet", "-b", "work", root);
  const hub = new WorkspaceResourceWatchHub();
  t.after(() => hub.close());
  let updates = 0;
  await hub.subscribe(root, (state) => {
    if (state === "changed") updates++;
  });
  const beforeRef = updates;
  git(repo, "branch", "external-ref");
  await waitFor(
    () => updates > beforeRef,
    "共同 refs 变化未更新 linked worktree",
  );
  const beforeHead = updates;
  git(root, "switch", "--quiet", "-c", "next");
  await waitFor(() => updates > beforeHead, "外部 HEAD 变化未更新");
  const beforeIndex = updates;
  // 只调整 index，工作树字节完全不变。
  git(root, "update-index", "--assume-unchanged", "tracked.txt");
  await waitFor(() => updates > beforeIndex, "外部 index 变化未更新");
});

test("已授权外部项目纳入观察，未授权目录请求仍拒绝", async (t) => {
  const { root, top } = await fixture(t);
  const project = path.join(top, "project");
  const outside = path.join(top, "outside");
  await mkdir(project);
  await mkdir(outside);
  const events = [];
  const scopes = new WorkspaceResourceScopes(
    {
      connection: async () => ({
        baseUrl: "http://127.0.0.1",
        token: "secret",
      }),
    },
    (event) => events.push(event),
    async () =>
      Response.json({
        chat: {
          id: chatId,
          workspaceDirectory: root,
          additionalWorkingDirectories: [project],
        },
      }),
  );
  t.after(() => scopes.dispose());
  const scope = await scopes.open(chatId);
  const ready = await scopes.watch(scope.id, "", undefined, true);
  await assert.rejects(
    scopes.watch(scope.id, "", outside, true),
    /不属于此 Chat/,
  );
  const before = events.length;
  await writeFile(path.join(project, "external.txt"), "external");
  await waitFor(
    () =>
      events.length > before &&
      events.some(
        (event) =>
          event.subscriptionId === ready.subscriptionId && event.sequence > 0,
      ),
  );
});

test("具体文件版本不变时不推送资源变化，根内其他文件修改不会刷新预览", async (t) => {
  const { root } = await fixture(t);
  await writeFile(path.join(root, "watched.txt"), "original");
  const events = [];
  const scopes = new WorkspaceResourceScopes(
    {
      connection: async () => ({
        baseUrl: "http://127.0.0.1",
        token: "secret",
      }),
    },
    (event) => events.push(event),
    async () =>
      Response.json({ chat: { id: chatId, workspaceDirectory: root } }),
  );
  t.after(() => scopes.dispose());
  const scope = await scopes.open(chatId);
  await scopes.watch(scope.id, "", undefined, true);
  const file = await scopes.watch(scope.id, "watched.txt");
  await new Promise((resolve) => setTimeout(resolve, 200));
  events.length = 0;
  await writeFile(path.join(root, "other.txt"), "other");
  await new Promise((resolve) => setTimeout(resolve, 200));
  assert.equal(
    events.filter((event) => event.subscriptionId === file.subscriptionId)
      .length,
    0,
  );
  await writeFile(path.join(root, "watched.txt"), "changed");
  await waitFor(() =>
    events.some(
      (event) =>
        event.subscriptionId === file.subscriptionId &&
        event.resource?.version !== file.resource.version,
    ),
  );
});

test("重载时尚在启动的旧观察清理不能删除新代观察者", async (t) => {
  const { root } = await fixture(t);
  const hub = new WorkspaceResourceWatchHub();
  t.after(() => hub.close());
  const old = hub.subscribe(root, () => {});
  hub.close();
  let updates = 0;
  const stopNew = await hub.subscribe(root, (state) => {
    if (state === "changed") updates++;
  });
  const stopOld = await old;
  stopOld();
  assert.equal(hub.trees.size, 1, "旧订阅清理删除了新代物理观察身份");
  const before = updates;
  await writeFile(path.join(root, "after-reload.txt"), "reload");
  await waitFor(() => updates > before);
  stopNew();
  assert.equal(hub.trees.size, 0);
});

test("Core 连接身份变化后重新核验 Chat 授权，旧 scope 不能继续读文件", async (t) => {
  const { root, top } = await fixture(t);
  const replacement = path.join(top, "replacement-workspace");
  await mkdir(replacement);
  await writeFile(path.join(root, "old.txt"), "old");
  let token = "old-token";
  let checks = 0;
  const scopes = new WorkspaceResourceScopes(
    { connection: async () => ({ baseUrl: "http://127.0.0.1", token }) },
    () => {},
    async () => {
      checks++;
      return Response.json({
        chat: {
          id: chatId,
          workspaceDirectory: token === "old-token" ? root : replacement,
        },
      });
    },
  );
  t.after(() => scopes.dispose());
  const original = await scopes.open(chatId);
  await scopes.watch(original.id, "", undefined, true);
  token = "new-token";
  const current = await scopes.open(chatId);
  assert.notEqual(current.id, original.id);
  assert.equal(current.root, replacement);
  assert.equal(checks, 2);
  await assert.rejects(scopes.stat(original.id, "old.txt"), /范围无效或已关闭/);
});

test("Git 元数据持续失败时有限重试后保持 failed，只有成功恢复才发 changed", async (t) => {
  const { root } = await fixture(t);
  git(root, "init", "--quiet", "-b", "main");
  const { promisify } = await import("node:util");
  const { execFile } = await import("node:child_process");
  let failing = true;
  let attempts = 0;
  const execute = promisify(execFile);
  const guardedExec = () => {};
  guardedExec[promisify.custom] = async (...args) => {
    attempts++;
    if (failing) throw new Error("controlled Git metadata failure");
    return execute(...args);
  };
  const { WorkspaceResourceWatchHub: FailedGitHub } = loadModule(
    "main/workspace-resource-watch.ts",
    { "node:child_process": { execFile: guardedExec } },
  );
  const hub = new FailedGitHub();
  t.after(() => hub.close());
  const states = [];
  await hub.subscribe(root, (state) => states.push(state));
  await waitFor(
    () => states.at(-1) === "failed",
    "持续 Git 故障没有进入耗尽状态",
  );
  assert.ok(states.includes("recovering"));
  assert.equal(states.includes("changed"), false, "Git 故障被伪装成已恢复");
  const exhaustedAttempts = attempts;
  await new Promise((resolve) => setTimeout(resolve, 400));
  assert.equal(attempts, exhaustedAttempts, "耗尽后仍在永久循环重试");
  assert.equal(states.at(-1), "failed");
  failing = false;
  await writeFile(path.join(root, "actual-recovery.txt"), "recovered");
  await waitFor(
    () => states.at(-1) === "changed",
    "真实事件且元数据读取成功后没有恢复",
  );
});

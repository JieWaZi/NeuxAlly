import { watch, type FSWatcher } from "node:fs";
import { promises as fs } from "node:fs";
import { execFile } from "node:child_process";
import path from "node:path";
import { promisify } from "node:util";

const execute = promisify(execFile);
const excluded = new Set([
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
]);
export type WorkspaceWatchState = "changed" | "recovering" | "failed";
type Listener = (state: WorkspaceWatchState) => void;

/** 原生目录事件共享物理观察者；Git 元数据只由受管 Git 命令发现，不接收 Renderer 路径。 */
class WorkspaceWatchTree {
  readonly listeners = new Set<Listener>();
  private readonly watchers = new Map<string, FSWatcher>();
  private parent?: FSWatcher;
  private changeTimer?: ReturnType<typeof setTimeout>;
  private retryTimer?: ReturnType<typeof setTimeout>;
  private closed = false;
  state: WorkspaceWatchState = "changed";
  private retries = 0;
  private metadataDirty = true;
  private refreshing = false;

  constructor(private readonly root: string) {}

  async start(): Promise<void> {
    // 父目录只观察授权根的身份变化，让删除/原子替换后的根可以由真实创建事件恢复。
    const basename = path.basename(this.root);
    this.parent = watch(path.dirname(this.root), (_event, filename) => {
      if (!filename || String(filename) === basename) {
        for (const watcher of this.watchers.values()) watcher.close();
        this.watchers.clear();
        this.metadataDirty = true;
        this.retries = 0;
        void this.rebuild();
      }
    });
    this.parent.on("error", () => this.failure());
    await this.rebuild();
  }

  private notify(state: WorkspaceWatchState): void {
    this.state = state;
    if (!this.closed) for (const listener of this.listeners) listener(state);
  }

  private changed(discover = false): void {
    if (this.closed) return;
    this.metadataDirty ||= discover;
    if (this.changeTimer) return;
    this.changeTimer = setTimeout(() => {
      this.changeTimer = undefined;
      if (this.metadataDirty) {
        void this.refreshMetadata().then((ready) => {
          if (ready) this.notify("changed");
        });
      } else if (this.state === "changed") this.notify("changed");
    }, 100);
  }

  private add(directory: string, discover: boolean): void {
    if (this.closed || this.watchers.has(directory)) return;
    if (this.watchers.size >= 128)
      throw new Error("工作区观察目录数量达到上限");
    const watcher = watch(directory, { recursive: true }, (event) => {
      this.changed(discover && event === "rename");
    });
    this.watchers.set(directory, watcher);
    watcher.on("error", () => {
      watcher.close();
      this.watchers.delete(directory);
      this.failure();
    });
  }

  private failure(): void {
    if (this.closed || this.retryTimer) return;
    this.metadataDirty = true;
    this.notify("recovering");
    if (this.retries >= 3) {
      this.notify("failed");
      return;
    }
    // 仅在真实 watcher 故障后有限重试，不定时扫描工作树。
    this.retryTimer = setTimeout(
      () => {
        this.retryTimer = undefined;
        this.retries++;
        this.metadataDirty = true;
        void this.rebuild();
      },
      250 * 2 ** this.retries,
    );
  }

  private async rebuild(): Promise<void> {
    if (this.closed) return;
    try {
      const status = await fs.lstat(this.root);
      if (!status.isDirectory() || status.isSymbolicLink())
        throw new Error("授权工作区根目录身份失效");
      this.add(this.root, true);
      const ready = await this.refreshMetadata();
      if (ready && !this.closed) this.notify("changed");
    } catch {
      for (const watcher of this.watchers.values()) watcher.close();
      this.watchers.clear();
      this.failure();
    }
  }

  private async refreshMetadata(): Promise<boolean> {
    if (this.closed || this.refreshing) return false;
    if (!this.metadataDirty) return this.state === "changed";
    this.refreshing = true;
    this.metadataDirty = false;
    try {
      const roots = await this.discoverRepositories();
      const metadata = new Set<string>();
      for (const directory of roots) {
        const { stdout } = await execute(
          "git",
          [
            "-C",
            directory,
            "rev-parse",
            "--path-format=absolute",
            "--git-dir",
            "--git-common-dir",
          ],
          {
            timeout: 5000,
            maxBuffer: 64 * 1024,
            env: { ...process.env, GIT_TERMINAL_PROMPT: "0" },
          },
        );
        for (const candidate of stdout.trim().split("\n")) {
          const canonical = await fs.realpath(candidate);
          if ((await fs.stat(canonical)).isDirectory()) metadata.add(canonical);
        }
      }
      if (this.closed) return false;
      for (const directory of metadata) this.add(directory, false);
      for (const [directory, watcher] of this.watchers) {
        if (directory !== this.root && !metadata.has(directory)) {
          watcher.close();
          this.watchers.delete(directory);
        }
      }
      this.retries = 0;
      if (this.retryTimer) clearTimeout(this.retryTimer);
      this.retryTimer = undefined;
      return true;
    } catch {
      this.metadataDirty = true;
      this.failure();
      return false;
    } finally {
      this.refreshing = false;
      if (this.metadataDirty && !this.retryTimer && this.retries < 3)
        this.changed();
    }
  }

  private async discoverRepositories(): Promise<string[]> {
    const pending = [this.root];
    const repositories: string[] = [];
    let entries = 0;
    for (let index = 0; index < pending.length; index++) {
      const directory = pending[index];
      if (!directory || this.closed) break;
      const children = await fs.readdir(directory, { withFileTypes: true });
      for (const child of children) {
        if (++entries > 50_000) throw new Error("工作区仓库发现范围达到上限");
        if (child.name === ".git") repositories.push(directory);
        else if (child.isDirectory() && !excluded.has(child.name))
          pending.push(path.join(directory, child.name));
        // 软链接从不扩大授权边界；已授权项目根由外层分别注册。
      }
      if (repositories.length > 64) throw new Error("工作区仓库数量达到上限");
    }
    // 授权根可能只是仓库内的子目录，其 HEAD/index 仍位于父仓库元数据内。
    if (!repositories.includes(this.root)) {
      try {
        await execute(
          "git",
          ["-C", this.root, "rev-parse", "--show-toplevel"],
          { timeout: 5000, maxBuffer: 64 * 1024 },
        );
        repositories.push(this.root);
      } catch {
        // 普通文件目录没有 Git 元数据；其 .git 创建仍由原生根观察者触发发现。
      }
    }
    return repositories;
  }

  async retry(): Promise<void> {
    if (this.closed || this.state === "changed") return;
    if (this.retryTimer) clearTimeout(this.retryTimer);
    this.retryTimer = undefined;
    this.retries = 0;
    this.metadataDirty = true;
    for (const watcher of this.watchers.values()) watcher.close();
    this.watchers.clear();
    await this.rebuild();
  }

  close(): void {
    this.closed = true;
    if (this.changeTimer) clearTimeout(this.changeTimer);
    if (this.retryTimer) clearTimeout(this.retryTimer);
    this.parent?.close();
    for (const watcher of this.watchers.values()) watcher.close();
    this.watchers.clear();
    this.listeners.clear();
  }
}

/** 同一授权真实根只保留一组文件/Git watcher，最后一个消费者离开即完整释放。 */
export class WorkspaceResourceWatchHub {
  private readonly trees = new Map<string, WorkspaceWatchTree>();
  private readonly starting = new Map<string, Promise<void>>();

  async subscribe(root: string, listener: Listener): Promise<() => void> {
    let tree = this.trees.get(root);
    if (!tree) {
      if (this.trees.size >= 64)
        throw new Error("工作区原生观察根数量达到上限");
      tree = new WorkspaceWatchTree(root);
      this.trees.set(root, tree);
      const starting = tree.start().finally(() => {
        if (this.starting.get(root) === starting) this.starting.delete(root);
      });
      this.starting.set(root, starting);
    }
    tree.listeners.add(listener);
    if (tree.state !== "changed" && !this.starting.has(root)) {
      const starting = tree.retry().finally(() => {
        if (this.starting.get(root) === starting) this.starting.delete(root);
      });
      this.starting.set(root, starting);
    }
    try {
      await this.starting.get(root);
    } catch (error) {
      tree.listeners.delete(listener);
      if (!tree.listeners.size) {
        tree.close();
        if (this.trees.get(root) === tree) this.trees.delete(root);
      }
      throw error;
    }
    const current = tree;
    let active = true;
    return () => {
      if (!active) return;
      active = false;
      current.listeners.delete(listener);
      if (!current.listeners.size) {
        current.close();
        if (this.trees.get(root) === current) this.trees.delete(root);
      }
    };
  }

  close(): void {
    for (const tree of this.trees.values()) tree.close();
    this.trees.clear();
    this.starting.clear();
  }
}

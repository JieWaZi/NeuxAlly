/**
 * Git operations for the sidebar source-control panel. Everything goes
 * through the system `git` binary spawned per request (no library, no state),
 * with porcelain-parseable output formats (`-z` NUL framing, unit separators)
 * so parsing never depends on locale or color config. All commands run with
 * `-C <cwd>` on the session's working directory and `--no-pager` /
 * `-c color.ui=false` so output stays machine-readable.
 *
 * Commits use the user's git global identity untouched (never sets
 * user.name/user.email).
 */
import { createReadStream } from "node:fs";
import { lstat, readdir, readlink } from "node:fs/promises";
import { join } from "node:path";
import { spawn } from "node:child_process";
import { resolve } from "node:path";

/** A parsed `git status --porcelain=v1 -z` entry. */
export interface GitStatusEntry {
  path: string;
  /** Two-letter index/worktree status (X Y), e.g. 'M ', ' M', 'A ', '??'. */
  xy: string;
  /** 暂存区相对 HEAD 增加的行数。 */
  stagedAddedLines: number;
  /** 暂存区相对 HEAD 删除的行数。 */
  stagedDeletedLines: number;
  /** 工作区相对暂存区增加的行数，未跟踪文件按新文件统计。 */
  unstagedAddedLines: number;
  /** 工作区相对暂存区删除的行数。 */
  unstagedDeletedLines: number;
  /** 未跟踪文件过大或无法读取时，行数不可用。 */
  lineCountUnavailable?: boolean;
}

/** The source-control panel snapshot. */
export interface GitStatusResult {
  isRepo: boolean;
  branch?: string;
  entries: GitStatusEntry[];
  /** True when the working tree had more rows than `GIT_STATUS_LIMIT`; the
   *  panel shows a truncation notice instead of freezing on a huge untracked
   *  set (issue #369). */
  truncated?: boolean;
  /** Selected repository root, or the discovered roots when the cwd is a container. */
  root?: string;
  repositories?: string[];
}

/** One linked checkout returned by `git worktree list --porcelain`. */
export interface GitWorktree {
  /** Absolute checkout root. */
  path: string;
  /** Branch name without `refs/heads/`, or `HEAD` when detached. */
  branch: string;
  /** Whether this checkout contains the session cwd. */
  current: boolean;
  /** Number of staged + unstaged status rows (a file changed on both sides counts once). */
  changes: number;
}

/** One `git log` row. */
export interface GitLogEntry {
  /** Short hash (7+ chars, display). */
  hash: string;
  /** Full 40-char hash (advanced operations: revert / cherry-pick). */
  hashFull: string;
  subject: string;
  author: string;
  /** ISO 8601 author date (`%ai`), e.g. `2024-01-01 10:00:00 +0800`. */
  date: string;
  /** Ref decorations (`%D` with --decorate=short), e.g. `HEAD -> main, origin/main`; '' when none. */
  refs: string;
}

/** One git failure (stderr text as the message). */
export class GitCommandError extends Error {
  constructor(
    message: string,
    readonly code = "git-error",
    readonly command: string,
  ) {
    super(message);
  }
}

/** Parse porcelain v1 -z output into entries (rename/copy pairs collapse to one row). */
export function parsePorcelainZ(
  output: string,
): Pick<GitStatusEntry, "path" | "xy">[] {
  const tokens = output.split("\0");
  const entries: Pick<GitStatusEntry, "path" | "xy">[] = [];
  let index = 0;
  while (index < tokens.length) {
    const token = tokens[index];
    index += 1;
    if (token === undefined) break;
    if (token === "") continue;
    const xy = token.slice(0, 2);
    const rest = token.slice(3);
    entries.push({ path: rest, xy });
    // Rename/copy entries carry the ORIGIN path as the next NUL field; the
    // new path (the file as it exists now) is the display path.
    if (
      (xy[0] === "R" || xy[0] === "C") &&
      tokens[index] !== undefined &&
      tokens[index] !== ""
    ) {
      index += 1;
    }
  }
  return entries;
}

/** One raw porcelain worktree record. Prunable checkouts are retained by
 * Git's administrative metadata after their directory disappears and must not
 * become selectable command targets. Locked checkouts remain usable. */
export interface GitWorktreeRecord {
  path: string;
  branch: string;
  locked: boolean;
  prunable: boolean;
}

/** Parse `git worktree list --porcelain` records. Production requests use
 * `-z` so even newlines and non-ASCII bytes in checkout paths stay lossless;
 * newline framing remains accepted for small fixtures and older Git output. */
export function parseWorktreeList(output: string): GitWorktreeRecord[] {
  const rows: GitWorktreeRecord[] = [];
  let path: string | undefined;
  let branch = "HEAD";
  let locked = false;
  let prunable = false;
  const flush = (): void => {
    if (path !== undefined) rows.push({ path, branch, locked, prunable });
    path = undefined;
    branch = "HEAD";
    locked = false;
    prunable = false;
  };
  const sep = output.includes("\0") ? "\0" : "\n";
  const framed = output.endsWith(sep) ? output : `${output}${sep}`;
  for (const line of framed.split(sep)) {
    if (line === "") {
      flush();
    } else if (line.startsWith("worktree ")) {
      path = line.slice("worktree ".length);
    } else if (line.startsWith("branch refs/heads/")) {
      branch = line.slice("branch refs/heads/".length);
    } else if (line === "locked" || line.startsWith("locked ")) {
      locked = true;
    } else if (line === "prunable" || line.startsWith("prunable ")) {
      prunable = true;
    }
  }
  return rows;
}

/** Parse `git log --pretty=format:%h%x1f%s%x1f%an%x1f%ai%x1f%H%x1f%D` rows. */
export function parseLogLines(output: string): GitLogEntry[] {
  const rows: GitLogEntry[] = [];
  for (const line of output.split("\n")) {
    if (line === "") continue;
    const [hash, subject, author, date, hashFull, refs] = line.split("\x1f");
    if (hash === undefined || subject === undefined) continue;
    rows.push({
      hash,
      subject,
      author: author ?? "",
      date: date ?? "",
      hashFull: hashFull ?? hash,
      refs: refs ?? "",
    });
  }
  return rows;
}

/** Run one git command; resolves with stdout, rejects with GitCommandError. */
function runGit(
  cwd: string,
  args: string[],
  timeoutMs = 30_000,
  acceptedExitCodes: readonly number[] = [0],
): Promise<string> {
  const full = ["-C", cwd, "--no-pager", "-c", "color.ui=false", ...args];
  return new Promise<string>((resolvePromise, reject) => {
    const child = spawn("git", full, {
      stdio: ["ignore", "pipe", "pipe"],
      windowsHide: true,
      env: { ...process.env, GIT_OPTIONAL_LOCKS: "0" },
    });
    let stdout = "";
    let stderr = "";
    const stdoutDecoder = new TextDecoder();
    const stderrDecoder = new TextDecoder();
    let stdoutBytes = 0;
    let stderrBytes = 0;
    let settled = false;
    const fail = (error: Error) => {
      if (settled) return;
      settled = true;
      clearTimeout(timer);
      child.kill("SIGKILL");
      reject(error);
    };
    const timer = setTimeout(() => {
      fail(
        new GitCommandError(
          `git ${args[0] ?? ""} timed out after ${timeoutMs}ms`,
          "git-error",
          args.join(" "),
        ),
      );
    }, timeoutMs);
    child.stdout.on("data", (chunk: Buffer) => {
      stdoutBytes += chunk.byteLength;
      if (stdoutBytes > 64 * 1024 * 1024) {
        fail(
          new GitCommandError(
            "Git 输出超过 64 MiB",
            "git-output",
            args.join(" "),
          ),
        );
        return;
      }
      stdout += stdoutDecoder.decode(chunk, { stream: true });
    });
    child.stderr.on("data", (chunk: Buffer) => {
      stderrBytes += chunk.byteLength;
      if (stderrBytes > 1024 * 1024) {
        fail(
          new GitCommandError("Git 错误输出过大", "git-output", args.join(" ")),
        );
        return;
      }
      stderr += stderrDecoder.decode(chunk, { stream: true });
    });
    child.on("error", (error) => {
      fail(
        new GitCommandError(
          `cannot run git: ${error.message}`,
          "git-error",
          args.join(" "),
        ),
      );
    });
    child.on("close", (code) => {
      if (settled) return;
      settled = true;
      clearTimeout(timer);
      stdout += stdoutDecoder.decode();
      stderr += stderrDecoder.decode();
      if (code !== null && acceptedExitCodes.includes(code)) {
        resolvePromise(stdout);
      } else {
        reject(
          new GitCommandError(
            stderr.trim() || `git exited with ${String(code)}`,
            "git-error",
            args.join(" "),
          ),
        );
      }
    });
  });
}

/** Cap on child directories probed by the workspace-container fallback scan.
 *  A home-directory cwd can hold hundreds of visible folders (Library, iCloud
 *  mounts…); probing them all serially is what froze the panel in #369. */
const DISCOVERY_LIMIT = 200;
/** Per-probe and direct-discovery budget. `rev-parse` is millisecond-scale on
 *  a healthy checkout; a probe that needs longer is a stalled mount and is
 *  better abandoned than waited on. */
const DISCOVERY_TIMEOUT_MS = 5_000;
/** Discovery results are cheap to recompute but expensive to storm: the panel
 *  polls every 2s and each poll fans out into several git.* calls that all
 *  resolve the same roots. A short TTL keeps fan-out at one scan per cwd. */
const DISCOVERY_CACHE_TTL_MS = 60_000;

const repoRootsCache = new Map<string, { roots: string[]; expires: number }>();
const repoRootsInFlight = new Map<string, Promise<string[]>>();

/** Whether the directory is inside a git work tree (exit-0 `git rev-parse`).
 *  Probe timeout is short: a cwd on a stalled mount must not hold the panel
 *  hostage for the full command budget (issue #369). */
export async function isGitRepo(cwd: string): Promise<boolean> {
  try {
    const out = await runGit(
      cwd,
      ["rev-parse", "--is-inside-work-tree"],
      DISCOVERY_TIMEOUT_MS,
    );
    return out.trim() === "true";
  } catch {
    return false;
  }
}

/** The repository top level containing `cwd` (`git rev-parse --show-toplevel`). */
async function directRepoRoot(cwd: string): Promise<string> {
  const out = await runGit(
    cwd,
    ["rev-parse", "--show-toplevel"],
    DISCOVERY_TIMEOUT_MS,
  );
  return out.trim();
}

/** Discover the current repository or direct child repositories. Results are
 *  cached per cwd and concurrent callers share one in-flight scan, so opening
 *  the panel (three parallel git.* requests) costs a single discovery pass. */
export function repoRoots(cwd: string): Promise<string[]> {
  const cached = repoRootsCache.get(cwd);
  if (cached !== undefined && cached.expires > Date.now())
    return Promise.resolve(cached.roots);
  const pending = repoRootsInFlight.get(cwd);
  if (pending !== undefined) return pending;
  const promise = discoverRepoRoots(cwd).then(
    (roots) => {
      repoRootsCache.set(cwd, {
        roots,
        expires: Date.now() + DISCOVERY_CACHE_TTL_MS,
      });
      repoRootsInFlight.delete(cwd);
      return roots;
    },
    (error: unknown) => {
      repoRootsInFlight.delete(cwd);
      throw error;
    },
  );
  repoRootsInFlight.set(cwd, promise);
  return promise;
}

async function discoverRepoRoots(cwd: string): Promise<string[]> {
  try {
    return [await directRepoRoot(cwd)];
  } catch {
    const entries = await readdir(cwd, { withFileTypes: true }).catch(() => []);
    const roots: string[] = [];
    for (const entry of entries
      .filter(
        (entry) =>
          entry.isDirectory() &&
          !entry.name.startsWith(".") &&
          entry.name !== "node_modules",
      )
      .sort((left, right) => left.name.localeCompare(right.name))
      .slice(0, DISCOVERY_LIMIT)) {
      try {
        const root = await directRepoRoot(join(cwd, entry.name));
        if (
          !roots.some(
            (existing) => pathIdentity(existing) === pathIdentity(root),
          )
        )
          roots.push(root);
      } catch {
        // Ordinary child directory; keep discovering sibling repositories.
      }
    }
    return roots;
  }
}

/** Resolve the selected repository, defaulting to the first discovered root. */
export async function repoRoot(
  cwd: string,
  selected?: string,
): Promise<string> {
  const roots = await repoRoots(cwd);
  const first = roots[0];
  if (first === undefined)
    throw new GitCommandError("not a git repository", "not-repo", "rev-parse");
  // Git for Windows may return forward-slash roots while callers pass
  // backslashes (or vice-versa); compare via the platform-aware identity.
  if (selected !== undefined) {
    const identity = pathIdentity(selected);
    const match = roots.find((root) => pathIdentity(root) === identity);
    if (match !== undefined) return match;
  }
  return first;
}

/** The current branch name (`git rev-parse --abbrev-ref HEAD`; 'HEAD' when detached). */
export async function currentBranch(cwd: string): Promise<string> {
  const out = await runGit(cwd, ["rev-parse", "--abbrev-ref", "HEAD"]);
  return out.trim();
}

/** Upper bound on status rows shipped to the client. Beyond this the result
 *  is truncated (with `truncated: true`) so a pathological untracked set —
 *  e.g. the working tree discovered under a home-directory cwd — cannot
 *  freeze the browser main thread on JSON parse or list render (#369). */
const GIT_STATUS_LIMIT = 2_000;

/** 解析 Git 的 NUL 分隔行数输出，重命名记录以新路径为键。 */
function parseNumstatZ(output: string): Map<string, [number, number]> {
  const counts = new Map<string, [number, number]>();
  const tokens = output.split("\0");
  for (let index = 0; index < tokens.length; index += 1) {
    const match = /^(\d+|-)\t(\d+|-)\t([\s\S]*)$/.exec(tokens[index] ?? "");
    if (!match) continue;
    const path = match[3] || tokens[index + 2];
    if (!path) continue;
    if (!match[3]) index += 2;
    counts.set(path, [Number(match[1]) || 0, Number(match[2]) || 0]);
  }
  return counts;
}

/** 限制每个未跟踪文件的读取量，避免状态轮询扫描大型二进制文件。 */
const UNTRACKED_LINE_COUNT_MAX_BYTES = 1024 * 1024;
const UNTRACKED_LINE_COUNT_TOTAL_BYTES = 64 * 1024 * 1024;

/** 统计小型未跟踪文件的文本行；超限时返回 null。 */
async function untrackedLineCount(
  root: string,
  path: string,
  reserveBytes: (size: number) => boolean,
): Promise<number | null> {
  const absolute = join(root, path);
  const file = await lstat(absolute);
  if (file.isSymbolicLink()) {
    const target = await readlink(absolute);
    return target === "" ? 0 : target.split("\n").length;
  }
  if (!file.isFile()) return 0;
  if (file.size > UNTRACKED_LINE_COUNT_MAX_BYTES || !reserveBytes(file.size))
    return null;
  if (file.size === 0) return 0;
  let breaks = 0;
  let lastByte: number | undefined;
  for await (const chunk of createReadStream(absolute, {
    start: 0,
    end: file.size - 1,
  })) {
    const bytes = chunk as Buffer;
    if (bytes.includes(0)) return 0;
    for (const byte of bytes) if (byte === 10) breaks += 1;
    lastByte = bytes.at(-1);
  }
  return lastByte === undefined ? 0 : breaks + (lastByte === 10 ? 0 : 1);
}

/** 限制同时读取的未跟踪文件数，避免大仓库耗尽文件描述符。 */
async function untrackedLineCounts(
  root: string,
  paths: readonly string[],
): Promise<Map<string, number | null>> {
  const counts = new Map<string, number | null>();
  let remainingBytes = UNTRACKED_LINE_COUNT_TOTAL_BYTES;
  const reserveBytes = (size: number): boolean => {
    if (size > remainingBytes) return false;
    remainingBytes -= size;
    return true;
  };
  let next = 0;
  await Promise.all(
    Array.from({ length: Math.min(8, paths.length) }, async () => {
      while (next < paths.length) {
        const path = paths[next++];
        if (!path) continue;
        counts.set(
          path,
          await untrackedLineCount(root, path, reserveBytes).catch(() => null),
        );
      }
    }),
  );
  return counts;
}

/**
 * Working-tree status (untracked included). `--untracked-files=all` lists
 * the contents of new directories as individual entries, while preserving
 * repository discovery and explicit repository selection for workspace roots.
 */
export async function status(
  cwd: string,
  selected?: string,
): Promise<GitStatusResult> {
  const repositories = await repoRoots(cwd);
  if (repositories.length === 0)
    return { isRepo: false, entries: [], repositories: [] };
  const root = await repoRoot(cwd, selected);
  const [branch, raw] = await Promise.all([
    currentBranch(root).catch(() => "HEAD"),
    runGit(root, ["status", "--porcelain=v1", "-z", "--untracked-files=all"]),
  ]);
  const parsed = parsePorcelainZ(raw);
  const truncated = parsed.length > GIT_STATUS_LIMIT;
  const visible = truncated ? parsed.slice(0, GIT_STATUS_LIMIT) : parsed;
  const [staged, unstaged, untracked] = await Promise.all([
    visible.some((entry) => entry.xy[0] !== " " && entry.xy[0] !== "?")
      ? runGit(root, ["diff", "--cached", "--numstat", "-z"]).then(
          parseNumstatZ,
        )
      : new Map<string, [number, number]>(),
    visible.some((entry) => entry.xy[1] !== " " && entry.xy[1] !== "?")
      ? runGit(root, ["diff", "--numstat", "-z"]).then(parseNumstatZ)
      : new Map<string, [number, number]>(),
    untrackedLineCounts(
      root,
      visible.filter((entry) => entry.xy === "??").map((entry) => entry.path),
    ),
  ]);
  return {
    isRepo: true,
    branch,
    entries: visible.map((entry): GitStatusEntry => {
      const stagedLines = staged.get(entry.path) ?? [0, 0];
      const untrackedCount =
        entry.xy === "??" ? untracked.get(entry.path) : undefined;
      const unstagedLines =
        entry.xy === "??"
          ? [untrackedCount ?? 0, 0]
          : (unstaged.get(entry.path) ?? [0, 0]);
      return {
        ...entry,
        stagedAddedLines: stagedLines[0] ?? 0,
        stagedDeletedLines: stagedLines[1] ?? 0,
        unstagedAddedLines: unstagedLines[0] ?? 0,
        unstagedDeletedLines: unstagedLines[1] ?? 0,
        ...(entry.xy === "??" && untrackedCount == null
          ? { lineCountUnavailable: true }
          : {}),
      };
    }),
    truncated,
    root,
    repositories,
  };
}

/** Platform-aware identity used only for comparing absolute checkout roots. */
function pathIdentity(path: string): string {
  const absolute = resolve(path).replace(/[\\/]+$/, "");
  return process.platform === "win32" ? absolute.toLowerCase() : absolute;
}

/** Whether the current Git binary supports NUL-framed `worktree list` output.
 * Git < 2.36 rejects `-z`; cache the capability after the first attempt so
 * the SCM panel's polling does not repeatedly spawn a command known to fail. */
let worktreeListSupportsZ: boolean | undefined;

/** Raw usable checkout records, shared by inventory and target validation.
 * Prunable records point at missing paths and are deliberately excluded from
 * both the selector and the command-target allowlist. */
async function listedWorktrees(cwd: string): Promise<GitWorktreeRecord[]> {
  let raw: string;
  if (worktreeListSupportsZ === false) {
    raw = await runGit(cwd, ["worktree", "list", "--porcelain"]);
  } else {
    try {
      raw = await runGit(cwd, ["worktree", "list", "--porcelain", "-z"]);
      worktreeListSupportsZ = true;
    } catch {
      worktreeListSupportsZ = false;
      raw = await runGit(cwd, ["worktree", "list", "--porcelain"]);
    }
  }
  return parseWorktreeList(raw).filter((entry) => !entry.prunable);
}

/** All linked checkouts of the repository containing `cwd`, enriched with a
 * live change count. The current checkout is first so a single-worktree repo
 * preserves the old UI ordering. */
export async function worktrees(cwd: string): Promise<GitWorktree[]> {
  if (!(await isGitRepo(cwd))) return [];
  const currentRoot = await repoRoot(cwd);
  const listed = await listedWorktrees(cwd);
  const rows = await Promise.all(
    listed.map(async (entry): Promise<GitWorktree> => ({
      path: entry.path,
      branch: entry.branch,
      current: pathIdentity(entry.path) === pathIdentity(currentRoot),
      // One stale/permission-raced linked checkout must not hide the valid
      // current repository from the panel. Targeted operations still fail loud.
      changes: await status(entry.path).then(
        (result) => result.entries.length,
        () => 0,
      ),
    })),
  );
  return rows.sort(
    (left, right) => Number(right.current) - Number(left.current),
  );
}

/** Resolve an optional client-selected linked checkout. A caller may never use
 * this seam to point Git operations at an unrelated repository: the target
 * must occur in the authoritative session repository's worktree list. */
export async function resolveWorktree(
  cwd: string,
  requested?: string,
): Promise<string> {
  if (requested === undefined || requested === "") return cwd;
  const identity = pathIdentity(requested);
  const match = (await listedWorktrees(cwd)).find(
    (entry) => pathIdentity(entry.path) === identity,
  );
  if (match === undefined) {
    throw new GitCommandError(
      `unknown linked worktree: ${requested}`,
      "git-worktree",
      "worktree list",
    );
  }
  return match.path;
}

/** Diff text of the worktree (unstaged) or the index (staged). */
export async function diff(
  cwd: string,
  path: string | undefined,
  staged: boolean,
  selected?: string,
): Promise<string> {
  const root = await repoRoot(cwd, selected);
  const args = ["diff", "--no-ext-diff", "--no-color", "-U3"];
  if (staged) args.push("--cached");
  if (path !== undefined) args.push("--", `:(literal)${path}`);
  const patch = await runGit(root, args);
  if (patch || staged || path === undefined) return patch;
  const untracked = await runGit(root, [
    "ls-files",
    "--others",
    "--exclude-standard",
    "-z",
    "--",
    `:(literal)${path}`,
  ]);
  if (!untracked.split("\0").includes(path)) return patch;
  return runGit(
    root,
    [
      "diff",
      "--no-index",
      "--no-ext-diff",
      "--no-color",
      "-U3",
      "--",
      process.platform === "win32" ? "NUL" : "/dev/null",
      path,
    ],
    30_000,
    [0, 1],
  );
}

/** Stage paths (all when path is undefined). */
export async function stage(
  cwd: string,
  path: string | undefined,
  selected?: string,
): Promise<void> {
  await runGit(await repoRoot(cwd, selected), [
    "add",
    "-A",
    ...(path !== undefined ? ["--", `:(literal)${path}`] : []),
  ]);
}

/** Unstage paths (all when path is undefined). */
export async function unstage(
  cwd: string,
  path: string | undefined,
  selected?: string,
): Promise<void> {
  await runGit(await repoRoot(cwd, selected), [
    "reset",
    "-q",
    ...(path !== undefined ? ["--", `:(literal)${path}`] : []),
  ]);
}

/** Commit the staged changes with a message (global identity untouched). */
export async function commit(
  cwd: string,
  message: string,
  selected?: string,
): Promise<void> {
  await runGit(await repoRoot(cwd, selected), ["commit", "-m", message]);
}

/** Branch names (current first). */
export async function branches(
  cwd: string,
  selected?: string,
): Promise<{ current: string; names: string[] }> {
  const root = await repoRoot(cwd, selected);
  const [current, raw] = await Promise.all([
    currentBranch(root).catch(() => "HEAD"),
    runGit(root, ["for-each-ref", "--format=%(refname:short)", "refs/heads"]),
  ]);
  const names = raw.split("\n").filter((line) => line !== "");
  return {
    current,
    names: names.includes(current) ? names : [current, ...names],
  };
}

/** Switch to an existing branch. */
export async function checkout(
  cwd: string,
  branch: string,
  selected?: string,
): Promise<void> {
  await runGit(await repoRoot(cwd, selected), ["checkout", branch]);
}

/** Recent commit history (newest first), lazily pageable via skip/count. */
export async function log(
  cwd: string,
  count = 30,
  skip = 0,
  selected?: string,
): Promise<GitLogEntry[]> {
  const raw = await runGit(await repoRoot(cwd, selected), [
    "log",
    "-n",
    String(count),
    "--skip",
    String(skip),
    "--decorate=short",
    "--pretty=format:%h%x1f%s%x1f%an%x1f%ai%x1f%H%x1f%D",
  ]);
  return parseLogLines(raw);
}

/**
 * Content of a file at a revision (`git show <rev>:<path>`), or null when the
 * revision has no such path (a new/untracked file has no HEAD side).
 */
export async function show(
  cwd: string,
  rev: string,
  path: string,
  selected?: string,
): Promise<string | null> {
  try {
    return await runGit(await repoRoot(cwd, selected), [
      "show",
      `${rev}:${path}`,
    ]);
  } catch {
    return null;
  }
}

/** Full patch text of one commit (`git show` with the commit header suppressed).
 *  Merge commits show their diff against the first parent (`-m --first-parent`
 *  is a no-op for regular commits), so a history click always has content. */
export async function commitDiff(
  cwd: string,
  hash: string,
  selected?: string,
): Promise<string> {
  return runGit(await repoRoot(cwd, selected), [
    "show",
    "--no-ext-diff",
    "--no-color",
    "--format=",
    "-m",
    "--first-parent",
    hash,
  ]);
}

/** Discard the worktree changes of one path (`git checkout -- <path>`; the index is untouched). */
export async function discard(
  cwd: string,
  path: string,
  selected?: string,
): Promise<void> {
  await runGit(await repoRoot(cwd, selected), [
    "checkout",
    "--",
    `:(literal)${path}`,
  ]);
}

/** Revert one commit onto the current branch with an auto-generated message. */
export async function revert(
  cwd: string,
  hash: string,
  selected?: string,
): Promise<void> {
  await runGit(await repoRoot(cwd, selected), ["revert", "--no-edit", hash]);
}

/** Cherry-pick one commit onto the current branch. */
export async function cherryPick(
  cwd: string,
  hash: string,
  selected?: string,
): Promise<void> {
  await runGit(await repoRoot(cwd, selected), ["cherry-pick", hash]);
}

import type {
  GlobalEventSubscription,
  GlobalEventDelivery,
} from "./core-event-protocol";
import type { ApplicationUpdateState } from "./application-update";
export type CoreConnection = {
  baseUrl: string;
  token: string;
};

export type FileDropEvent =
  | {
      type: "enter" | "over" | "drop";
      paths: string[];
      position: { x: number; y: number };
    }
  | { type: "leave" };

export type WorkspaceFileEntry = {
  name: string;
  path: string;
  relativePath: string;
};

export type ChatWorkspaceFiles = {
  generated: WorkspaceFileEntry[];
  removedPaths: string[];
};

export type WorkspaceDirectoryEntry = {
  name: string;
  relativePath: string;
  isDirectory: boolean;
  size?: number;
};

export type WorkspaceFileContent = {
  name: string;
  relativePath: string;
  kind: "text" | "image" | "binary" | "too_large";
  mediaType?: string;
  content?: string;
  dataUrl?: string;
  size: number;
  version?: string;
};

/** Main 根据 Core 的 Chat 身份建立的受限文件范围。 */
export type WorkspaceResourceScope = { id: string; root: string };

/** 文件元数据和乐观版本；版本只用于普通外部修改检测。 */
export type WorkspaceResourceStat = {
  relativePath: string;
  size: number;
  version: string;
  modifiedAt: number;
  mediaType: string;
};

/** 有大小上限的完整读取结果，传输时保留 Uint8Array。 */
export type WorkspaceResourceRead = WorkspaceResourceStat & {
  bytes: Uint8Array;
};

/** 保存返回写入版本及对应的编辑器修订号。 */
export type WorkspaceResourceWrite = {
  resource: WorkspaceResourceStat;
  saveRevision: number;
};

/** 文件选择器的逐项导入结果，不通过预览缓存搬运大文件。 */
export type WorkspaceResourceImport = {
  imported: string[];
  failed: { name: string; message: string }[];
};

/** 先注册事件监听，再启动订阅；序号用于处理 ready 与变化竞态。 */
export type WorkspaceResourceChangeEvent = {
  subscriptionId: string;
  sequence: number;
  /** 监听有效性与文件存在性分开；恢复或失败不能伪装为删除。 */
  state?: "ready" | "changed" | "recovering" | "failed";
  resource: WorkspaceResourceStat | null;
};

export type WorkspaceGitChange = {
  path: string;
  name: string;
  status: "modified" | "added" | "deleted" | "untracked";
  addedLines: number;
  deletedLines: number;
  gitRoot: string;
};

export type WorkspaceGitChanges = {
  isGitRepo: boolean;
  /** 当前 HEAD 所在分支；detached HEAD 时为短 commit hash。 */
  branch?: string;
  files: WorkspaceGitChange[];
  totalAddedLines: number;
  totalDeletedLines: number;
};

/** Git porcelain 快照，保留暂存区与工作区各自的状态字母。 */
export type ScopedGitStatus = {
  isRepo: boolean;
  branch?: string;
  entries: {
    path: string;
    xy: string;
    stagedAddedLines: number;
    stagedDeletedLines: number;
    unstagedAddedLines: number;
    unstagedDeletedLines: number;
    lineCountUnavailable?: boolean;
  }[];
  truncated?: boolean;
  root?: string;
  repositories?: string[];
};

export type ScopedGitLogEntry = {
  hash: string;
  hashFull: string;
  subject: string;
  author: string;
  date: string;
  refs: string;
};

export type WorkspaceGitBranch = {
  /** 分支短名，例如 main 或 origin/main。 */
  name: string;
  /** 是否为当前检出的分支。 */
  isCurrent: boolean;
  /** 是否为远程跟踪分支。 */
  isRemote: boolean;
};

export type WorkspaceGitBranches = {
  isGitRepo: boolean;
  /** Git 仓库根目录。 */
  gitRoot?: string;
  branches: WorkspaceGitBranch[];
};

export type WorkspaceGitCheckoutConflict = {
  path: string;
  addedLines: number;
  deletedLines: number;
};

export type WorkspaceGitCheckoutPreview = {
  /** 是否可以直接切换，无需先提交。 */
  canCheckout: boolean;
  conflicts: WorkspaceGitCheckoutConflict[];
};

export type WorkspaceGitCheckoutResult = {
  /** 切换后的当前分支名。 */
  branch: string;
};

export type WorkspaceGitRepository = {
  /** 用于展示与 git -C 的目录路径。 */
  directory: string;
  /** Git 仓库根目录。 */
  gitRoot: string;
  /** 目录展示名，通常为路径最后一段。 */
  label: string;
  /** 当前 HEAD 所在分支；detached HEAD 时为短 commit hash。 */
  branch?: string;
};

export type WorkspaceGitRepositories = {
  repositories: WorkspaceGitRepository[];
};

export type WorkspaceDiffContent = {
  name: string;
  relativePath: string;
  oldContent: string;
  newContent: string;
};

export type FileReferenceRequest = {
  path: string;
  baseDirectory: string;
  workspaceRoot: string;
  sourceRoots: readonly string[];
};

export type FileReferenceResolution =
  | {
      status: "resolved" | "directory";
      canonicalPath: string;
      baseDirectory: string;
      workspaceRoot: string;
      sourceRoots: string[];
    }
  | { status: "missing" | "outside_scope" | "ambiguous" | "unavailable" };

export type BrowserBounds = {
  x: number;
  y: number;
  width: number;
  height: number;
};

export type BrowserNavigationEvent = {
  viewId: string;
  url: string;
  title: string;
  canGoBack: boolean;
  canGoForward: boolean;
};

export type BrowserControlAction = "back" | "forward" | "reload";

export type TerminalStarted = {
  sessionId: string;
  shell: string;
};

export type TerminalSnapshot = TerminalStarted & {
  attachmentId: string;
  screen: string;
  sequence: number;
  state: "running" | "exited";
  exitCode?: number;
};

export type TerminalOutputEvent = {
  sessionId: string;
  sequence: number;
  data: string;
};

export type TerminalExitEvent = {
  sessionId: string;
  code?: number;
};

export type DesktopCommands = {
  core_connection: { args: undefined; result: CoreConnection };
  application_update_state: { args: undefined; result: ApplicationUpdateState };
  check_application_update: { args: undefined; result: ApplicationUpdateState };
  download_application_update: {
    args: undefined;
    result: ApplicationUpdateState;
  };
  open_application_installer: {
    args: undefined;
    result: ApplicationUpdateState;
  };
  subscribe_core_events: { args: GlobalEventSubscription; result: boolean };
  resync_core_events: {
    args: { subscriptionId: string; after?: number | string };
    result: boolean;
  };
  unsubscribe_core_events: {
    args: { subscriptionId: string };
    result: boolean;
  };
  save_file: { args: { bytes: number[]; fileName: string }; result: boolean };
  select_directory: { args: undefined; result: string | null };
  select_files: { args: undefined; result: string[] };
  stage_attachment: {
    args: { name: string; bytes: Uint8Array };
    result: string;
  };
  search_workspace_files: {
    args: { root: string; query: string; limit: number; scopePath?: string };
    result: WorkspaceFileEntry[];
  };
  list_workspace_directory: {
    args: { root: string; relativePath: string };
    result: WorkspaceDirectoryEntry[];
  };
  resolve_file_reference: {
    args: { request: FileReferenceRequest };
    result: FileReferenceResolution;
  };
  read_workspace_file: {
    args: { root: string; relativePath: string };
    result: WorkspaceFileContent;
  };
  open_workspace_scope: {
    args: { chatId: string };
    result: WorkspaceResourceScope;
  };
  list_workspace_resource_directory: {
    args: { scopeId: string; relativePath: string; root?: string };
    result: WorkspaceDirectoryEntry[];
  };
  search_workspace_resource_files: {
    args: {
      scopeId: string;
      query: string;
      limit: number;
      scopePath: string;
      root?: string;
    };
    result: WorkspaceFileEntry[];
  };
  save_workspace_resource_as: {
    args: { scopeId: string; relativePath: string; root?: string };
    result: boolean;
  };
  rename_workspace_resource: {
    args: {
      scopeId: string;
      relativePath: string;
      nextName: string;
      expectedVersion: string;
      root?: string;
    };
    result: WorkspaceResourceStat;
  };
  delete_workspace_resource: {
    args: { scopeId: string; relativePath: string; root?: string };
    result: boolean;
  };
  rename_workspace_directory: {
    args: {
      scopeId: string;
      relativePath: string;
      nextName: string;
      root?: string;
    };
    result: string;
  };
  delete_workspace_directory: {
    args: { scopeId: string; relativePath: string; root?: string };
    result: boolean;
  };
  import_workspace_resources: {
    args: { scopeId: string; directoryPath: string; root?: string };
    result: WorkspaceResourceImport;
  };
  import_workspace_resource_bytes: {
    args: {
      scopeId: string;
      directoryPath: string;
      root?: string;
      name: string;
      bytes: Uint8Array;
    };
    result: string;
  };
  open_workspace_resource_external: {
    args: { scopeId: string; relativePath: string; root?: string };
    result: boolean;
  };
  resolve_workspace_resource_reference: {
    args: { scopeId: string; path: string; baseDirectory?: string };
    result: FileReferenceResolution;
  };
  stat_workspace_resource: {
    args: { scopeId: string; relativePath: string; root?: string };
    result: WorkspaceResourceStat | null;
  };
  read_workspace_resource: {
    args: {
      scopeId: string;
      relativePath: string;
      requestId: string;
      root?: string;
    };
    result: WorkspaceResourceRead;
  };
  write_workspace_resource: {
    args: {
      scopeId: string;
      root?: string;
      relativePath: string;
      content: string;
      expectedVersion: string;
      saveRevision: number;
    };
    result: WorkspaceResourceWrite;
  };
  cancel_workspace_resource_read: {
    args: { scopeId: string; requestId: string };
    result: boolean;
  };
  watch_workspace_resource: {
    args: {
      scopeId: string;
      relativePath: string;
      root?: string;
      directory?: boolean;
    };
    result: WorkspaceResourceChangeEvent;
  };
  unwatch_workspace_resource: {
    args: { scopeId: string; subscriptionId: string };
    result: boolean;
  };
  write_workspace_file: {
    args: { root: string; relativePath: string; content: string };
    result: boolean;
  };
  list_chat_workspace_files: {
    args: { chatId: string; root: string };
    result: ChatWorkspaceFiles;
  };
  download_chat_workspace_file: {
    args: { chatId: string; root: string; relativePath: string };
    result: boolean;
  };
  delete_chat_workspace_file: {
    args: { chatId: string; root: string; relativePath: string };
    result: boolean;
  };
  get_workspace_git_changes: {
    args: { root: string };
    result: WorkspaceGitChanges;
  };
  list_workspace_git_branches: {
    args: { root: string };
    result: WorkspaceGitBranches;
  };
  preview_workspace_git_checkout: {
    args: { root: string; branch: string };
    result: WorkspaceGitCheckoutPreview;
  };
  checkout_workspace_git_branch: {
    args: { root: string; branch: string; commitMessage?: string };
    result: WorkspaceGitCheckoutResult;
  };
  list_workspace_git_repositories: {
    args: { roots: string[] };
    result: WorkspaceGitRepositories;
  };
  read_workspace_diff: {
    args: { root: string; relativePath: string };
    result: WorkspaceDiffContent;
  };
  scoped_git_status: {
    args: { chatId: string; root: string; repository?: string };
    result: ScopedGitStatus;
  };
  scoped_git_diff: {
    args: {
      chatId: string;
      root: string;
      repository: string;
      relativePath: string;
      staged: boolean;
    };
    result: string;
  };
  scoped_git_log: {
    args: {
      chatId: string;
      root: string;
      repository: string;
      count: number;
      skip: number;
    };
    result: ScopedGitLogEntry[];
  };
  scoped_git_show: {
    args: {
      chatId: string;
      root: string;
      repository: string;
      revision: string;
      relativePath: string;
    };
    result: string | null;
  };
  scoped_git_commit_diff: {
    args: { chatId: string; root: string; repository: string; hash: string };
    result: string;
  };
  scoped_git_stage: {
    args: {
      chatId: string;
      root: string;
      repository: string;
      relativePath: string;
    };
    result: void;
  };
  scoped_git_unstage: {
    args: {
      chatId: string;
      root: string;
      repository: string;
      relativePath: string;
    };
    result: void;
  };
  scoped_git_commit: {
    args: { chatId: string; root: string; repository: string; message: string };
    result: void;
  };
  scoped_git_discard: {
    args: {
      chatId: string;
      root: string;
      repository: string;
      relativePath: string;
    };
    result: boolean;
  };
  open_workspace_file: {
    args: { root: string; relativePath: string };
    result: boolean;
  };
  browser_create: {
    args: BrowserBounds & {
      url: string;
      localRoot?: string;
      chatId?: string;
    };
    result: string;
  };
  browser_prepare_webview: {
    args: { url: string; localRoot?: string; chatId?: string };
    result: string;
  };
  browser_navigate: { args: { viewId: string; url: string }; result: void };
  browser_control: {
    args: { viewId: string; action: BrowserControlAction };
    result: void;
  };
  browser_set_bounds: {
    args: BrowserBounds & { viewId: string };
    result: void;
  };
  browser_set_visible: {
    args: { viewId: string; visible: boolean };
    result: void;
  };
  browser_close: { args: { viewId: string }; result: boolean };
  open_external_url: { args: { url: string }; result: void };
  terminal_start: {
    args: {
      root: string;
      chatId: string;
      ownerKey: string;
      createWhenMissing: boolean;
    };
    result: TerminalStarted;
  };
  terminal_attach: {
    args: { sessionId: string };
    result: TerminalSnapshot;
  };
  terminal_detach: {
    args: { sessionId: string; attachmentId: string };
    result: void;
  };
  terminal_write: {
    args: { sessionId: string; attachmentId: string; data: string };
    result: void;
  };
  terminal_resize: {
    args: {
      sessionId: string;
      attachmentId: string;
      size: { rows: number; cols: number };
    };
    result: void;
  };
  terminal_close: { args: { ownerKey: string }; result: boolean };
  show_notification: {
    args: { id: string; title: string; body: string };
    result: boolean;
  };
};
export type DesktopEvents = {
  "application-update": ApplicationUpdateState;
  "core-event": GlobalEventDelivery;
  "browser-navigation": BrowserNavigationEvent;
  "workspace-resource-change": WorkspaceResourceChangeEvent;
  "terminal-output": TerminalOutputEvent;
  "terminal-exit": TerminalExitEvent;
  "notification-action": { id: string; action: "click" | "close" };
};
export type DesktopBridge = {
  [K in keyof DesktopCommands]: (
    ...args: DesktopCommands[K]["args"] extends undefined
      ? [args?: undefined]
      : [args: DesktopCommands[K]["args"]]
  ) => Promise<DesktopCommands[K]["result"]>;
} & {
  onApplicationUpdate(
    handler: (state: ApplicationUpdateState) => void,
  ): () => void;
  onCoreEvent(handler: (event: GlobalEventDelivery) => void): () => void;
  onBrowserNavigation(
    handler: (event: BrowserNavigationEvent) => void,
  ): () => void;
  onWorkspaceResourceChange(
    handler: (event: WorkspaceResourceChangeEvent) => void,
  ): () => void;
  onTerminalOutput(handler: (event: TerminalOutputEvent) => void): () => void;
  onTerminalExit(handler: (event: TerminalExitEvent) => void): () => void;
  onNotificationAction(
    handler: (event: DesktopEvents["notification-action"]) => void,
  ): () => void;
  onFileDrop(handler: (event: FileDropEvent) => void): () => void;
};

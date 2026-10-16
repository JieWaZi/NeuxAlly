import {
  dialog,
  ipcMain,
  Notification,
  shell,
  type BrowserWindow,
} from "electron";
import { copyFile, writeFile as save } from "node:fs/promises";
import path from "node:path";
import type { DesktopCommands, DesktopEvents } from "../shared/contracts";
import { commandSchemas } from "../shared/validation";
import type { CoreSupervisor } from "./core-supervisor";
import { BrowserManager } from "./browser-manager";
import { BrowserWebviewPolicy } from "./browser-webview-policy";
import { TerminalManager } from "./terminal-manager";
import { WorkspaceResourceScopes } from "./workspace-resource-scope";
import { ScopedGit } from "./scoped-git";
import * as files from "./workspace-files";
import { createAttachmentStaging } from "./attachment-staging";
import type { CoreEventClient } from "../shared/core-event-client";
import type { ApplicationUpdate } from "./application-update";

export function installDesktopServices(
  window: BrowserWindow,
  core: CoreSupervisor,
  trustedURL: string,
  events?: CoreEventClient,
  updates?: ApplicationUpdate,
) {
  // BrowserWindow 的 closed 事件触发时再读取 window.webContents 会抛出
  // “Object has been destroyed”，因此安装阶段固定持有本次窗口的内容对象。
  const contents = window.webContents;
  const emit = <K extends keyof DesktopEvents>(
    name: K,
    payload: DesktopEvents[K],
  ) => {
    if (!window.isDestroyed() && !contents.isDestroyed())
      contents.send(`ally:${name}`, payload);
  };
  let browser = new BrowserManager(window, (event) =>
    emit("browser-navigation", event),
  );
  const browserWebviews = new BrowserWebviewPolicy(window);
  const terminals = new TerminalManager(emit);
  let resources = new WorkspaceResourceScopes(core, (event) =>
    emit("workspace-resource-change", event),
  );
  let scopedGit = new ScopedGit(resources);
  const notifications = new Map<string, Notification>();
  let coreEpoch: string | undefined;
  const eventSubscriptions = new Map<
    string,
    { stop: () => void; resync: (after?: number | string) => void }
  >();
  let attachments = createAttachmentStaging();
  const handlers: {
    [K in keyof DesktopCommands]: (
      args: DesktopCommands[K]["args"],
    ) => DesktopCommands[K]["result"] | Promise<DesktopCommands[K]["result"]>;
  } = {
    application_update_state: () => {
      if (!updates) throw new Error("软件更新服务尚未装配");
      return updates.snapshot();
    },
    check_application_update: () => {
      if (!updates) throw new Error("软件更新服务尚未装配");
      return updates.check();
    },
    download_application_update: () => {
      if (!updates) throw new Error("软件更新服务尚未装配");
      return updates.download();
    },
    open_application_installer: () => {
      if (!updates) throw new Error("软件更新服务尚未装配");
      return updates.openInstaller();
    },
    core_connection: () => core.connection(),
    subscribe_core_events: ({ subscriptionId, topic, resource, after }) => {
      if (!events) throw new Error("全局事件服务尚未装配");
      if (eventSubscriptions.has(subscriptionId)) return true;
      if (eventSubscriptions.size >= 128) throw new Error("事件订阅超过上限");
      const stop = events.subscribe(topic, resource, {
        after,
        onEvent: (frame) => {
          if (coreEpoch !== undefined && coreEpoch !== frame.epoch) {
            // 同地址/令牌也可能是新 Core；旧授权和监听必须重新核验。
            resources.dispose();
            resources = new WorkspaceResourceScopes(core, (event) =>
              emit("workspace-resource-change", event),
            );
            scopedGit = new ScopedGit(resources);
          }
          coreEpoch = frame.epoch;
          emit("core-event", { subscriptionId, kind: "event", frame });
        },
        onState: (state, connectionChanged) =>
          emit("core-event", {
            subscriptionId,
            kind: "state",
            state,
            connectionChanged,
          }),
        onResync: (reason) =>
          emit("core-event", { subscriptionId, kind: "resync", reason }),
      });
      eventSubscriptions.set(subscriptionId, {
        stop,
        resync: (after) => events.requestResync(topic, resource, after),
      });
      return true;
    },
    resync_core_events: ({ subscriptionId, after }) => {
      const subscription = eventSubscriptions.get(subscriptionId);
      subscription?.resync(after);
      return Boolean(subscription);
    },
    unsubscribe_core_events: ({ subscriptionId }) => {
      eventSubscriptions.get(subscriptionId)?.stop();
      return eventSubscriptions.delete(subscriptionId);
    },
    save_file: async ({ bytes, fileName }) => {
      const result = await dialog.showSaveDialog(window, {
        defaultPath: path.basename(fileName),
      });
      if (result.canceled || !result.filePath) return false;
      await save(result.filePath, Uint8Array.from(bytes));
      return true;
    },
    select_directory: async () => {
      const result = await dialog.showOpenDialog(window, {
        properties: ["openDirectory"],
      });
      return result.canceled ? null : (result.filePaths[0] ?? null);
    },
    select_files: async () => {
      const result = await dialog.showOpenDialog(window, {
        properties: ["openFile", "multiSelections"],
      });
      return result.canceled ? [] : result.filePaths;
    },
    stage_attachment: ({ name, bytes }) => attachments.stage(name, bytes),
    search_workspace_files: ({ root, query, limit, scopePath }) =>
      files.searchFiles(root, query, limit, scopePath),
    list_workspace_directory: ({ root, relativePath }) =>
      files.listDirectory(root, relativePath),
    resolve_file_reference: ({ request }) =>
      files.resolveFileReference(request),
    read_workspace_file: ({ root, relativePath }) =>
      files.readFile(root, relativePath),
    open_workspace_scope: ({ chatId }) => resources.open(chatId),
    list_workspace_resource_directory: ({ scopeId, relativePath, root }) =>
      resources.list(scopeId, relativePath, root),
    search_workspace_resource_files: ({
      scopeId,
      query,
      limit,
      scopePath,
      root,
    }) => resources.search(scopeId, query, limit, scopePath, root),
    save_workspace_resource_as: async ({ scopeId, relativePath, root }) => {
      if (!(await resources.stat(scopeId, relativePath, root)))
        throw new Error("另存的文件不存在");
      const result = await dialog.showSaveDialog(window, {
        defaultPath: path.basename(relativePath),
      });
      if (result.canceled || !result.filePath) return false;
      await resources.copyTo(scopeId, relativePath, result.filePath, root);
      return true;
    },
    rename_workspace_resource: ({
      scopeId,
      relativePath,
      nextName,
      expectedVersion,
      root,
    }) =>
      resources.rename(scopeId, relativePath, nextName, expectedVersion, root),
    delete_workspace_resource: async ({ scopeId, relativePath, root }) => {
      const resource = await resources.stat(scopeId, relativePath, root);
      if (!resource) throw new Error("文件不存在或已被删除");
      const response = await dialog.showMessageBox(window, {
        type: "warning",
        buttons: ["取消", "删除"],
        cancelId: 0,
        defaultId: 0,
        noLink: true,
        message: `删除“${path.basename(relativePath)}”？`,
        detail: "此操作会删除项目中的原文件。",
      });
      if (response.response !== 1) return false;
      await resources.removeFile(scopeId, relativePath, resource.version, root);
      return true;
    },
    rename_workspace_directory: async ({
      scopeId,
      relativePath,
      nextName,
      root,
    }) => {
      const version = await resources.directoryVersion(
        scopeId,
        relativePath,
        root,
      );
      return resources.renameDirectory(
        scopeId,
        relativePath,
        nextName,
        version,
        root,
      );
    },
    delete_workspace_directory: async ({ scopeId, relativePath, root }) => {
      const version = await resources.directoryVersion(
        scopeId,
        relativePath,
        root,
      );
      const response = await dialog.showMessageBox(window, {
        type: "warning",
        buttons: ["取消", "删除目录及全部内容"],
        cancelId: 0,
        defaultId: 0,
        noLink: true,
        message: `删除目录“${path.basename(relativePath)}”？`,
        detail: "此操作会删除该目录中的所有文件和子目录。",
      });
      if (response.response !== 1) return false;
      await resources.removeDirectory(scopeId, relativePath, version, root);
      return true;
    },
    import_workspace_resources: async ({ scopeId, directoryPath, root }) => {
      await resources.list(scopeId, directoryPath, root);
      const result = await dialog.showOpenDialog(window, {
        properties: ["openFile", "multiSelections"],
      });
      if (result.canceled) return { imported: [], failed: [] };
      return resources.importFiles(
        scopeId,
        directoryPath,
        result.filePaths,
        root,
      );
    },
    import_workspace_resource_bytes: ({
      scopeId,
      directoryPath,
      name,
      bytes,
      root,
    }) => resources.importBytes(scopeId, directoryPath, name, bytes, root),
    open_workspace_resource_external: async ({
      scopeId,
      relativePath,
      root,
    }) => {
      const target = await resources.filePath(scopeId, relativePath, root);
      const error = await shell.openPath(target);
      if (error) throw new Error(error);
      return true;
    },
    resolve_workspace_resource_reference: ({ scopeId, path, baseDirectory }) =>
      resources.resolveReference(scopeId, path, baseDirectory),
    stat_workspace_resource: ({ scopeId, relativePath, root }) =>
      resources.stat(scopeId, relativePath, root),
    read_workspace_resource: ({ scopeId, relativePath, requestId, root }) =>
      resources.read(scopeId, relativePath, requestId, root),
    write_workspace_resource: ({
      scopeId,
      root,
      relativePath,
      content,
      expectedVersion,
      saveRevision,
    }) =>
      resources.write(
        scopeId,
        relativePath,
        content,
        expectedVersion,
        saveRevision,
        root,
      ),
    cancel_workspace_resource_read: ({ scopeId, requestId }) =>
      resources.cancel(scopeId, requestId),
    watch_workspace_resource: ({ scopeId, relativePath, root, directory }) =>
      resources.watch(scopeId, relativePath, root, directory),
    unwatch_workspace_resource: ({ scopeId, subscriptionId }) =>
      resources.unwatch(scopeId, subscriptionId),
    write_workspace_file: ({ root, relativePath, content }) =>
      files.writeFile(root, relativePath, content),
    list_chat_workspace_files: async ({ chatId, root }) => {
      const scope = await resources.open(chatId);
      if ((await resources.verifyRoot(scope.id, root)) !== scope.root)
        throw new Error("会话文件目录不属于此 Chat");
      return files.listChatWorkspaceFiles(scope.root);
    },
    download_chat_workspace_file: async ({ chatId, root, relativePath }) => {
      const scope = await resources.open(chatId);
      if ((await resources.verifyRoot(scope.id, root)) !== scope.root)
        throw new Error("会话文件目录不属于此 Chat");
      const source = await files.chatWorkspaceFilePath(
        scope.root,
        relativePath,
      );
      const result = await dialog.showSaveDialog(window, {
        defaultPath: path.basename(source),
      });
      if (result.canceled || !result.filePath) return false;
      await copyFile(source, result.filePath);
      return true;
    },
    delete_chat_workspace_file: async ({ chatId, root, relativePath }) => {
      const scope = await resources.open(chatId);
      if ((await resources.verifyRoot(scope.id, root)) !== scope.root)
        throw new Error("会话文件目录不属于此 Chat");
      return files.deleteChatWorkspaceFile(scope.root, relativePath);
    },
    get_workspace_git_changes: ({ root }) => files.gitChanges(root),
    list_workspace_git_branches: ({ root }) => files.listGitBranches(root),
    preview_workspace_git_checkout: ({ root, branch }) =>
      files.previewGitCheckout(root, branch),
    checkout_workspace_git_branch: ({ root, branch, commitMessage }) =>
      files.checkoutGitBranch(root, branch, commitMessage),
    list_workspace_git_repositories: ({ roots }) =>
      files.listGitRepositories(roots),
    read_workspace_diff: ({ root, relativePath }) =>
      files.readDiff(root, relativePath),
    scoped_git_status: ({ chatId, root, repository }) =>
      scopedGit.status(chatId, root, repository),
    scoped_git_diff: ({ chatId, root, repository, relativePath, staged }) =>
      scopedGit.diff(chatId, root, repository, relativePath, staged),
    scoped_git_log: ({ chatId, root, repository, count, skip }) =>
      scopedGit.log(chatId, root, repository, count, skip),
    scoped_git_show: ({ chatId, root, repository, revision, relativePath }) =>
      scopedGit.show(chatId, root, repository, revision, relativePath),
    scoped_git_commit_diff: ({ chatId, root, repository, hash }) =>
      scopedGit.commitDiff(chatId, root, repository, hash),
    scoped_git_stage: ({ chatId, root, repository, relativePath }) =>
      scopedGit.stage(chatId, root, repository, relativePath),
    scoped_git_unstage: ({ chatId, root, repository, relativePath }) =>
      scopedGit.unstage(chatId, root, repository, relativePath),
    scoped_git_commit: ({ chatId, root, repository, message }) =>
      scopedGit.commit(chatId, root, repository, message),
    scoped_git_discard: async ({ chatId, root, repository, relativePath }) => {
      const response = await dialog.showMessageBox(window, {
        type: "warning",
        buttons: ["取消", "丢弃工作区改动"],
        cancelId: 0,
        defaultId: 0,
        noLink: true,
        message: `丢弃“${relativePath}”的未暂存改动？`,
        detail: "此操作无法撤销。已暂存内容会保留。",
      });
      if (response.response !== 1) return false;
      await scopedGit.discard(chatId, root, repository, relativePath);
      return true;
    },
    open_workspace_file: async ({ root, relativePath }) => {
      const error = await shell.openPath(
        await files.workspacePath(root, relativePath),
      );
      if (error) throw new Error(error);
      return true;
    },
    browser_create: async (input) => {
      if (!input.localRoot) return browser.create(input);
      if (new URL(input.url).protocol !== "file:" && !input.chatId)
        return browser.create({ ...input, localRoot: undefined });
      if (!input.chatId) throw new Error("本地浏览器预览缺少 Chat 身份");
      const scope = await resources.open(input.chatId);
      const localRoot = await resources.verifyRoot(scope.id, input.localRoot);
      return browser.create({ ...input, localRoot });
    },
    browser_prepare_webview: async ({ url, localRoot, chatId }) => {
      if (!localRoot || !chatId) return browserWebviews.prepare(url);
      const scope = await resources.open(chatId);
      const root = await resources.verifyRoot(scope.id, localRoot);
      return browserWebviews.prepare(url, root);
    },
    browser_navigate: ({ viewId, url }) => browser.navigate(viewId, url),
    browser_control: ({ viewId, action }) => browser.control(viewId, action),
    browser_set_bounds: ({ viewId, x, y, width, height }) =>
      browser.bounds(viewId, { x, y, width, height }),
    browser_set_visible: ({ viewId, visible }) =>
      browser.visible(viewId, visible),
    browser_close: ({ viewId }) => browser.close(viewId),
    open_external_url: async ({ url }) => {
      const parsed = new URL(url);
      if (!["http:", "https:"].includes(parsed.protocol))
        throw new Error("外链仅支持 HTTP 或 HTTPS");
      await shell.openExternal(parsed.href);
    },
    terminal_start: async ({ root, chatId, ownerKey, createWhenMissing }) => {
      const scope = await resources.open(chatId);
      const authorizedRoot = await resources.verifyRoot(scope.id, root);
      return terminals.start(authorizedRoot, ownerKey, createWhenMissing);
    },
    terminal_attach: ({ sessionId }) => terminals.attach(sessionId),
    terminal_detach: ({ sessionId, attachmentId }) =>
      terminals.detach(sessionId, attachmentId),
    terminal_write: ({ sessionId, attachmentId, data }) =>
      terminals.write(sessionId, attachmentId, data),
    terminal_resize: ({ sessionId, attachmentId, size }) =>
      terminals.resize(sessionId, attachmentId, size),
    terminal_close: ({ ownerKey }) => terminals.close(ownerKey),
    show_notification: ({ id, title, body }) => {
      if (!Notification.isSupported()) return false;
      notifications.get(id)?.close();
      const notification = new Notification({ title, body });
      notifications.set(id, notification);
      const finish = (action: "click" | "close") => {
        if (notifications.get(id) !== notification) return;
        notifications.delete(id);
        emit("notification-action", { id, action });
      };
      notification.on("click", () => {
        if (!window.isDestroyed()) {
          window.show();
          window.focus();
        }
        finish("click");
      });
      notification.on("close", () => finish("close"));
      notification.on("failed", () => finish("close"));
      notification.show();
      return true;
    },
  };
  const trusted = (url: string) =>
    url.split("#")[0] === trustedURL.split("#")[0];
  function register<K extends keyof DesktopCommands>(name: K) {
    ipcMain.handle(`ally:${name}`, (event, args: unknown) => {
      if (
        event.sender !== contents ||
        event.senderFrame !== contents.mainFrame ||
        !trusted(event.senderFrame.url)
      )
        throw new Error("桌面调用来源无效");
      // 共享 Schema 校验外部输入后才进入与同名契约绑定的处理函数。
      const input = commandSchemas[name].parse(args);
      return handlers[name](input);
    });
  }
  for (const name of Object.keys(commandSchemas) as (keyof DesktopCommands)[])
    register(name);
  const stopUpdates = updates?.subscribe((state) =>
    emit("application-update", state),
  );
  const releaseViews = () => {
    for (const subscription of eventSubscriptions.values()) subscription.stop();
    eventSubscriptions.clear();
    void attachments.dispose();
    browser.dispose();
    browserWebviews.reset();
    resources.dispose();
    for (const notification of notifications.values()) notification.close();
    notifications.clear();
  };
  const handleRenderProcessGone = () => releaseViews();
  const handleNavigation = (
    _event: unknown,
    _url: string,
    isInPlace: boolean,
    isMainFrame: boolean,
  ) => {
    if (isMainFrame && !isInPlace) {
      releaseViews();
      attachments = createAttachmentStaging();
      browser = new BrowserManager(window, (event) =>
        emit("browser-navigation", event),
      );
      resources = new WorkspaceResourceScopes(core, (event) =>
        emit("workspace-resource-change", event),
      );
      scopedGit = new ScopedGit(resources);
    }
  };
  contents.on("render-process-gone", handleRenderProcessGone);
  contents.on("did-start-navigation", handleNavigation);
  let disposed = false;
  return () => {
    if (disposed) return;
    disposed = true;
    stopUpdates?.();
    if (!window.isDestroyed() && !contents.isDestroyed()) {
      contents.off("render-process-gone", handleRenderProcessGone);
      contents.off("did-start-navigation", handleNavigation);
    }
    releaseViews();
    browserWebviews.dispose();
    terminals.dispose();
    for (const name of Object.keys(commandSchemas))
      ipcMain.removeHandler(`ally:${name}`);
  };
}

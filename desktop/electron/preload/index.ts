import { contextBridge, ipcRenderer, webUtils } from "electron";
import type {
  DesktopBridge,
  DesktopEvents,
  FileDropEvent,
} from "../shared/contracts";

function subscribe<K extends keyof DesktopEvents>(
  name: K,
  handler: (event: DesktopEvents[K]) => void,
): () => void {
  const listener = (
    _event: Electron.IpcRendererEvent,
    payload: DesktopEvents[K],
  ) => handler(payload);
  ipcRenderer.on(`ally:${name}`, listener);
  return () => ipcRenderer.removeListener(`ally:${name}`, listener);
}
const drops = new Set<(event: FileDropEvent) => void>();
for (const [name, type] of [
  ["dragenter", "enter"],
  ["dragover", "over"],
  ["drop", "drop"],
  ["dragleave", "leave"],
] as const) {
  window.addEventListener(
    name,
    (event: DragEvent) => {
      if (!event.dataTransfer?.types.includes("Files")) return;
      event.preventDefault();
      if (type === "leave" && event.relatedTarget !== null) return;
      const payload: FileDropEvent =
        type === "leave"
          ? { type }
          : {
              type,
              paths:
                type === "drop"
                  ? Array.from(event.dataTransfer.files)
                      .map((file) => webUtils.getPathForFile(file))
                      .filter(Boolean)
                  : [],
              position: { x: event.clientX, y: event.clientY },
            };
      for (const handler of drops) handler(payload);
    },
    true,
  );
}
const bridge: DesktopBridge = {
  application_update_state: () =>
    ipcRenderer.invoke("ally:application_update_state"),
  check_application_update: () =>
    ipcRenderer.invoke("ally:check_application_update"),
  download_application_update: () =>
    ipcRenderer.invoke("ally:download_application_update"),
  open_application_installer: () =>
    ipcRenderer.invoke("ally:open_application_installer"),
  onApplicationUpdate: (handler) => subscribe("application-update", handler),
  core_connection: (args) => ipcRenderer.invoke("ally:core_connection", args),
  subscribe_core_events: (args) =>
    ipcRenderer.invoke("ally:subscribe_core_events", args),
  resync_core_events: (args) =>
    ipcRenderer.invoke("ally:resync_core_events", args),
  unsubscribe_core_events: (args) =>
    ipcRenderer.invoke("ally:unsubscribe_core_events", args),
  onCoreEvent: (handler) => subscribe("core-event", handler),
  save_file: (args) => ipcRenderer.invoke("ally:save_file", args),
  select_directory: (args) => ipcRenderer.invoke("ally:select_directory", args),
  select_files: (args) => ipcRenderer.invoke("ally:select_files", args),
  stage_attachment: (args) => ipcRenderer.invoke("ally:stage_attachment", args),
  search_workspace_files: (args) =>
    ipcRenderer.invoke("ally:search_workspace_files", args),
  list_workspace_directory: (args) =>
    ipcRenderer.invoke("ally:list_workspace_directory", args),
  resolve_file_reference: (args) =>
    ipcRenderer.invoke("ally:resolve_file_reference", args),
  read_workspace_file: (args) =>
    ipcRenderer.invoke("ally:read_workspace_file", args),
  open_workspace_scope: (args) =>
    ipcRenderer.invoke("ally:open_workspace_scope", args),
  list_workspace_resource_directory: (args) =>
    ipcRenderer.invoke("ally:list_workspace_resource_directory", args),
  search_workspace_resource_files: (args) =>
    ipcRenderer.invoke("ally:search_workspace_resource_files", args),
  save_workspace_resource_as: (args) =>
    ipcRenderer.invoke("ally:save_workspace_resource_as", args),
  rename_workspace_resource: (args) =>
    ipcRenderer.invoke("ally:rename_workspace_resource", args),
  delete_workspace_resource: (args) =>
    ipcRenderer.invoke("ally:delete_workspace_resource", args),
  rename_workspace_directory: (args) =>
    ipcRenderer.invoke("ally:rename_workspace_directory", args),
  delete_workspace_directory: (args) =>
    ipcRenderer.invoke("ally:delete_workspace_directory", args),
  import_workspace_resources: (args) =>
    ipcRenderer.invoke("ally:import_workspace_resources", args),
  import_workspace_resource_bytes: (args) =>
    ipcRenderer.invoke("ally:import_workspace_resource_bytes", args),
  open_workspace_resource_external: (args) =>
    ipcRenderer.invoke("ally:open_workspace_resource_external", args),
  resolve_workspace_resource_reference: (args) =>
    ipcRenderer.invoke("ally:resolve_workspace_resource_reference", args),
  stat_workspace_resource: (args) =>
    ipcRenderer.invoke("ally:stat_workspace_resource", args),
  read_workspace_resource: (args) =>
    ipcRenderer.invoke("ally:read_workspace_resource", args),
  write_workspace_resource: (args) =>
    ipcRenderer.invoke("ally:write_workspace_resource", args),
  cancel_workspace_resource_read: (args) =>
    ipcRenderer.invoke("ally:cancel_workspace_resource_read", args),
  watch_workspace_resource: (args) =>
    ipcRenderer.invoke("ally:watch_workspace_resource", args),
  unwatch_workspace_resource: (args) =>
    ipcRenderer.invoke("ally:unwatch_workspace_resource", args),
  write_workspace_file: (args) =>
    ipcRenderer.invoke("ally:write_workspace_file", args),
  list_chat_workspace_files: (args) =>
    ipcRenderer.invoke("ally:list_chat_workspace_files", args),
  download_chat_workspace_file: (args) =>
    ipcRenderer.invoke("ally:download_chat_workspace_file", args),
  delete_chat_workspace_file: (args) =>
    ipcRenderer.invoke("ally:delete_chat_workspace_file", args),
  get_workspace_git_changes: (args) =>
    ipcRenderer.invoke("ally:get_workspace_git_changes", args),
  list_workspace_git_branches: (args) =>
    ipcRenderer.invoke("ally:list_workspace_git_branches", args),
  preview_workspace_git_checkout: (args) =>
    ipcRenderer.invoke("ally:preview_workspace_git_checkout", args),
  checkout_workspace_git_branch: (args) =>
    ipcRenderer.invoke("ally:checkout_workspace_git_branch", args),
  list_workspace_git_repositories: (args) =>
    ipcRenderer.invoke("ally:list_workspace_git_repositories", args),
  read_workspace_diff: (args) =>
    ipcRenderer.invoke("ally:read_workspace_diff", args),
  scoped_git_status: (args) =>
    ipcRenderer.invoke("ally:scoped_git_status", args),
  scoped_git_diff: (args) => ipcRenderer.invoke("ally:scoped_git_diff", args),
  scoped_git_log: (args) => ipcRenderer.invoke("ally:scoped_git_log", args),
  scoped_git_show: (args) => ipcRenderer.invoke("ally:scoped_git_show", args),
  scoped_git_commit_diff: (args) =>
    ipcRenderer.invoke("ally:scoped_git_commit_diff", args),
  scoped_git_stage: (args) => ipcRenderer.invoke("ally:scoped_git_stage", args),
  scoped_git_unstage: (args) =>
    ipcRenderer.invoke("ally:scoped_git_unstage", args),
  scoped_git_commit: (args) =>
    ipcRenderer.invoke("ally:scoped_git_commit", args),
  scoped_git_discard: (args) =>
    ipcRenderer.invoke("ally:scoped_git_discard", args),
  open_workspace_file: (args) =>
    ipcRenderer.invoke("ally:open_workspace_file", args),
  browser_create: (args) => ipcRenderer.invoke("ally:browser_create", args),
  browser_prepare_webview: (args) =>
    ipcRenderer.invoke("ally:browser_prepare_webview", args),
  browser_navigate: (args) => ipcRenderer.invoke("ally:browser_navigate", args),
  browser_control: (args) => ipcRenderer.invoke("ally:browser_control", args),
  browser_set_bounds: (args) =>
    ipcRenderer.invoke("ally:browser_set_bounds", args),
  browser_set_visible: (args) =>
    ipcRenderer.invoke("ally:browser_set_visible", args),
  browser_close: (args) => ipcRenderer.invoke("ally:browser_close", args),
  open_external_url: (args) =>
    ipcRenderer.invoke("ally:open_external_url", args),
  terminal_start: (args) => ipcRenderer.invoke("ally:terminal_start", args),
  terminal_attach: (args) => ipcRenderer.invoke("ally:terminal_attach", args),
  terminal_detach: (args) => ipcRenderer.invoke("ally:terminal_detach", args),
  terminal_write: (args) => ipcRenderer.invoke("ally:terminal_write", args),
  terminal_resize: (args) => ipcRenderer.invoke("ally:terminal_resize", args),
  terminal_close: (args) => ipcRenderer.invoke("ally:terminal_close", args),
  show_notification: (args) =>
    ipcRenderer.invoke("ally:show_notification", args),
  onBrowserNavigation: (handler) => subscribe("browser-navigation", handler),
  onWorkspaceResourceChange: (handler) =>
    subscribe("workspace-resource-change", handler),
  onTerminalOutput: (handler) => subscribe("terminal-output", handler),
  onTerminalExit: (handler) => subscribe("terminal-exit", handler),
  onNotificationAction: (handler) => subscribe("notification-action", handler),
  onFileDrop: (handler) => {
    drops.add(handler);
    return () => {
      drops.delete(handler);
    };
  },
};
contextBridge.exposeInMainWorld("allyDesktop", bridge);

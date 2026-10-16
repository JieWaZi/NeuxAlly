import { z } from "zod";
import { globalEventSubscriptionSchema } from "./core-event-protocol";

import type { DesktopCommands } from "./contracts";
import { workspaceSingleFileBytesLimit } from "./workspace-byte-limits";

type CommandSchemaMap = {
  [K in keyof DesktopCommands]: z.ZodType<DesktopCommands[K]["args"]>;
};

const text = z.string();
const root = z.string().min(1);
const chatId = z.string().regex(/^[0-9a-fA-F]{32}$/);
const bounds = {
  x: z.number().finite(),
  y: z.number().finite(),
  width: z.number().positive().finite(),
  height: z.number().positive().finite(),
};
const file = { root, relativePath: text };
export const commandSchemas = {
  core_connection: z.undefined(),
  subscribe_core_events: globalEventSubscriptionSchema,
  resync_core_events: z
    .object({
      subscriptionId: z.string().uuid(),
      after: z
        .union([z.number().int().nonnegative().safe(), z.string().max(512)])
        .optional(),
    })
    .strict(),
  unsubscribe_core_events: z
    .object({ subscriptionId: z.string().uuid() })
    .strict(),
  select_directory: z.undefined(),
  select_files: z.undefined(),
  stage_attachment: z.object({
    name: text
      .min(1)
      .refine((name) => name !== "." && name !== ".." && !/[\\/\0]/.test(name)),
    bytes: z.instanceof(Uint8Array),
  }),
  save_file: z.object({
    bytes: z.array(z.number().int().min(0).max(255)),
    fileName: text,
  }),
  search_workspace_files: z.object({
    root,
    query: text,
    limit: z.number().int().min(1).max(200),
    scopePath: text.optional(),
  }),
  list_workspace_directory: z.object(file),
  read_workspace_file: z.object(file),
  open_workspace_scope: z.object({ chatId }),
  list_workspace_resource_directory: z.object({
    scopeId: root,
    relativePath: text,
    root: root.optional(),
  }),
  search_workspace_resource_files: z.object({
    scopeId: root,
    query: text,
    limit: z.number().int().min(1).max(200),
    scopePath: text,
    root: root.optional(),
  }),
  save_workspace_resource_as: z.object({
    scopeId: root,
    relativePath: text,
    root: root.optional(),
  }),
  rename_workspace_resource: z.object({
    scopeId: root,
    relativePath: text.min(1),
    nextName: text.min(1),
    expectedVersion: text.min(1),
    root: root.optional(),
  }),
  delete_workspace_resource: z.object({
    scopeId: root,
    relativePath: text.min(1),
    root: root.optional(),
  }),
  rename_workspace_directory: z.object({
    scopeId: root,
    relativePath: text.min(1),
    nextName: text.min(1),
    root: root.optional(),
  }),
  delete_workspace_directory: z.object({
    scopeId: root,
    relativePath: text.min(1),
    root: root.optional(),
  }),
  import_workspace_resources: z.object({
    scopeId: root,
    directoryPath: text,
    root: root.optional(),
  }),
  import_workspace_resource_bytes: z.object({
    scopeId: root,
    directoryPath: text,
    root: root.optional(),
    name: text.min(1).max(255),
    bytes: z
      .instanceof(Uint8Array)
      .refine((value) => value.byteLength <= workspaceSingleFileBytesLimit),
  }),
  open_workspace_resource_external: z.object({
    scopeId: root,
    relativePath: text.min(1),
    root: root.optional(),
  }),
  resolve_workspace_resource_reference: z.object({
    scopeId: root,
    path: text.min(1),
    baseDirectory: root.optional(),
  }),
  stat_workspace_resource: z.object({
    scopeId: root,
    relativePath: text,
    root: root.optional(),
  }),
  read_workspace_resource: z.object({
    scopeId: root,
    relativePath: text,
    requestId: z.string().uuid(),
    root: root.optional(),
  }),
  write_workspace_resource: z.object({
    scopeId: root,
    root: root.optional(),
    relativePath: text,
    content: text,
    expectedVersion: z.string().min(1),
    saveRevision: z.number().int().nonnegative(),
  }),
  cancel_workspace_resource_read: z.object({
    scopeId: root,
    requestId: z.string().uuid(),
  }),
  watch_workspace_resource: z.object({
    scopeId: root,
    relativePath: text,
    root: root.optional(),
    directory: z.boolean().optional(),
  }),
  unwatch_workspace_resource: z.object({
    scopeId: root,
    subscriptionId: root,
  }),
  write_workspace_file: z.object({ ...file, content: text }),
  list_chat_workspace_files: z.object({ chatId, root }),
  download_chat_workspace_file: z.object({ chatId, ...file }),
  delete_chat_workspace_file: z.object({ chatId, ...file }),
  resolve_file_reference: z.object({
    request: z.object({
      path: text,
      baseDirectory: text,
      workspaceRoot: text,
      sourceRoots: z.array(text),
    }),
  }),
  get_workspace_git_changes: z.object({ root }),
  list_workspace_git_branches: z.object({ root }),
  preview_workspace_git_checkout: z.object({ root, branch: text.min(1) }),
  checkout_workspace_git_branch: z.object({
    root,
    branch: text.min(1),
    commitMessage: text.min(1).optional(),
  }),
  list_workspace_git_repositories: z.object({
    roots: z.array(root).max(32),
  }),
  read_workspace_diff: z.object(file),
  scoped_git_status: z.object({ chatId, root, repository: root.optional() }),
  scoped_git_diff: z.object({
    chatId,
    root,
    repository: root,
    relativePath: root,
    staged: z.boolean(),
  }),
  scoped_git_log: z.object({
    chatId,
    root,
    repository: root,
    count: z.number().int().min(1).max(100),
    skip: z.number().int().nonnegative().max(10000),
  }),
  scoped_git_show: z.object({
    chatId,
    root,
    repository: root,
    revision: root,
    relativePath: root,
  }),
  scoped_git_commit_diff: z.object({
    chatId,
    root,
    repository: root,
    hash: root,
  }),
  scoped_git_stage: z.object({
    chatId,
    root,
    repository: root,
    relativePath: root,
  }),
  scoped_git_unstage: z.object({
    chatId,
    root,
    repository: root,
    relativePath: root,
  }),
  scoped_git_commit: z.object({
    chatId,
    root,
    repository: root,
    message: text.min(1).max(4096),
  }),
  scoped_git_discard: z.object({
    chatId,
    root,
    repository: root,
    relativePath: root,
  }),
  open_workspace_file: z.object(file),
  browser_create: z.object({
    ...bounds,
    url: text,
    localRoot: text.optional(),
    chatId: text.optional(),
  }),
  browser_prepare_webview: z.object({
    url: text,
    localRoot: text.optional(),
    chatId: text.optional(),
  }),
  browser_navigate: z.object({ viewId: root, url: text }),
  browser_control: z.object({
    viewId: root,
    action: z.enum(["back", "forward", "reload"]),
  }),
  browser_set_bounds: z.object({
    viewId: root,
    ...bounds,
  }),
  browser_set_visible: z.object({ viewId: root, visible: z.boolean() }),
  browser_close: z.object({ viewId: root }),
  open_external_url: z.object({ url: text }),
  terminal_start: z.object({
    root,
    chatId: text.min(1),
    ownerKey: text.min(1).max(128),
    createWhenMissing: z.boolean(),
  }),
  terminal_attach: z.object({ sessionId: root }),
  terminal_detach: z.object({ sessionId: root, attachmentId: root }),
  terminal_write: z.object({
    sessionId: root,
    attachmentId: root,
    data: text.max(64 * 1024),
  }),
  terminal_resize: z.object({
    sessionId: root,
    attachmentId: root,
    size: z.object({
      rows: z.number().int().min(1).max(1000),
      cols: z.number().int().min(1).max(1000),
    }),
  }),
  terminal_close: z.object({ ownerKey: root }),
  show_notification: z.object({ id: root, title: text, body: text }),
} satisfies CommandSchemaMap;

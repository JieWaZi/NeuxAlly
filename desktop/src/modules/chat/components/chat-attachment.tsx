import {
  fileDirectory,
  isAbsoluteFilePath,
  type ResolvedFileReference,
} from "@/shared/workspace/file-reference";
import {
  AttachmentPrimitive,
  ComposerPrimitive,
  MessagePrimitive,
  useAui,
  useAuiState,
} from "@assistant-ui/react";
import { X } from "lucide-react";
import { useTranslation } from "react-i18next";
import type { AllyPathFile } from "../runtime/chat-attachment-adapter";
import {
  chatComposerReferenceChipClass,
  chatMessageReferenceChipClass,
} from "./chat-reference-chip";

export type ChatFileOpenRequest = {
  reference?: ResolvedFileReference;
  name: string;
  path: string;
  rootPath?: string;
};

function sourcePathPreviewOf(
  sourcePath: string,
  name: string,
): ChatFileOpenRequest {
  return {
    name,
    path: sourcePath,
    rootPath: isAbsoluteFilePath(sourcePath)
      ? fileDirectory(sourcePath)
      : undefined,
  };
}

function ChatComposerAttachmentChip({
  name,
  sourcePath,
  onOpenFile,
  onRemove,
}: {
  name: string;
  sourcePath: string;
  onOpenFile?: (file: ChatFileOpenRequest) => void;
  onRemove?: (path: string) => Promise<void> | void;
}) {
  const { t } = useTranslation("chat");
  const aui = useAui();
  const preview = sourcePathPreviewOf(sourcePath, name);
  return (
    <AttachmentPrimitive.Root
      data-chat-composer-attachment={sourcePath || undefined}
      className={`${chatComposerReferenceChipClass}${onOpenFile ? "cursor-pointer" : ""}`}
      role={onOpenFile ? "button" : undefined}
      tabIndex={onOpenFile ? 0 : undefined}
      onClick={onOpenFile ? () => onOpenFile(preview) : undefined}
      onKeyDown={
        onOpenFile
          ? (event) => {
              if (event.key !== "Enter" && event.key !== " ") return;
              event.preventDefault();
              onOpenFile(preview);
            }
          : undefined
      }
    >
      <span className="max-w-40 truncate">
        <AttachmentPrimitive.Name />
      </span>
      <button
        type="button"
        className="text-muted-foreground hover:text-foreground size-icon-sm ml-1 inline-flex shrink-0 items-center justify-center rounded-full transition-colors"
        aria-label={t("removeAttachment")}
        title={t("removeAttachment")}
        onClick={(event) => {
          event.preventDefault();
          event.stopPropagation();
          void (onRemove ? onRemove(sourcePath) : aui.attachment.remove());
        }}
      >
        <X className="size-icon-xs" />
      </button>
    </AttachmentPrimitive.Root>
  );
}

export function ChatComposerAttachments({
  onOpenFile,
  onRemove,
}: {
  onOpenFile?: (file: ChatFileOpenRequest) => void;
  onRemove?: (path: string) => Promise<void> | void;
}) {
  const attachments = useAuiState((state) => state.composer.attachments);
  return (
    <>
      {attachments.map((attachment, index) => (
        <ComposerPrimitive.AttachmentByIndex
          key={attachment.id}
          index={index}
          components={{
            Attachment: () => (
              <ChatComposerAttachmentChip
                name={attachment.name}
                sourcePath={
                  (attachment.file as AllyPathFile | undefined)
                    ?.allySourcePath ?? ""
                }
                onOpenFile={onOpenFile}
                onRemove={onRemove}
              />
            ),
          }}
        />
      ))}
    </>
  );
}

/** 用户上传与助手正式交付文件共用同一展示和工作台打开入口。 */
function MessageAttachmentFileChip({
  name,
  path,
  onOpenFile,
}: {
  name: string;
  path: string;
  onOpenFile?: (file: ChatFileOpenRequest) => void;
}) {
  return (
    <span className={chatMessageReferenceChipClass}>
      {onOpenFile && path ? (
        <button
          type="button"
          className="max-w-40 truncate text-left"
          onClick={() => onOpenFile({ name, path })}
        >
          {name}
        </button>
      ) : (
        <span className="max-w-40 truncate">{name}</span>
      )}
    </span>
  );
}
function ChatMessageAttachmentChip({
  onOpenFile,
}: {
  onOpenFile?: (file: ChatFileOpenRequest) => void;
}) {
  const attachment = useAuiState((state) => state.attachment);
  const path =
    attachment.content?.find((part) => part.type === "file")?.data ?? "";
  return (
    <AttachmentPrimitive.Root>
      <MessageAttachmentFileChip
        name={attachment.name}
        path={path}
        onOpenFile={onOpenFile}
      />
    </AttachmentPrimitive.Root>
  );
}

export function ChatMessageAttachments({
  onOpenFile,
}: {
  onOpenFile?: (file: ChatFileOpenRequest) => void;
}) {
  const role = useAuiState((state) => state.message.role);
  const delivered = useAuiState(
    (state) => state.message.metadata.custom.deliveredAttachments,
  );
  if (role === "assistant") {
    if (!Array.isArray(delivered)) return null;
    return (
      <div className="mt-2 flex min-w-0 flex-wrap gap-2">
        {delivered.flatMap((value: unknown) => {
          if (
            !value ||
            typeof value !== "object" ||
            !("path" in value) ||
            typeof value.path !== "string" ||
            !("name" in value) ||
            typeof value.name !== "string"
          )
            return [];
          return (
            <MessageAttachmentFileChip
              key={value.path}
              name={value.name}
              path={value.path}
              onOpenFile={onOpenFile}
            />
          );
        })}
      </div>
    );
  }
  return (
    <MessagePrimitive.Attachments>
      {() => <ChatMessageAttachmentChip onOpenFile={onOpenFile} />}
    </MessagePrimitive.Attachments>
  );
}

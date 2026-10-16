import { useState } from "react";
import { Badge } from "@/shared/ui/badge";
import { Dialog, DialogContent, DialogFooter } from "@/shared/ui/dialog";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/shared/ui/select";
import { PageAction, PageActionLabel } from "@/shared/ui/resource-page";
import { useMutation } from "@tanstack/react-query";
import { Plus } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Link, useSearchParams } from "react-router-dom";
import { selectImConversation } from "@/shared/api/generated/sdk.gen";
import {
  localApiErrorMessage,
  withLocalErrorSurface,
} from "@/shared/api/local-error-surface";
import { Button } from "@/shared/ui/button";
import { PaginatedTable } from "@/shared/ui/paginated-table";
import {
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/shared/ui/table";
import { useIMConversations } from "../queries";
import type { IMBinding } from "../types";

export function ConnectionConversationsTable({
  connectionId,
  bindings,
  binding,
  ready,
  onSaved,
  sourceError,
  onRetrySource,
}: {
  connectionId: string;
  bindings: IMBinding[];
  binding?: IMBinding;
  ready: boolean;
  onSaved: () => void;
  sourceError?: unknown;
  onRetrySource: () => void;
}) {
  const { t, i18n } = useTranslation("im");
  const [params, setParams] = useSearchParams();
  const query = useIMConversations(connectionId, bindings, ready);
  const inputPage = Number(params.get("page"));
  const requestedPage = Number.isFinite(inputPage)
    ? Math.max(1, Math.floor(inputPage))
    : 1;
  const pageSize = 10;
  const rows = query.data || [];
  const page = Math.min(
    requestedPage,
    Math.max(1, Math.ceil(rows.length / pageSize)),
  );
  const [creating, setCreating] = useState(false);
  const [newBindingId, setNewBindingId] = useState<string>();
  const approvedBindings = bindings.filter(
    (item) => item.state === "approved" && item.id,
  );
  const select = useMutation({
    mutationFn: ({
      bindingId,
      linkId,
    }: {
      bindingId: string;
      linkId: string;
    }) =>
      selectImConversation(
        withLocalErrorSurface({
          path: { bindingId },
          body: { linkId },
          throwOnError: true,
        }),
      ),
    onSuccess: () => {
      setCreating(false);
      onSaved();
    },
  });
  return (
    <section
      aria-label={t("conversations")}
      className="@container/page-header grid min-w-0 grid-cols-1 gap-4"
    >
      <header className="min-h-control-md flex flex-wrap items-center justify-between gap-3">
        <div className="flex min-w-0 flex-wrap items-center gap-3">
          <h2 className="type-title font-semibold">{t("conversations")}</h2>
          {query.data && !query.isError && !sourceError ? (
            <span className="text-content-secondary type-caption">
              {t("conversationSummary", { count: rows.length })}
            </span>
          ) : null}
        </div>
        <PageAction
          hitArea="comfortable"
          aria-label={t("newConversation")}
          disabled={!ready || !approvedBindings.length || select.isPending}
          onClick={() => {
            select.reset();
            if (approvedBindings.length === 1)
              select.mutate({
                bindingId: approvedBindings[0]?.id || "",
                linkId: "",
              });
            else {
              setNewBindingId(
                binding?.state === "approved" ? binding.id : undefined,
              );
              setCreating(true);
            }
          }}
        >
          <Plus />
          <PageActionLabel>{t("newConversation")}</PageActionLabel>
        </PageAction>
      </header>
      <PaginatedTable
        aria-label={t("conversations")}
        pagination={
          rows.length
            ? {
                page,
                pageSize,
                totalItems: rows.length,
                onPageChange: (value) => {
                  const next = new URLSearchParams(params);
                  next.set("page", String(value));
                  setParams(next);
                },
                labels: {
                  navigation: t("pagination"),
                  previous: t("previous"),
                  next: t("next"),
                  page: (value) => t("pageNumber", { page: value }),
                  pageSize: (value) => t("pageSize", { count: value }),
                  pageSizeSelection: t("pageSizeSelection"),
                  total: (value) => t("conversationCount", { count: value }),
                },
              }
            : undefined
        }
      >
        <TableHeader>
          <TableRow>
            <TableHead>{t("conversationTitle")}</TableHead>
            <TableHead>{t("turnCount")}</TableHead>
            <TableHead>{t("createdAt")}</TableHead>
            <TableHead>{t("operation")}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {sourceError || query.isError ? (
            <TableRow>
              <TableCell colSpan={4}>
                <p role="alert" className="text-destructive type-body">
                  {localApiErrorMessage(
                    sourceError || query.error,
                    t("common:detailState.failed"),
                  )}
                </p>
                <Button
                  size="sm"
                  variant="secondary"
                  onClick={() =>
                    sourceError ? onRetrySource() : void query.refetch()
                  }
                >
                  {t("reload")}
                </Button>
              </TableCell>
            </TableRow>
          ) : query.isPending || !ready ? (
            <TableRow>
              <TableCell colSpan={4}>
                <span role="status">{t("loading")}</span>
              </TableCell>
            </TableRow>
          ) : !rows.length ? (
            <TableRow>
              <TableCell
                colSpan={4}
                className="text-content-secondary text-center"
              >
                {t("noConversations")}
              </TableCell>
            </TableRow>
          ) : (
            rows
              .slice((page - 1) * pageSize, page * pageSize)
              .map(({ conversation, binding: item }) => (
                <TableRow key={conversation.id}>
                  <TableCell>
                    <div className="flex min-w-0 flex-wrap items-center gap-2">
                      <span className="[overflow-wrap:anywhere]">
                        {conversation.title || t("conversationTitle")}
                      </span>
                      {item.activeLinkId === conversation.id ? (
                        <Badge role="status" variant="success">
                          {t("currentConversation")}
                        </Badge>
                      ) : null}
                    </div>
                  </TableCell>
                  <TableCell className="text-content-secondary tabular-nums">
                    {conversation.turnCount ?? 0}
                  </TableCell>
                  <TableCell className="text-content-secondary whitespace-nowrap">
                    {conversation.createdAt
                      ? new Intl.DateTimeFormat(i18n.resolvedLanguage, {
                          dateStyle: "medium",
                          timeStyle: "short",
                        }).format(new Date(conversation.createdAt))
                      : "—"}
                  </TableCell>
                  <TableCell>
                    <div className="flex flex-wrap items-center gap-2">
                      {conversation.chatId ? (
                        <Button asChild size="sm" variant="ghost">
                          <Link
                            to={
                              conversation.threadId && item.channelId
                                ? `/teams/${item.channelId}/threads/${conversation.threadId}`
                                : `/chats/${conversation.chatId}`
                            }
                          >
                            {t("openWork")}
                          </Link>
                        </Button>
                      ) : null}
                      {item.activeLinkId !== conversation.id ? (
                        <Button
                          size="sm"
                          variant="ghost"
                          disabled={
                            !ready ||
                            select.isPending ||
                            item.state !== "approved"
                          }
                          onClick={() =>
                            select.mutate({
                              bindingId: item.id || "",
                              linkId: conversation.id || "",
                            })
                          }
                        >
                          {t("useConversation")}
                        </Button>
                      ) : null}
                    </div>
                  </TableCell>
                </TableRow>
              ))
          )}
        </TableBody>
      </PaginatedTable>
      {select.error && !creating ? (
        <p role="alert" className="text-destructive type-body">
          {localApiErrorMessage(select.error, t("operationFailed"))}
        </p>
      ) : null}
      <Dialog
        open={creating}
        onOpenChange={(open) => {
          if (!select.isPending) {
            setCreating(open);
            select.reset();
          }
        }}
      >
        <DialogContent
          title={t("newConversation")}
          description={t("newConversationHint")}
        >
          <div className="grid gap-2 px-6 pb-4">
            <label id="im-new-conversation-binding" className="type-ui">
              {t("chooseChatAccess")}
            </label>
            <Select
              value={newBindingId}
              onValueChange={setNewBindingId}
              disabled={select.isPending}
            >
              <SelectTrigger aria-labelledby="im-new-conversation-binding">
                <SelectValue placeholder={t("chooseChatAccess")} />
              </SelectTrigger>
              <SelectContent>
                {approvedBindings.map((item) => (
                  <SelectItem key={item.id} value={item.id || ""}>
                    {item.name || item.externalChatId || item.id}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            {select.error ? (
              <p role="alert" className="text-destructive type-body">
                {localApiErrorMessage(select.error, t("operationFailed"))}
              </p>
            ) : null}
          </div>
          <DialogFooter>
            <Button
              variant="secondary"
              disabled={select.isPending}
              onClick={() => {
                setCreating(false);
                select.reset();
              }}
            >
              {t("cancel")}
            </Button>
            <Button
              disabled={
                !ready ||
                select.isPending ||
                !approvedBindings.some((item) => item.id === newBindingId)
              }
              onClick={() =>
                select.mutate({ bindingId: newBindingId || "", linkId: "" })
              }
            >
              {t("newConversation")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </section>
  );
}

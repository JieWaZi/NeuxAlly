import { ReadOnlyField } from "@/shared/ui/read-only-field";
import { QueryState } from "@/shared/ui/query-state";
import { useMutation } from "@tanstack/react-query";
import { Pencil, ShieldCheck, ShieldOff, Trash2 } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate, useParams, useSearchParams } from "react-router-dom";
import { updateImBinding } from "@/shared/api/generated/sdk.gen";
import {
  localApiErrorMessage,
  withLocalErrorSurface,
} from "@/shared/api/local-error-surface";
import { Button } from "@/shared/ui/button";
import { Dialog, DialogContent, DialogFooter } from "@/shared/ui/dialog";
import { PageHeader } from "@/shared/ui/page-header";
import { PageAction, ResourcePageSurface } from "@/shared/ui/resource-page";
import { BindingAuthorizationDialog } from "./components/binding-authorization-dialog";
import {
  bindingAuthorizationConfiguration,
  canAuthorizeWithoutConfiguration,
} from "./model/binding-authorization";
import { BindingOverview } from "./components/binding-overview";
import { ConnectionConversationsTable } from "./components/connection-conversations-table";
import { ConnectionDeleteDialog } from "./components/connection-delete-dialog";
import { ConnectionDialog } from "./components/connection-dialog";
import {
  BindingStatus,
  ConnectionStatus,
} from "./components/connection-status";
import { PlatformIcon } from "./components/platform-icon";
import { useIMBindings, useIMConnections } from "./queries";
import type { IMBinding } from "./types";

export function IMConnectionDetailPage() {
  const { t, i18n } = useTranslation("im");
  const { connectionId = "" } = useParams();
  const [params] = useSearchParams();
  const query = useIMConnections();
  const bindingsQuery = useIMBindings(connectionId);
  const bindings = bindingsQuery.data || [];
  const binding =
    bindings.find((item) => item.id === params.get("chat")) ||
    bindings.find((item) => item.state !== "approved") ||
    bindings[0];
  const connection = query.data?.resources?.find(
    (item) => item.id === connectionId,
  );
  const [editing, setEditing] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const navigate = useNavigate();
  const [revokeTarget, setRevokeTarget] = useState<IMBinding>();
  const [authorizeTarget, setAuthorizeTarget] = useState<IMBinding>();
  const authorize = useMutation({
    mutationFn: (target: IMBinding) =>
      updateImBinding(
        withLocalErrorSurface({
          path: { bindingId: target.id || "" },
          body: bindingAuthorizationConfiguration(target, "approved"),
          throwOnError: true,
        }),
      ),
    onSuccess: () => query.refresh(),
  });
  const revoke = useMutation({
    mutationFn: (target: IMBinding) =>
      updateImBinding(
        withLocalErrorSurface({
          path: { bindingId: target.id || "" },
          body: bindingAuthorizationConfiguration(target, "rejected"),
          throwOnError: true,
        }),
      ),
    onSuccess: () => {
      setRevokeTarget(undefined);
      void query.refresh();
    },
  });
  function authorizationAction(target: IMBinding) {
    const approved = target.state === "approved";
    return (
      <div className="min-h-control-md flex flex-wrap items-center gap-3">
        <BindingStatus binding={target} />
        <PageAction
          intent={approved ? "revoke" : "secondary"}
          label={t(approved ? "revoke" : "authorize")}
          disabled={
            authorize.isPending || revoke.isPending || bindingsQuery.isError
          }
          onClick={() => {
            authorize.reset();
            if (approved) {
              revoke.reset();
              setRevokeTarget(target);
            } else if (canAuthorizeWithoutConfiguration(target))
              authorize.mutate(target);
            else setAuthorizeTarget(target);
          }}
        >
          {approved ? <ShieldOff /> : <ShieldCheck />}
        </PageAction>
      </div>
    );
  }
  return (
    <ResourcePageSurface>
      <PageHeader
        layout="detail"
        backAction={{
          label: t("common:backToList"),
          onClick: () => navigate("/im"),
        }}
        title={
          <span className="flex min-w-0 flex-wrap items-center gap-3">
            <span className="[overflow-wrap:anywhere]">
              {connection?.name || t("connectionDetail")}
            </span>
            {connection ? <ConnectionStatus connection={connection} /> : null}
          </span>
        }
        description={t("connectionDetailDescription")}
        visual={
          connection ? (
            <PlatformIcon platform={connection.platform} />
          ) : (
            <span className="size-avatar-md shrink-0" aria-hidden="true" />
          )
        }
        actions={
          <>
            <PageAction
              intent="edit"
              disabled={!connection}
              label={t("editConnectionAction")}
              onClick={() => setEditing(true)}
            >
              <Pencil />
            </PageAction>
            <PageAction
              intent="delete"
              disabled={!connection}
              label={t("deleteConnection")}
              onClick={() => setDeleting(true)}
            >
              <Trash2 />
            </PageAction>
          </>
        }
      />
      <main className="mt-6 grid gap-8">
        {!connection ? (
          <QueryState
            pending={query.isPending}
            error={query.error}
            emptyLabel={t("connectionNotFound")}
            onRetry={() => void query.refetch()}
          />
        ) : (
          <>
            {query.isError ? (
              <QueryState
                error={query.error}
                onRetry={() => void query.refetch()}
              />
            ) : null}
            <section
              aria-labelledby="im-connection-heading"
              className="grid min-w-0 grid-cols-1 gap-4"
            >
              <header className="min-h-control-md flex items-center">
                <h2
                  id="im-connection-heading"
                  className="type-title font-semibold"
                >
                  {t("connectionInformation")}
                </h2>
              </header>
              <dl className="grid min-w-0 grid-cols-[repeat(auto-fit,minmax(min(100%,18rem),1fr))] gap-x-8 gap-y-4">
                <ReadOnlyField
                  label={t(
                    connection.platform === "feishu" ? "appId" : "botAccount",
                  )}
                >
                  {connection.appId || t("notConnected")}
                </ReadOnlyField>
                <ReadOnlyField label={t("createdAt")}>
                  {connection.createdAt
                    ? new Intl.DateTimeFormat(i18n.resolvedLanguage, {
                        dateStyle: "medium",
                        timeStyle: "short",
                      }).format(new Date(connection.createdAt))
                    : "—"}
                </ReadOnlyField>
              </dl>
              {connection.errorCode ? (
                <p role="alert" className="text-destructive type-body">
                  {t("errors." + connection.errorCode, {
                    defaultValue: t("needsAttention"),
                  })}
                </p>
              ) : null}
            </section>
            <section
              aria-labelledby="im-binding-heading"
              className="@container/page-header grid min-w-0 grid-cols-1 gap-4"
            >
              <header className="min-h-control-md flex flex-wrap items-center justify-between gap-3">
                <h2
                  id="im-binding-heading"
                  className="type-title font-semibold"
                >
                  {t("chatAccess")}
                </h2>
                {bindings.length === 1 && bindings[0]
                  ? authorizationAction(bindings[0])
                  : null}
              </header>
              {bindingsQuery.isError ? (
                <QueryState
                  error={bindingsQuery.error}
                  onRetry={() => void bindingsQuery.refetch()}
                />
              ) : bindingsQuery.isPending ? (
                <p role="status" className="text-content-secondary type-body">
                  {t("loading")}
                </p>
              ) : bindings.length ? (
                bindings.map((item) => (
                  <div key={item.id} className="grid min-w-0 gap-5">
                    {bindings.length > 1 ? (
                      <header className="min-h-control-md flex flex-wrap items-center justify-between gap-3">
                        <h3 className="type-label min-w-0 font-semibold [overflow-wrap:anywhere]">
                          {item.name || item.externalChatId || t("chatAccess")}
                        </h3>
                        {authorizationAction(item)}
                      </header>
                    ) : null}
                    {authorize.error && authorize.variables?.id === item.id ? (
                      <p role="alert" className="text-destructive type-body">
                        {localApiErrorMessage(
                          authorize.error,
                          t("operationFailed"),
                        )}
                      </p>
                    ) : null}
                    <BindingOverview
                      binding={item}
                      nested={bindings.length > 1}
                    />
                  </div>
                ))
              ) : (
                <p className="text-content-secondary type-body">
                  {t("waitingForChat")}
                </p>
              )}
            </section>
            <ConnectionConversationsTable
              connectionId={connectionId}
              bindings={bindings}
              binding={binding}
              ready={!bindingsQuery.isPending && !bindingsQuery.isError}
              sourceError={bindingsQuery.error}
              onRetrySource={() => void bindingsQuery.refetch()}
              onSaved={() => void query.refresh()}
            />
          </>
        )}
      </main>
      {authorizeTarget ? (
        <BindingAuthorizationDialog
          key={authorizeTarget.id}
          binding={authorizeTarget}
          onClose={() => setAuthorizeTarget(undefined)}
          onSaved={() => void query.refresh()}
        />
      ) : null}
      {editing && connection ? (
        <ConnectionDialog
          key={connection.id}
          connection={connection}
          initialBindingId={binding?.id}
          onClose={() => setEditing(false)}
          onSaved={() => void query.refresh()}
        />
      ) : null}
      {deleting && connection ? (
        <ConnectionDeleteDialog
          connection={connection}
          onClose={() => setDeleting(false)}
          onDeleted={() => {
            void query.refresh();
            navigate("/im");
          }}
        />
      ) : null}
      {revokeTarget ? (
        <Dialog
          open
          onOpenChange={(open) => {
            if (!open && !revoke.isPending) {
              setRevokeTarget(undefined);
              revoke.reset();
            }
          }}
        >
          <DialogContent
            title={t("revoke")}
            description={t("revokeHint", { name: revokeTarget.name })}
          >
            {revoke.error ? (
              <p role="alert" className="text-destructive type-body px-6 pb-4">
                {localApiErrorMessage(revoke.error, t("operationFailed"))}
              </p>
            ) : null}
            <DialogFooter>
              <Button
                variant="secondary"
                disabled={revoke.isPending}
                onClick={() => {
                  setRevokeTarget(undefined);
                  revoke.reset();
                }}
              >
                {t("cancel")}
              </Button>
              <Button
                variant="danger"
                disabled={revoke.isPending}
                onClick={() => revoke.mutate(revokeTarget)}
              >
                {t("revoke")}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      ) : null}
    </ResourcePageSurface>
  );
}

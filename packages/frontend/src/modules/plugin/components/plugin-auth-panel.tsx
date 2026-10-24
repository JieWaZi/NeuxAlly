import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router-dom";
import {
  AuthorizationCode,
  PluginAuthorizationDialog,
} from "./plugin-authorization-dialog";
import { openExternalURL } from "@/shared/platform";
import { Button } from "@/shared/ui/button";
import { Input } from "@/shared/ui/input";
import { QueryState } from "@/shared/ui/query-state";
import { useQueryEvents } from "@/shared/api/use-query-events";
import { useRouteSceneActive } from "@/shared/workspace/app-scene-workspace";
import {
  localApiErrorMessage,
  withLocalErrorMutation,
  withLocalErrorSurface,
} from "@/shared/api/local-error-surface";
import {
  getPluginAuthorizationOptions,
  startPluginAuthorizationMutation,
  configurePluginAuthorizationMutation,
  cancelPluginAuthorizationMutation,
  logoutPluginAuthorizationMutation,
  invalidatePluginQueries,
} from "../api/plugin-queries";

export function PluginAuthPanel({
  pluginId,
  wakerId = "",
  versionId,
  onReady,
}: {
  pluginId: string;
  wakerId?: string;
  versionId?: string;
  onReady?: () => void;
}) {
  const { t } = useTranslation("plugin");
  const client = useQueryClient();
  const active = useRouteSceneActive();
  const scope = {
    path: { pluginId },
    query: { wakerId, versionId, verify: true },
  };
  const options = getPluginAuthorizationOptions(withLocalErrorSurface(scope));
  const status = useQuery({ ...options, enabled: active });
  useQueryEvents({
    topic: "plugins",
    resource: { kind: "global" },
    queryKeys: [options.queryKey],
    active,
  });
  const [editingApp, setEditingApp] = useState(false);
  const [appId, setAppId] = useState("");
  const [additionalScopes, setAdditionalScopes] = useState("");
  const [appSecret, setAppSecret] = useState("");
  const [openError, setOpenError] = useState("");
  const refresh = async () => {
    await invalidatePluginQueries(client);
    onReady?.();
  };
  const configure = useMutation({
    ...withLocalErrorMutation(configurePluginAuthorizationMutation()),
    onError: () => {
      void status.refetch();
    },
    onSuccess: async () => {
      setAppSecret("");
      setEditingApp(false);
      await refresh();
    },
  });
  const login = useMutation({
    ...withLocalErrorMutation(startPluginAuthorizationMutation()),
    onError: () => {
      void status.refetch();
    },
    onSuccess: refresh,
  });
  const cancel = useMutation({
    ...withLocalErrorMutation(cancelPluginAuthorizationMutation()),
    onSuccess: refresh,
  });
  const logout = useMutation({
    ...withLocalErrorMutation(logoutPluginAuthorizationMutation()),
    onError: () => {
      void status.refetch();
    },
    onSuccess: refresh,
  });
  const github = status.data?.provider === "github";
  const account = status.data?.account;
  const activeUses = status.data?.activeUses ?? [];
  const session = status.data?.session;
  const verificationURL = session?.verificationUrl ?? "";
  const waiting = session?.state === "waiting";
  const ready = Boolean(account?.available);
  const pending =
    configure.isPending ||
    login.isPending ||
    cancel.isPending ||
    logout.isPending;
  const error = configure.error ?? login.error ?? cancel.error ?? logout.error;
  const message =
    session?.state === "expired"
      ? t("authExpired")
      : session?.state === "failed"
        ? t("authFailed")
        : session?.state === "cancelled"
          ? t("authCancelled")
          : t("authUnknown");
  return (
    <section className="space-y-4">
      <div>
        <h3 className="type-label font-medium">{t("authentication")}</h3>
        <p className="type-body text-muted-foreground mt-1">
          {t(github ? "githubAccountHelp" : "accountHelp")}
        </p>
      </div>
      {status.isPending || status.isError ? (
        <QueryState
          pending={status.isPending}
          error={status.error}
          onRetry={() => void status.refetch()}
        />
      ) : (
        <>
          {!github && (!account?.configured || editingApp) && (
            <div className="space-y-3">
              <p className="type-caption text-muted-foreground">
                {t("appCredentialsHelp")}
              </p>
              <label className="type-body grid gap-2">
                {t("appId")}
                <Input
                  value={appId}
                  placeholder={t("appIdExample")}
                  autoComplete="off"
                  onChange={(event) => setAppId(event.target.value)}
                />
              </label>
              <label className="type-body grid gap-2">
                {t("appSecret")}
                <Input
                  type="password"
                  value={appSecret}
                  placeholder={t("appSecretHelp")}
                  autoComplete="new-password"
                  onChange={(event) => setAppSecret(event.target.value)}
                />
              </label>
              <Button
                disabled={
                  pending ||
                  activeUses.length > 0 ||
                  !appId.trim() ||
                  !appSecret.trim()
                }
                onClick={() =>
                  configure.mutate({
                    path: { pluginId },
                    query: { wakerId, versionId },
                    body: { appId: appId.trim(), appSecret },
                  })
                }
              >
                {t("configureApp")}
              </Button>
              {editingApp && (
                <Button
                  variant="ghost"
                  disabled={pending}
                  onClick={() => {
                    setEditingApp(false);
                    setAppSecret("");
                  }}
                >
                  {t("cancel")}
                </Button>
              )}
            </div>
          )}
          {ready ? (
            <p className="type-body">
              {t("sharedIdentity", { name: account?.name || account?.id })}
            </p>
          ) : (
            <p className="type-body text-muted-foreground" role="status">
              {waiting ? t(github ? "githubWaiting" : "authWaiting") : message}
            </p>
          )}
          {waiting && session?.verificationUrl && (
            <div className="space-y-3">
              {session.userCode && (
                <AuthorizationCode
                  key={session.userCode}
                  code={session.userCode}
                />
              )}
              <div className="flex flex-wrap gap-2">
                <PluginAuthorizationDialog
                  key={session.id}
                  url={verificationURL}
                  code={session.userCode}
                />
                <Button
                  variant="secondary"
                  onClick={() => {
                    setOpenError("");
                    void openExternalURL(verificationURL).catch(() =>
                      setOpenError(t("failed")),
                    );
                  }}
                >
                  {t("openAuth")}
                </Button>
              </div>
              {session.expiresAt && (
                <p className="type-caption text-muted-foreground">
                  {t(github ? "localWaitUntil" : "expires", {
                    time: new Date(session.expiresAt).toLocaleTimeString(),
                  })}
                </p>
              )}
            </div>
          )}
          {account?.configured && !waiting && (
            <label className="type-body grid gap-2">
              {t("additionalScopes")}
              <span className="type-caption text-muted-foreground">
                {t("additionalScopesHelp")}
              </span>
              <Input
                value={additionalScopes}
                aria-label={t("additionalScopes")}
                disabled={pending}
                placeholder={t("additionalScopesHelp")}
                onChange={(event) => setAdditionalScopes(event.target.value)}
              />
            </label>
          )}
          {Boolean(status.data?.missingScopes?.length) && (
            <p className="type-caption text-muted-foreground break-all">
              {t("missingScopes")} {status.data?.missingScopes?.join("、")}
            </p>
          )}
          <div className="flex flex-wrap gap-2">
            {account?.configured && !waiting && !editingApp && (
              <Button
                disabled={pending || activeUses.length > 0}
                onClick={() =>
                  login.mutate({
                    path: { pluginId },
                    query: { wakerId, versionId },
                    body: {
                      additionalScopes: additionalScopes
                        .split(/[,\s]+/)
                        .filter(Boolean),
                    },
                  })
                }
              >
                {t("login")}
              </Button>
            )}
            {waiting && session?.id && (
              <Button
                variant="secondary"
                disabled={pending}
                onClick={() =>
                  cancel.mutate({
                    path: { pluginId, sessionId: session.id ?? "" },
                    query: { wakerId, versionId },
                  })
                }
              >
                {t("cancelAuth")}
              </Button>
            )}
            {!github && account?.configured && !waiting && !editingApp && (
              <Button
                variant="ghost"
                disabled={pending || activeUses.length > 0}
                onClick={() => {
                  setAppId(account.appId ?? "");
                  setEditingApp(true);
                }}
              >
                {t("changeApp")}
              </Button>
            )}
            <Button
              variant="ghost"
              disabled={pending}
              onClick={() => void status.refetch().then(refresh)}
            >
              {t("authRefresh")}
            </Button>
            {ready && (
              <Button
                variant="ghost"
                disabled={pending || activeUses.length > 0}
                onClick={() =>
                  logout.mutate({
                    path: { pluginId },
                    query: { wakerId, versionId },
                  })
                }
              >
                {t("logout")}
              </Button>
            )}
          </div>
          {activeUses.length > 0 && (
            <section className="space-y-2">
              <h4 className="type-label">{t("activeWork")}</h4>
              {activeUses.map((use) => (
                <Link
                  key={use.executionId}
                  to={`/chats/${use.chatId}`}
                  className="type-body text-primary block underline underline-offset-4"
                >
                  {use.title || t("viewActiveWork")}
                </Link>
              ))}
            </section>
          )}
          <p className="type-caption text-muted-foreground">
            {t(github ? "githubScopeHelp" : "scopeHelp")}
          </p>
        </>
      )}
      {(error || openError) && (
        <p role="alert" className="type-body text-destructive">
          {openError || localApiErrorMessage(error, t("failed"))}
        </p>
      )}
    </section>
  );
}

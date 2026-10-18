import { StatusNotice } from "@/shared/ui/status-notice";
import {
  hashKey,
  useMutation,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import { useState, useEffect, useEffectEvent, useRef } from "react";
import { RefreshCw, Unplug } from "lucide-react";
import type { GetPluginAuthorizationResponse } from "@/shared/api/generated/types.gen";
import { useTranslation } from "react-i18next";
import { Link } from "react-router-dom";
import {
  AuthorizationCode,
  PluginAuthorizationDialog,
  AuthorizationQRCode,
} from "./plugin-authorization-dialog";
import { openExternalURL } from "@/shared/desktop";
import { Button } from "@/shared/ui/button";
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
  cancelPluginAuthorizationMutation,
  logoutPluginAuthorizationMutation,
  invalidatePluginQueries,
} from "../api/plugin-queries";

export function PluginAuthPanel({
  pluginId,
  wakerId = "",
  versionId,
  onReady,
  installation = false,
  starting = false,
}: {
  pluginId: string;
  wakerId?: string;
  versionId?: string;
  onReady?: () => void;
  installation?: boolean;
  starting?: boolean;
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
  const [openError, setOpenError] = useState("");
  const [expiredDevice, setExpiredDevice] = useState("");
  const refresh = async () => {
    await invalidatePluginQueries(client);
  };
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
  const deviceKey = `${session?.id ?? ""}:${session?.expiresAt ?? ""}`;
  const expired = session?.state === "expired" || expiredDevice === deviceKey;
  const waiting = session?.state === "waiting" && !expired;
  const ready = Boolean(account?.available);
  const pending =
    starting || login.isPending || cancel.isPending || logout.isPending;
  const error = login.error ?? cancel.error ?? logout.error;
  const message = expired
    ? t("authExpired")
    : session?.state === "failed"
      ? t("authFailed")
      : session?.state === "cancelled"
        ? t("authCancelled")
        : t("authUnknown");
  const notifyReady = useEffectEvent(() => onReady?.());
  const notifiedSession = useRef("");
  const statusHash = hashKey(options.queryKey);
  useEffect(() => {
    const cache = client.getQueryCache();
    const notify = () => {
      const current =
        cache.get<GetPluginAuthorizationResponse>(statusHash)?.state.data;
      const id = current?.session?.id;
      if (
        id &&
        current.session?.state === "ready" &&
        current.account?.available &&
        notifiedSession.current !== id
      ) {
        notifiedSession.current = id;
        notifyReady();
      }
    };
    const close = cache.subscribe((event) => {
      if (
        event.type === "updated" &&
        event.action.type === "success" &&
        event.query.queryHash === statusHash
      )
        notify();
    });
    notify();
    return close;
  }, [client, statusHash]);
  const onDeadline = useEffectEvent((expiredKey: string) => {
    setExpiredDevice(expiredKey);
    void status.refetch();
  });
  useEffect(() => {
    if (session?.state !== "waiting" || !session.expiresAt) return;
    // 当前设备期限只唤醒一次：移除旧二维码并对账，不轮询认证状态。
    const timer = setTimeout(
      () => onDeadline(deviceKey),
      Math.max(0, session.expiresAt - Date.now()),
    );
    return () => clearTimeout(timer);
  }, [deviceKey, session?.expiresAt, session?.state]);
  async function restart() {
    setOpenError("");
    if (session?.state === "waiting") setExpiredDevice(deviceKey);
    try {
      if (session?.state === "waiting" && session.id) {
        await cancel.mutateAsync({
          path: { pluginId, sessionId: session.id },
          query: { wakerId, versionId },
        });
      }
      await login.mutateAsync({
        path: { pluginId },
        query: { wakerId, versionId },
        body: { additionalScopes: status.data?.missingScopes ?? [] },
      });
    } catch {
      // Mutation 的稳定错误在当前面板统一呈现。
    }
  }
  if (installation)
    return (
      <section className="space-y-4">
        {status.isPending || status.isError ? (
          <QueryState
            loadingLayout="content"
            pending={status.isPending}
            error={status.error}
            onRetry={() => void status.refetch()}
          />
        ) : (
          <>
            <StatusNotice>
              {t(
                github
                  ? "githubAuthorizationNotice"
                  : "feishuAuthorizationNotice",
              )}
            </StatusNotice>
            {waiting && verificationURL ? (
              <>
                {github ? (
                  <>
                    <AuthorizationCode code={session?.userCode ?? ""} />
                    <Button
                      type="button"
                      className="w-full"
                      onClick={() => {
                        setOpenError("");
                        void openExternalURL(verificationURL).catch(() =>
                          setOpenError(t("failed")),
                        );
                      }}
                    >
                      {t("authorizeOnWebsite")}
                    </Button>
                  </>
                ) : (
                  <AuthorizationQRCode url={verificationURL} />
                )}
                <p
                  role="status"
                  className="type-caption text-content-caption text-center"
                >
                  {t(github ? "githubWaiting" : "authWaiting")}
                </p>
                {!github && (
                  <Button
                    type="button"
                    variant="secondary"
                    className="mx-auto flex"
                    disabled={pending || activeUses.length > 0}
                    onClick={() => void restart()}
                  >
                    <RefreshCw />
                    {t("refreshQR")}
                  </Button>
                )}
              </>
            ) : (
              !ready && (
                <div className="space-y-3">
                  <p role="status" className="type-body text-content-secondary">
                    {pending ? t("busy") : message}
                  </p>
                  <Button
                    type="button"
                    className="w-full"
                    disabled={pending || activeUses.length > 0}
                    onClick={() => void restart()}
                  >
                    <RefreshCw />
                    {t(github ? "retryAuthorization" : "refreshQR")}
                  </Button>
                </div>
              )
            )}
            {activeUses.length > 0 && (
              <p className="type-body text-content-secondary">
                {t("accountInUse")}
              </p>
            )}
          </>
        )}
        {(error || openError) && (
          <p role="alert" className="type-body text-destructive">
            {openError || localApiErrorMessage(error, t("failed"))}
          </p>
        )}
      </section>
    );
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
          loadingLayout="content"
          pending={status.isPending}
          error={status.error}
          onRetry={() => void status.refetch()}
        />
      ) : (
        <>
          {!github && !account?.configured && (
            <p className="type-body text-content-secondary">
              {t("setupScanHelp")}
            </p>
          )}
          {ready ? (
            <StatusNotice tone="success">
              {t("sharedIdentity", { name: account?.name || account?.id })}
            </StatusNotice>
          ) : (
            <p className="type-body text-muted-foreground" role="status">
              {waiting ? t(github ? "githubWaiting" : "authWaiting") : message}
            </p>
          )}
          {waiting && session?.verificationUrl && (
            <div className="space-y-3">
              {!github && <AuthorizationQRCode url={verificationURL} />}
              {!github && (
                <Button
                  type="button"
                  variant="secondary"
                  disabled={pending || activeUses.length > 0}
                  onClick={() => void restart()}
                >
                  <RefreshCw />
                  {t("refreshQR")}
                </Button>
              )}
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
                  type="button"
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
                  {t("localWaitUntil", {
                    time: new Date(session.expiresAt).toLocaleTimeString(),
                  })}
                </p>
              )}
            </div>
          )}
          {Boolean(status.data?.missingScopes?.length) && (
            <p className="type-body text-content-secondary">
              {t("missingScopesHelp")}
            </p>
          )}
          <div className="flex flex-wrap gap-2">
            {!waiting && !ready && (
              <Button
                type="button"
                disabled={pending || activeUses.length > 0}
                onClick={() => void restart()}
              >
                {t(
                  account?.configured
                    ? "retryAuthorization"
                    : github
                      ? "login"
                      : "scanLogin",
                )}
              </Button>
            )}
            {waiting && session?.id && (
              <Button
                type="button"
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
            <Button
              type="button"
              variant="ghost"
              disabled={pending}
              onClick={() => void status.refetch().then(refresh)}
            >
              <RefreshCw aria-hidden="true" strokeWidth={1.5} />
              {t("authRefresh")}
            </Button>
            {ready && (
              <Button
                type="button"
                variant="ghost"
                disabled={pending || activeUses.length > 0}
                onClick={() =>
                  logout.mutate({
                    path: { pluginId },
                    query: { wakerId, versionId },
                  })
                }
              >
                <Unplug aria-hidden="true" strokeWidth={1.5} />
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

import { ReadOnlyField } from "@/shared/ui/read-only-field";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { getProjectOptions } from "@/shared/api/generated/@tanstack/react-query.gen";
import { withLocalErrorSurface } from "@/shared/api/local-error-surface";
import { useWorkExecutors } from "@/shared/hooks/use-work-executors";
import { runtimeDisplayName } from "@/shared/lib/runtime-display-name";
import {
  permissionLevelOf,
  permissionLevelTranslationKeys,
} from "@/shared/lib/runtime-permissions";
import type { IMBinding } from "../types";
import { Card } from "@/shared/ui/card";
import { Button } from "@/shared/ui/button";
import { localApiErrorMessage } from "@/shared/api/local-error-surface";
import type { ReactNode } from "react";

export function BindingOverview({
  binding,
  nested = false,
}: {
  binding: IMBinding;
  nested?: boolean;
}) {
  const { t } = useTranslation("im");
  const executors = useWorkExecutors();
  const team = executors.data?.teams.find(
    (item) => item.channel?.id === binding.channelId,
  );
  const owner = binding.channelId
    ? team?.channel?.leadWakerId
    : binding.wakerId;
  const projects = useQuery({
    ...getProjectOptions(
      withLocalErrorSurface({
        path: { wakerId: owner || "", projectId: binding.projectId || "" },
      }),
    ),
    enabled: Boolean(owner && binding.projectId),
  });
  const target = binding.channelId
    ? team?.channel?.name
    : executors.data?.wakers.find((item) => item.id === binding.wakerId)?.name;
  const level =
    permissionLevelOf(binding.options?.permissionLevel) || "default";
  const Heading = nested ? "h4" : "h3";
  const hasConfiguration = Boolean(binding.wakerId || binding.channelId);
  function loadedValue(
    pending: boolean,
    error: unknown,
    retry: () => void,
    value: ReactNode,
  ) {
    if (error)
      return (
        <span role="alert" className="text-destructive">
          {localApiErrorMessage(error, t("common:detailState.failed"))}
          <Button size="sm" variant="ghost" onClick={retry}>
            {t("reload")}
          </Button>
        </span>
      );
    return pending ? (
      <span role="status" className="text-content-secondary">
        {t("loading")}
      </span>
    ) : (
      value
    );
  }
  return (
    <div className="grid min-w-0 gap-5">
      <dl className="grid min-w-0 grid-cols-[repeat(auto-fit,minmax(min(100%,18rem),1fr))] gap-x-8 gap-y-4">
        <ReadOnlyField label={t("externalChatId")}>
          {binding.externalChatId || binding.name || "—"}
        </ReadOnlyField>
        <ReadOnlyField label={t("kind")}>
          {t(binding.kind === "group" ? "group" : "direct")}
        </ReadOnlyField>
      </dl>
      {hasConfiguration ? (
        <Card variant="section">
          <div className="grid min-w-0 grid-cols-[repeat(auto-fit,minmax(min(100%,16rem),1fr))] gap-x-8 gap-y-6">
            <section aria-label={t("runtimeConfiguration")}>
              <Heading className="type-label font-semibold">
                {t("runtimeConfiguration")}
              </Heading>
              <dl className="mt-4 grid gap-3 [&_dd]:mt-0 [&>div]:grid [&>div]:grid-cols-[6rem_minmax(0,1fr)] [&>div]:items-baseline [&>div]:gap-4">
                <ReadOnlyField
                  label={t(binding.channelId ? "team" : "partner")}
                >
                  {loadedValue(
                    executors.isPending,
                    executors.error,
                    () => void executors.refetch(),
                    target || t("notConfigured"),
                  )}
                </ReadOnlyField>
                {!binding.channelId ? (
                  <>
                    <ReadOnlyField label={t("agent")}>
                      {runtimeDisplayName({ id: binding.options?.runtimeId }) ||
                        t("managed")}
                    </ReadOnlyField>
                    <ReadOnlyField label={t("model")}>
                      {binding.options?.model || t("managed")}
                    </ReadOnlyField>
                  </>
                ) : null}
              </dl>
              {binding.channelId ? (
                <p className="text-content-secondary type-ui mt-4">
                  {t("teamDefaults")}
                </p>
              ) : null}
            </section>
            <section aria-label={t("permissionsAndScope")}>
              <Heading className="type-label font-semibold">
                {t("permissionsAndScope")}
              </Heading>
              <dl className="mt-4 grid gap-3 [&_dd]:mt-0 [&>div]:grid [&>div]:grid-cols-[6rem_minmax(0,1fr)] [&>div]:items-baseline [&>div]:gap-4">
                {!binding.channelId ? (
                  <ReadOnlyField label={t("permission")}>
                    {t(`common:${permissionLevelTranslationKeys[level].label}`)}
                  </ReadOnlyField>
                ) : null}
                <ReadOnlyField label={t("scope")}>
                  {binding.workingDirectory ||
                    (binding.projectId
                      ? loadedValue(
                          owner ? projects.isPending : executors.isPending,
                          projects.error || (!owner ? executors.error : null),
                          () => {
                            void executors.refetch();
                            if (owner) void projects.refetch();
                          },
                          projects.data?.name || binding.projectId,
                        )
                      : t("managed"))}
                </ReadOnlyField>
                <ReadOnlyField label={t("workflowDelivery")}>
                  {t(binding.allowWorkflowDelivery ? "allowed" : "notAllowed")}
                </ReadOnlyField>
              </dl>
            </section>
          </div>
        </Card>
      ) : (
        <p className="text-content-secondary type-body">{t("notConfigured")}</p>
      )}
    </div>
  );
}

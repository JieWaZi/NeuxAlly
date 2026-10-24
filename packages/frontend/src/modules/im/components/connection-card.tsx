import {
  Ellipsis,
  FolderOpen,
  MessageCircle,
  Pencil,
  ShieldCheck,
  Trash2,
  TriangleAlert,
  Users,
} from "lucide-react";
import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { Link } from "react-router-dom";
import { TeamAvatarStack } from "@/modules/team";
import { Avatar } from "@/shared/ui/avatar";
import { Checkbox } from "@/shared/ui/checkbox";
import { RuntimeCliIcon } from "@/shared/ui/runtime-cli-icon";
import { runtimeDisplayName } from "@/shared/lib/runtime-display-name";
import {
  permissionLevelOf,
  permissionLevelTranslationKeys,
} from "@/shared/lib/runtime-permissions";
import { getProjectOptions } from "@/shared/api/generated/@tanstack/react-query.gen";
import { withLocalErrorSurface } from "@/shared/api/local-error-surface";
import { Button } from "@/shared/ui/button";
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
} from "@/shared/ui/dropdown-menu";
import { ConnectionDialog } from "./connection-dialog";
import { ConnectionDeleteDialog } from "./connection-delete-dialog";
import { Card, CardFooter } from "@/shared/ui/card";
import { cn } from "@/shared/lib/cn";
import { Badge } from "@/shared/ui/badge";
import { useWorkExecutors } from "@/shared/hooks/use-work-executors";
import type { IMConnection, IMBinding } from "../types";
import { PlatformIcon } from "./platform-icon";
import { ConnectionStatus } from "./connection-status";

export function ConnectionCard({
  connection,
  onSaved,
  bindings,
  selected = false,
  onSelectedChange,
}: {
  connection: IMConnection;
  onSaved: () => void;
  bindings: IMBinding[];
  selected?: boolean;
  onSelectedChange?: (selected: boolean) => void;
}) {
  const { t } = useTranslation("im");
  const executors = useWorkExecutors();
  const approved = bindings.filter((binding) => binding.state === "approved");
  const pendingAuthorization = bindings.some(
    (binding) => binding.state !== "approved",
  );
  const targets = [
    ...new Map(
      approved.map((binding) => [
        JSON.stringify([
          binding.channelId,
          binding.wakerId,
          binding.channelId ? undefined : binding.options?.runtimeId,
          binding.channelId ? undefined : binding.options?.model,
          binding.projectId,
          binding.workingDirectory,
          binding.channelId
            ? undefined
            : permissionLevelOf(binding.options?.permissionLevel) || "default",
        ]),
        binding,
      ]),
    ).values(),
  ];
  const primaryBinding = targets[0];
  const team = primaryBinding?.channelId
    ? executors.data?.teams.find(
        (item) => item.channel?.id === primaryBinding.channelId,
      )
    : undefined;
  const waker = executors.data?.wakers.find(
    (item) => item.id === primaryBinding?.wakerId,
  );
  const targetName =
    team?.channel?.name || waker?.name || t("responseConfigured");
  const ownerId = primaryBinding?.channelId
    ? team?.channel?.leadWakerId
    : primaryBinding?.wakerId;
  const project = useQuery({
    ...getProjectOptions(
      withLocalErrorSurface({
        path: {
          wakerId: ownerId || "",
          projectId: primaryBinding?.projectId || "",
        },
      }),
    ),
    enabled: Boolean(
      ownerId && primaryBinding?.projectId && !primaryBinding.workingDirectory,
    ),
  });
  const scope =
    primaryBinding?.workingDirectory ||
    (primaryBinding?.projectId
      ? project.isError
        ? t("needsAttention")
        : project.data?.name || t("loading")
      : t("managed"));
  const permission = primaryBinding?.channelId
    ? t("cardTeamPermissions")
    : t(
        `common:${permissionLevelTranslationKeys[permissionLevelOf(primaryBinding?.options?.permissionLevel) || "default"].label}`,
      );
  const errorMessage = connection.errorCode
    ? t("errors." + connection.errorCode, { defaultValue: t("needsAttention") })
    : undefined;
  const [editing, setEditing] = useState(false);
  const [deleting, setDeleting] = useState(false);
  return (
    <Card
      asChild
      variant="resource"
      className={cn("h-im-connection-card", selected && "bg-accent-surface")}
    >
      <article>
        <div className="hover:bg-surface-subtle relative min-h-0 flex-1 overflow-hidden transition-colors">
          <Link
            to={`/im/${connection.id}`}
            title={[
              connection.name,
              connection.appId,
              primaryBinding
                ? `${t("cardScope", { scope })} · ${permission}`
                : undefined,
              ...targets.map((binding) =>
                binding.channelId
                  ? executors.data?.teams.find(
                      (item) => item.channel?.id === binding.channelId,
                    )?.channel?.name
                  : [
                      executors.data?.wakers.find(
                        (item) => item.id === binding.wakerId,
                      )?.name,
                      runtimeDisplayName({ id: binding.options?.runtimeId }),
                      binding.options?.model,
                    ]
                      .filter(Boolean)
                      .join(" · "),
              ),
              connection.errorCode
                ? t("errors." + connection.errorCode, {
                    defaultValue: t("needsAttention"),
                  })
                : undefined,
            ]
              .filter(Boolean)
              .join("\n")}
            aria-label={t("viewConnection", { name: connection.name })}
            className="focus-visible:ring-ring absolute inset-0 z-10 rounded-t-xl outline-none focus-visible:ring-2 focus-visible:ring-inset"
          />
          <div className="flex h-full min-h-0 flex-col px-5 pt-5 pb-1">
            <div className="h-avatar-md grid shrink-0 grid-cols-[40px_minmax(0,1fr)_auto] items-start gap-4">
              <PlatformIcon platform={connection.platform} />
              <div className="min-w-0">
                <div className="flex h-5 items-center gap-2">
                  <h2
                    className="type-label min-w-0 truncate font-medium"
                    title={connection.name}
                  >
                    {connection.name}
                  </h2>
                  {targets.length > 1 ? (
                    <Badge variant="secondary">
                      {t("responseConfigurationCount", {
                        count: targets.length,
                      })}
                    </Badge>
                  ) : null}
                  {pendingAuthorization ? (
                    <Badge asChild variant="warning" className="relative z-20">
                      <Link
                        to={`/im/${connection.id}`}
                        aria-label={t("pendingAuthorization")}
                      >
                        {t("pendingAuthorization")}
                      </Link>
                    </Badge>
                  ) : null}
                </div>
                <p
                  className="text-content-secondary type-caption mt-1 truncate"
                  title={connection.appId}
                >
                  {connection.appId || t("notConnected")}
                </p>
              </div>
              {onSelectedChange ? (
                <Checkbox
                  className="relative z-20"
                  size="sm"
                  checked={selected}
                  aria-label={t("selectConnection", { name: connection.name })}
                  onCheckedChange={(checked) =>
                    onSelectedChange(checked === true)
                  }
                />
              ) : null}
            </div>
            <div className="text-content-secondary type-ui mt-1 grid h-15 min-w-0 shrink-0 grid-cols-[minmax(0,1fr)_minmax(0,1.5fr)] grid-rows-2 items-center gap-x-3">
              {primaryBinding ? (
                <>
                  <span
                    className="h-avatar-xs grid min-w-0 grid-cols-[var(--spacing-avatar-xs)_minmax(0,1fr)] items-center gap-1"
                    title={targetName}
                  >
                    <span
                      className="size-avatar-xs grid place-items-center"
                      aria-hidden="true"
                    >
                      {team ? (
                        <TeamAvatarStack
                          animated={false}
                          members={team.members || []}
                          size="xs"
                        />
                      ) : (
                        <Avatar
                          name={targetName}
                          src={waker?.avatar}
                          size="xs"
                          interactive={false}
                          animated={false}
                        />
                      )}
                    </span>
                    <span className="min-w-0 truncate">{targetName}</span>
                  </span>
                  {primaryBinding.channelId ? (
                    <span className="h-avatar-xs grid min-w-0 grid-cols-[var(--spacing-avatar-xs)_minmax(0,1fr)] items-center gap-1">
                      <span
                        className="size-avatar-xs grid place-items-center"
                        aria-hidden="true"
                      >
                        <Users className="size-icon-sm" />
                      </span>
                      <span className="truncate">
                        {team
                          ? t("cardTeamMemberCount", {
                              count: team.members?.length ?? 0,
                            })
                          : t("loading")}
                      </span>
                    </span>
                  ) : (
                    <ExecutionSummary
                      runtimeId={primaryBinding.options?.runtimeId}
                      model={primaryBinding.options?.model}
                    />
                  )}
                </>
              ) : (
                <span className="h-avatar-xs col-span-2 flex min-w-0 items-center">
                  <span className="truncate">
                    {t(
                      bindings.length
                        ? "awaitingAuthorization"
                        : connection.appId
                          ? "waitingForMessage"
                          : "connectToReceive",
                    )}
                  </span>
                </span>
              )}
              {errorMessage ? (
                <p
                  role="alert"
                  className="text-destructive col-span-2 flex h-5 min-w-0 items-center gap-1"
                >
                  <span
                    className="w-avatar-xs grid shrink-0 place-items-center"
                    aria-hidden="true"
                  >
                    <TriangleAlert className="size-icon-sm" />
                  </span>
                  <span className="truncate">{errorMessage}</span>
                </p>
              ) : primaryBinding ? (
                <>
                  <span
                    className="grid h-5 min-w-0 grid-cols-[var(--spacing-avatar-xs)_minmax(0,1fr)] items-center gap-1"
                    title={t("cardScope", { scope })}
                  >
                    <span
                      className="w-avatar-xs grid place-items-center"
                      aria-hidden="true"
                    >
                      <FolderOpen className="size-icon-sm" />
                    </span>
                    <span className="truncate">{scope}</span>
                  </span>
                  <span
                    className="grid h-5 min-w-0 grid-cols-[var(--spacing-avatar-xs)_minmax(0,1fr)] items-center gap-1"
                    title={
                      primaryBinding.channelId
                        ? t("teamDefaults")
                        : t("permission")
                    }
                  >
                    <span
                      className="w-avatar-xs grid place-items-center"
                      aria-hidden="true"
                    >
                      <ShieldCheck className="size-icon-sm" />
                    </span>
                    <span className="truncate">{permission}</span>
                  </span>
                </>
              ) : null}
            </div>
          </div>
        </div>
        <CardFooter className="h-row-navigation shrink-0 gap-3">
          <ConnectionStatus connection={connection} appearance="inline" />
          <span className="text-content-secondary type-caption flex min-w-0 flex-1 items-center gap-1">
            <MessageCircle className="size-icon-xs shrink-0" />
            <span className="truncate">
              {t("cardConversationCount", {
                count: connection.conversationCount || 0,
              })}
            </span>
          </span>
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button
                variant="ghost"
                size="icon"
                aria-label={t("connectionActions", { name: connection.name })}
              >
                <Ellipsis />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem onSelect={() => setEditing(true)}>
                <Pencil />
                {t("editConnection")}
              </DropdownMenuItem>
              <DropdownMenuItem
                className="text-destructive"
                onSelect={() => setDeleting(true)}
              >
                <Trash2 />
                {t("delete")}
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </CardFooter>
        {editing ? (
          <ConnectionDialog
            connection={connection}
            onClose={() => setEditing(false)}
            onSaved={onSaved}
          />
        ) : null}
        {deleting ? (
          <ConnectionDeleteDialog
            connection={connection}
            onClose={() => setDeleting(false)}
            onDeleted={onSaved}
          />
        ) : null}
      </article>
    </Card>
  );
}

function ExecutionSummary({
  runtimeId,
  model,
}: {
  runtimeId?: string;
  model?: string;
}) {
  const { t } = useTranslation("im");
  const runtimeName =
    runtimeDisplayName({ id: runtimeId }) || t("notConfigured");
  return (
    <span
      className="h-avatar-xs flex min-w-0 items-center gap-1"
      title={`${runtimeName} · ${model || t("managed")}`}
    >
      <span
        className="size-avatar-xs grid shrink-0 place-items-center"
        aria-hidden="true"
      >
        <RuntimeCliIcon runtimeId={runtimeId} size={14} />
      </span>
      <span className="shrink-0">{runtimeName}</span>
      <span aria-hidden="true">·</span>
      <span className="min-w-0 truncate">{model || t("managed")}</span>
    </span>
  );
}

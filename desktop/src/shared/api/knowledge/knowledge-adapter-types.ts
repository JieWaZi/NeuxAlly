import type * as api from "@/shared/api/generated/sdk.gen";
import type { GetRemoteCapabilitiesResponse } from "@/shared/api/generated/types.gen";
import { ApiError } from "@/shared/api/errors";
import { withLocalErrorSurface } from "@/shared/api/local-error-surface";
import { i18n } from "@/shared/i18n";
import { knowledgeZh } from "@/shared/i18n/knowledge";
export type ResourceRef = {
  backend: string;
  connectionId: string;
  knowledgeBaseId: string;
};

export type KnowledgeOptions<F extends (...args: never[]) => unknown> =
  NonNullable<Parameters<F>[0]>;
export type KnowledgeCapabilities = {
  operations: Record<string, boolean>;
  details: Record<
    string,
    { supported: boolean; permission: string; condition: string }
  >;
  profiles: NonNullable<GetRemoteCapabilitiesResponse["profiles"]>;
};
export type KnowledgePageReadOptions = Omit<
  KnowledgeOptions<typeof api.getPage>,
  "query"
> & {
  query?: { slug?: string };
};
export type KnowledgeRevisionsOptions = Omit<
  KnowledgeOptions<typeof api.listRevisions>,
  "query"
> & {
  query?: { offset?: number; limit?: number; slug?: string };
};
export type KnowledgeOrganizeOptions =
  | KnowledgeOptions<typeof api.organize>
  | KnowledgeOptions<typeof api.organizeRemote>;

/** 每次调用都以实例身份覆盖路径，不能信任调用方带入的连接和库。 */
export function scopedKnowledgeOptions<T extends { path?: object }>(
  ref: ResourceRef,
  options: T,
) {
  return withLocalErrorSurface({
    ...options,
    path: {
      ...options.path,
      libraryId: ref.knowledgeBaseId,
      connectionId: ref.connectionId,
    },
    throwOnError: true as const,
  });
}

export function knowledgeUnsupported(): never {
  throw new ApiError(
    i18n.t("knowledge:remoteUnsupported", {
      defaultValue: knowledgeZh.remoteUnsupported,
    }),
    "KnowledgeUnsupported",
    501,
  );
}
export function knowledgeDirectoryIncomplete(): never {
  throw new ApiError(
    i18n.t("knowledge:incompleteDirectory", {
      defaultValue: knowledgeZh.incompleteDirectory,
    }),
    "KnowledgeIncompleteDirectory",
    502,
  );
}
export function assertKnowledgeCapability(
  capabilities: KnowledgeCapabilities,
  key: string,
) {
  const status = capabilities.details[key];
  if (!status?.supported) knowledgeUnsupported();
  if (status.permission !== "allowed")
    throw new ApiError(
      i18n.t("knowledge:permissionDenied", {
        defaultValue: knowledgeZh.permissionDenied,
      }),
      "KnowledgeForbidden",
      403,
    );
  if (status.condition !== "ready" || !capabilities.operations[key])
    throw new ApiError(
      i18n.t("knowledge:conditionUnavailable", {
        defaultValue: knowledgeZh.conditionUnavailable,
      }),
      "KnowledgeConditionUnavailable",
      409,
    );
}

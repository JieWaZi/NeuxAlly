import { KnowledgeLibrarySettings } from "./knowledge-library-settings";
import { CreateLibrary } from "./knowledge-library-dialogs";
import { lazy, Suspense, useEffect, useState, type ReactNode } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useNavigate, useParams, useSearchParams } from "react-router-dom";
import type { KnowledgeWikiCommands } from "./knowledge-wiki-command";
import { knowledgeAdapter } from "@/shared/api/knowledge";
import {
  localApiErrorMessage,
  withLocalErrorSurface,
} from "@/shared/api/local-error-surface";
import { useQueryEvents } from "@/shared/api/use-query-events";
import { useRouteSceneActive } from "@/shared/workspace/app-scene-workspace";
import { DetailDockLayout } from "@/shared/ui/detail-dock-layout";
import { ResourcePageSurface } from "@/shared/ui/resource-page";
import { Pagination } from "@/shared/ui/pagination";
import { Button } from "@/shared/ui/button";
import {
  createKnowledgeEditorController,
  type KnowledgeEditorController,
} from "./knowledge-editor-controller";
import { buildExecutionDefaults } from "./knowledge-execution-config";
import {
  createKnowledgeUploadQueue,
  type KnowledgeUploadQueue,
} from "./knowledge-upload-queue";
import { KnowledgeUploadDialog as UploadSources } from "./knowledge-upload-dialog";
import {
  type KnowledgeOrganizeIntent,
  createKnowledgeOrganizeIntent,
  KnowledgeOrganizeDialog,
} from "./knowledge-organize-dialog";
import { KnowledgeLibraryActions } from "./knowledge-library-actions";
import { KnowledgeDetailDock } from "./knowledge-detail-dock";
import {
  KnowledgeGraphLoading,
  KnowledgeWikiLoading,
} from "./knowledge-loading";
import { RawView } from "./knowledge-raw-view";
import { LibraryList } from "./knowledge-library";
import type { createLibraryDeleter } from "./delete-library";
import { useKnowledgeWrites } from "./knowledge-writes";
import { useKnowledgeWorkspaceQueries } from "./knowledge-workspace-queries";
import { knowledgeKeys, toLibrary, toSource } from "./knowledge-queries";
import { KNOWLEDGE_SOURCE_PAGE_SIZE } from "@/shared/api/knowledge";
import {
  readRoute,
  isOrganizableSourceState,
  type Source,
  type Route,
  type KnowledgeReadState,
} from "./knowledge-model";
import { isKnowledgeJobBusy, resourceKey } from "./knowledge-transfer";
import { useKnowledgeSourcePaginationLabels } from "./use-knowledge-source-pagination";
import { useCopy } from "./use-copy";

const KnowledgeWikiView = lazy(() =>
  import("./knowledge-wiki-view").then((module) => ({
    default: module.KnowledgeWikiView,
  })),
);

const KnowledgeGraphView = lazy(() =>
  import("./knowledge-graph-view").then((module) => ({
    default: module.KnowledgeGraphView,
  })),
);

function reading(query: {
  isPending: boolean;
  isFetching: boolean;
  error: unknown;
  data: unknown;
  refetch: () => Promise<unknown>;
}): KnowledgeReadState {
  return {
    pending: query.isPending,
    error: query.error,
    refreshing: query.isFetching && !query.isPending,
    stale: query.data !== undefined && Boolean(query.error),
    retry: () => void query.refetch(),
  };
}
/** 库详情装配文档、Wiki 与图谱，各视图按可见范围读取。 */
export function KnowledgePage() {
  const t = useCopy();
  const paginationLabels = useKnowledgeSourcePaginationLabels();
  const [params] = useSearchParams();
  const path = useParams();
  const navigate = useNavigate();
  const active = useRouteSceneActive();
  const client = useQueryClient();
  const [deleteLibrary] = useState(() => {
    let deleter: Promise<ReturnType<typeof createLibraryDeleter>> | undefined;
    return async (
      ...args: Parameters<ReturnType<typeof createLibraryDeleter>>
    ) => {
      deleter ??= import("./delete-library")
        .then((module) => module.createLibraryDeleter())
        .catch((cause: unknown) => {
          deleter = undefined;
          throw cause;
        });
      return (await deleter)(...args);
    };
  });
  const route = readRoute(params);
  if (path.libraryId) {
    route.libraryId = path.libraryId;
    route.view =
      params.get("view") === "graph"
        ? "graph"
        : params.get("view") === "wiki"
          ? "wiki"
          : "raw";
  }
  const detail = route.view !== "library" && Boolean(route.libraryId);
  const ref = {
    backend: detail ? params.get("backend") || "local" : "local",
    connectionId: detail ? params.get("connection") || "local" : "local",
    knowledgeBaseId: route.libraryId,
  };
  const identity = resourceKey(ref);
  const root = knowledgeKeys.resource(ref);
  const location =
    params.get("location") === "local"
      ? "local"
      : params.get("location") === "remote"
        ? "remote"
        : "all";
  const listOffset = Math.max(0, Number(params.get("librariesOffset")) || 0);
  const requestedSourcePageSize = Number(params.get("sourcesPageSize"));
  const sourcePageSize = [10, 30, 50].includes(requestedSourcePageSize)
    ? requestedSourcePageSize
    : KNOWLEDGE_SOURCE_PAGE_SIZE;
  const sourceOffset = Math.max(0, Number(params.get("sourcesOffset")) || 0);
  const { libraries, libraryQuery, sources, jobs, activeJob } =
    useKnowledgeWorkspaceQueries({
      ref,
      detail,
      documentsVisible: route.view === "raw",
      active,
      listOffset,
      sourceOffset,
      sourcePageSize,
      sourceFilters: {
        query: route.sourceQuery,
        state: route.sourceState,
        sort: route.sourceSort,
      },
      location,
    });
  useEffect(() => {
    if (route.view !== "raw" || !sources.isSuccess || sourceOffset === 0)
      return;
    const total = sources.data?.total ?? 0;
    const lastOffset = Math.max(
      0,
      (Math.ceil(total / sourcePageSize) - 1) * sourcePageSize,
    );
    if (sourceOffset <= lastOffset) return;
    const query = new URLSearchParams(params);
    query.set("sourcesOffset", String(lastOffset));
    navigate(`?${query}`, { replace: true });
  }, [
    route.view,
    sources.isSuccess,
    sources.data?.total,
    sourceOffset,
    sourcePageSize,
    params,
    navigate,
  ]);
  const library = libraryQuery.data
    ? {
        ...toLibrary(libraryQuery.data),
        sources: (sources.data?.items ?? []).map((source) =>
          toSource(source, ref.backend),
        ),
        sourceTotal: sources.data?.total,
      }
    : undefined;
  const busy = Boolean(
    activeJob && isKnowledgeJobBusy(activeJob.state ?? "unknown"),
  );
  const [selected, setSelected] = useState<{
    scope: string;
    sources: Source[];
  }>();
  const selection = selected?.scope === identity ? selected.sources : [];
  const rows = library?.sources ?? [];
  const batch = (selection.length ? selection : rows)
    .filter((source) => isOrganizableSourceState(source.state))
    .map((source) => source.id);
  const writes = useKnowledgeWrites(ref);
  const [modal, setModal] = useState<{
    kind: "create" | "upload" | "organize" | "rebuild" | "settings";
    scope: string;
    batch?: string[];
  }>();
  const [organizeIntents] = useState(
    () => new Map<string, KnowledgeOrganizeIntent>(),
  );
  const [createLoaded, setCreateLoaded] = useState(false);
  const [maintenance, setMaintenance] = useState<ReactNode>();
  const [wikiCommands] = useState<KnowledgeWikiCommands>(() => new Map());
  const [editorControllers] = useState(
    () => new Map<string, KnowledgeEditorController>(),
  );
  const [uploadQueues] = useState(
    () => new Map<string, KnowledgeUploadQueue>(),
  );
  const currentModal = modal?.scope === identity ? modal : undefined;
  useQueryEvents({
    topic: "knowledge",
    resource: detail
      ? {
          kind: "knowledge_library",
          id: route.libraryId,
          parentId: ref.connectionId,
        }
      : { kind: "global" },
    queryKeys: detail ? [root] : [knowledgeKeys.libraryLists],
    enabled: active,
    active,
  });
  const legacy =
    [
      ...(route.view === "wiki" ? [] : ["page"]),
      "pageSlug",
      "source",
      "pageSearch",
      "themeId",
      "pageType",
      "pagesOffset",
      "historyOffset",
    ].some((key) => params.has(key)) ||
    (params.has("view") &&
      ![
        detail ? "raw" : "library",
        ...(detail ? ["wiki", "graph"] : []),
      ].includes(params.get("view") ?? ""));
  useEffect(() => {
    if (!legacy) return;
    const query = new URLSearchParams(params);
    for (const key of [
      "view",
      "library",
      "page",
      "pageSlug",
      "source",
      "pageSearch",
      "themeId",
      "pageType",
      "pagesOffset",
      "historyOffset",
    ])
      query.delete(key);
    navigate(
      detail
        ? `/knowledge/${encodeURIComponent(route.libraryId)}?${query}`
        : "/knowledge",
      { replace: true },
    );
  }, [legacy, params, navigate, detail, route.libraryId]);
  function changeQuery(patch: Record<string, string>) {
    const query = new URLSearchParams(params);
    for (const [key, value] of Object.entries(patch)) query.set(key, value);
    navigate(`?${query}`);
  }
  function go(patch: Partial<Route>) {
    changeQuery(
      Object.fromEntries(
        Object.entries({ ...patch, sourcesOffset: "0" }).filter(
          (entry): entry is [string, string] => typeof entry[1] === "string",
        ),
      ),
    );
  }
  const [actionError, setActionError] = useState<{
    scope: string;
    message: string;
  }>();
  async function openAction(
    capability: string,
    callback: () => void | Promise<void>,
  ) {
    setActionError(undefined);
    try {
      if (!knowledgeAdapter(ref).localConfiguration)
        await knowledgeAdapter(ref).assertCapability(capability);
      await callback();
    } catch (cause) {
      setActionError({
        scope: identity,
        message: localApiErrorMessage(cause, t("knowledgeOperationFailed")),
      });
    }
  }
  function openUpload() {
    if (!uploadQueues.has(identity))
      uploadQueues.set(identity, createKnowledgeUploadQueue());
    setModal({ kind: "upload", scope: identity });
  }
  const retainedOrganize = organizeIntents.get(identity)?.get();
  const recoveringOrganize = Boolean(
    retainedOrganize?.start || retainedOrganize?.remoteCommand,
  );
  function openOrganize() {
    if (!organizeIntents.has(identity))
      organizeIntents.set(identity, createKnowledgeOrganizeIntent());
    setModal({ kind: "organize", scope: identity, batch: [...batch] });
  }
  const libraryActions = (
    <KnowledgeLibraryActions
      library={library}
      busy={busy}
      onUpload={() => void openAction("rawOnly", openUpload)}
      onSettings={() =>
        void openAction("settings", () =>
          setModal({ kind: "settings", scope: identity }),
        )
      }
      organizeDisabled={
        !library ||
        (!recoveringOrganize &&
          (busy || jobs.isLoading || Boolean(jobs.error) || sources.isFetching))
      }
      onOrganize={() => {
        // 原冻结请求通过 Adapter 对账；当前能力和任务状态只约束新接纳。
        if (recoveringOrganize) openOrganize();
        else void openAction("organize", openOrganize);
      }}
    />
  );
  const documents = (
    <>
      {actionError?.scope === identity && (
        <p role="alert" className="type-body text-destructive">
          {actionError.message}
        </p>
      )}
      {route.view === "raw" && jobs.error && (
        <div role="alert" className="flex flex-wrap items-center gap-3">
          <p className="type-body text-destructive">
            {t("organizeAvailabilityFailed")}
          </p>
          <Button
            size="sm"
            variant="secondary"
            onClick={() => void jobs.refetch()}
          >
            {t("retry")}
          </Button>
        </div>
      )}
      {route.view === "wiki" ? (
        <Suspense fallback={<KnowledgeWikiLoading />}>
          <KnowledgeWikiView
            key={identity}
            resource={ref}
            commands={wikiCommands}
            onMaintenance={(page) =>
              void openAction("maintenanceChat", async () => {
                const module =
                  await import("./knowledge-wiki-maintenance-chat");
                setMaintenance(module.renderWikiMaintenance(ref, page));
              })
            }
            pageId={params.get("page") ?? ""}
            onPage={(id) => {
              const query = new URLSearchParams(params);
              if (id) query.set("page", id);
              else query.delete("page");
              query.set("view", "wiki");
              navigate(`?${query}`);
            }}
            writable={Boolean(library)}
            structureVersion={libraryQuery.data?.structureVersion ?? 0}
          />
        </Suspense>
      ) : route.view === "graph" ? (
        <Suspense fallback={<KnowledgeGraphLoading />}>
          <KnowledgeGraphView
            key={identity}
            resource={ref}
            center={params.get("graphCenter") ?? ""}
            onCenter={(slug) => {
              const query = new URLSearchParams(params);
              if (slug) query.set("graphCenter", slug);
              else query.delete("graphCenter");
              navigate(`?${query}`);
            }}
          />
        </Suspense>
      ) : (
        <RawView
          key={identity}
          library={library}
          actions={libraryActions}
          route={route}
          go={go}
          rows={rows}
          selection={selection}
          setSelection={(update) =>
            setSelected((current) => ({
              scope: identity,
              sources: update(
                current?.scope === identity ? current.sources : [],
              ),
            }))
          }
          upload={() => void openAction("rawOnly", openUpload)}
          busy={busy}
          readState={{
            ...reading(sources),
            pending: libraryQuery.isPending || sources.isPending,
            error: libraryQuery.error || sources.error,
            retry: () => {
              void libraryQuery.refetch();
              void sources.refetch();
            },
          }}
          onRebuild={(source) => {
            const chosen = [source.id];
            const key = `${identity}:rebuild:${JSON.stringify(chosen)}`;
            if (!organizeIntents.has(key))
              organizeIntents.set(key, createKnowledgeOrganizeIntent());
            setModal({ kind: "rebuild", scope: identity, batch: chosen });
          }}
          sourceOffset={sourceOffset}
          sourcePageSize={sourcePageSize}
          onSourcePageSizeChange={(size) =>
            changeQuery({ sourcesOffset: "0", sourcesPageSize: String(size) })
          }
          sourceTotal={sources.data?.total ?? 0}
          sourcePagePending={sources.isFetching}
          onSourcePageChange={(page) =>
            changeQuery({
              sourcesOffset: String((page - 1) * sourcePageSize),
            })
          }
        />
      )}
    </>
  );
  return (
    <>
      {maintenance}
      {detail ? (
        <DetailDockLayout
          navigation={
            <KnowledgeDetailDock
              view={
                route.view === "wiki"
                  ? "wiki"
                  : route.view === "graph"
                    ? "graph"
                    : "raw"
              }
              onChange={(view) => {
                const query = new URLSearchParams(params);
                query.set("view", view);
                if (view !== "wiki") query.delete("page");
                navigate(`?${query}`);
              }}
            />
          }
        >
          {route.view === "graph" ? (
            <div className="flex h-full min-h-0 flex-col">{documents}</div>
          ) : (
            <ResourcePageSurface
              contentLayout={route.view === "raw" ? "scroll" : "pane"}
              contentClassName="flex flex-col gap-5"
            >
              {documents}
            </ResourcePageSurface>
          )}
        </DetailDockLayout>
      ) : (
        <>
          <LibraryList
            libraries={(libraries.data?.items ?? []).map(toLibrary)}
            state={
              libraries.error
                ? "error"
                : libraries.isPending
                  ? "loading"
                  : undefined
            }
            error={libraries.error}
            onRetry={() => void libraries.refetch()}
            location={location}
            onLocationChange={(value) =>
              changeQuery({ location: value, librariesOffset: "0" })
            }
            total={libraries.data?.total}
            open={(value) =>
              navigate(
                `/knowledge/${encodeURIComponent(value.id)}?${new URLSearchParams({ backend: value.ref.backend, connection: value.ref.connectionId, title: value.name })}`,
              )
            }
            create={() => {
              if (!editorControllers.get("create")?.getSnapshot().command)
                editorControllers.set(
                  "create",
                  createKnowledgeEditorController(),
                );
              setCreateLoaded(true);
              setModal({ kind: "create", scope: identity });
            }}
            canCreate
            onDelete={async (value, requestId) => {
              await deleteLibrary(value, requestId);
              await client.invalidateQueries({
                queryKey: knowledgeKeys.libraryLists,
              });
            }}
          />
          {(libraries.data?.total ?? 0) > 50 && (
            <Pagination
              labels={paginationLabels}
              page={Math.floor(listOffset / 50) + 1}
              pageSize={50}
              totalItems={libraries.data?.total ?? 0}
              onPageChange={(page) =>
                changeQuery({ librariesOffset: String((page - 1) * 50) })
              }
            />
          )}
        </>
      )}
      {createLoaded && (
        <CreateLibrary
          opened={currentModal?.kind === "create"}
          controller={editorControllers.get("create")}
          close={() => {
            if (!editorControllers.get("create")?.getSnapshot().command)
              editorControllers.delete("create");
            setModal(undefined);
          }}
          create={async (value, execution, requestId, controller) => {
            const { buildCreateLibraryBody } =
              await import("./knowledge-library-write-body");
            const response = await knowledgeAdapter(value.ref).createLibrary(
              withLocalErrorSurface({
                body: {
                  ...buildCreateLibraryBody(requestId, value),
                  ...(knowledgeAdapter(value.ref).localConfiguration
                    ? { executionDefaults: buildExecutionDefaults(execution) }
                    : {}),
                  ...(knowledgeAdapter(value.ref).localConfiguration &&
                  controller.getSnapshot().draft.embedding.baseUrl
                    ? {
                        embeddingProfile: {
                          baseUrl:
                            controller.getSnapshot().draft.embedding.baseUrl,
                          model: controller.getSnapshot().draft.embedding.model,
                          apiKey:
                            controller.getSnapshot().draft.embedding.apiKey,
                        },
                      }
                    : {}),
                },
                throwOnError: true,
              }),
            );
            const id =
              response.data.resourceId ?? response.data.libraryId ?? "";
            if (!id) throw new Error(t("createLibraryFailed"));
            controller.acknowledgeLibrary({
              ...value,
              id,
              revision: response.data.version ?? value.revision,
              ref: { ...value.ref, knowledgeBaseId: id },
            });
            if (knowledgeAdapter(value.ref).localConfiguration)
              controller.acknowledgeExecution(execution, 1);
            const embedding = controller.getSnapshot().draft.embedding;
            if (embedding.baseUrl)
              controller.acknowledgeEmbedding(
                { ...embedding, hasApiKey: true },
                1,
                response.data.version ?? value.revision,
                controller.getSnapshot().command?.embeddingRequestId ?? "",
              );
            await client.invalidateQueries({
              queryKey: knowledgeKeys.libraryLists,
            });
            setModal(undefined);
            navigate(
              `/knowledge/${encodeURIComponent(id)}?${new URLSearchParams({ title: value.name, ...(value.ref.backend !== "local" ? { backend: value.ref.backend, connection: value.ref.connectionId } : {}) })}`,
            );
          }}
        />
      )}
      {library && currentModal?.kind === "settings" && (
        <KnowledgeLibrarySettings
          library={library}
          controllers={editorControllers}
          close={() => setModal(undefined)}
        />
      )}
      {library && currentModal?.kind === "upload" && (
        <UploadSources
          queue={uploadQueues.get(identity)}
          opened
          close={() => setModal(undefined)}
          upload={async (file, requestId, signal) => {
            const { uploadKnowledgeSource } =
              await import("./knowledge-upload-source");
            const receipt = await uploadKnowledgeSource(
              ref,
              file,
              requestId,
              signal,
            );
            await Promise.all([
              client.invalidateQueries({ queryKey: [...root, "sources"] }),
              client.invalidateQueries({ queryKey: [...root, "detail"] }),
            ]);
            return receipt;
          }}
        />
      )}

      {library &&
        (currentModal?.kind === "organize" ||
          currentModal?.kind === "rebuild") && (
          <KnowledgeOrganizeDialog
            library={library}
            rebuild={currentModal.kind === "rebuild"}
            batch={currentModal.batch ?? []}
            operation={writes.organize}
            intent={organizeIntents.get(
              currentModal.kind === "rebuild"
                ? `${identity}:rebuild:${JSON.stringify(currentModal.batch ?? [])}`
                : identity,
            )}
            onClose={() => {
              const key =
                currentModal.kind === "rebuild"
                  ? `${identity}:rebuild:${JSON.stringify(currentModal.batch ?? [])}`
                  : identity;
              // 普通取消舍弃草稿；已提交请求保留原范围与要求，供待定结果核验。
              if (
                !organizeIntents.get(key)?.get().start &&
                !organizeIntents.get(key)?.get().remoteCommand
              )
                organizeIntents.delete(key);
              setModal(undefined);
            }}
            onAccepted={() => {
              organizeIntents.delete(
                currentModal.kind === "rebuild"
                  ? `${identity}:rebuild:${JSON.stringify(currentModal.batch ?? [])}`
                  : identity,
              );
              setModal((current) =>
                (current?.kind === "organize" || current?.kind === "rebuild") &&
                current.kind === currentModal.kind &&
                current.scope === identity &&
                JSON.stringify(current.batch ?? []) ===
                  JSON.stringify(currentModal.batch ?? [])
                  ? undefined
                  : current,
              );
            }}
          />
        )}
    </>
  );
}

import { KnowledgeLoading } from "./knowledge-loading";
import { QueryState } from "@/shared/ui/query-state";
import { Button } from "@/shared/ui/button";
import { localApiErrorMessage } from "@/shared/api/local-error-surface";
import { lazy, Suspense, useState } from "react";
import { useRouteSceneActive } from "@/shared/workspace/app-scene-workspace";
import { BookOpen, Plus } from "lucide-react";
import { ResourceFilterSelect } from "@/shared/ui/resource-filter-select";
import {
  PageAction,
  ResourceCardGrid,
  ResourcePageHeader,
  ResourcePageSearchField,
  ResourcePageSurface,
  ResourcePageToolbar,
} from "@/shared/ui/resource-page";
import { useCopy } from "./use-copy";
import { type Library } from "./knowledge-model";
import { Empty } from "./knowledge-controls";
import { KnowledgeLibraryCard } from "./knowledge-library-card";

const KnowledgeLibraryDeleteDialog = lazy(() =>
  import("./knowledge-library-delete-dialog").then((module) => ({
    default: module.KnowledgeLibraryDeleteDialog,
  })),
);

export function LibraryList({
  libraries,
  open,
  create,
  canCreate = true,
  state,
  error,
  onRetry,
  location = "all",
  onLocationChange,
  total,
  onDelete,
  canDelete = () => true,
}: {
  libraries: Library[];
  onDelete?: (library: Library, requestId: string) => Promise<void>;
  canDelete?: (library: Library) => boolean;
  state?: "loading" | "error";
  error?: unknown;
  onRetry?: () => void;
  open: (library: Library) => void;
  location?: "all" | "local" | "remote";
  onLocationChange?: (location: "all" | "local" | "remote") => void;
  total?: number;
  create: () => void;
  canCreate?: boolean;
}) {
  const t = useCopy();
  const active = useRouteSceneActive();
  const [deleting, setDeleting] = useState<Library | null>(null);
  const [search, setSearch] = useState("");
  const rows = libraries.filter((library) =>
    `${library.name} ${library.description}`
      .toLowerCase()
      .includes(search.toLowerCase()),
  );
  return (
    <ResourcePageSurface contentClassName="pt-6">
      <ResourcePageHeader
        title={t("knowledge")}
        description={t("intro")}
        actions={
          <>
            {canCreate && (
              <PageAction intent="primary" label={t("create")} onClick={create}>
                <Plus />
              </PageAction>
            )}
          </>
        }
      />
      <ResourcePageToolbar
        filters={
          <>
            <ResourceFilterSelect
              ariaLabel={t("storageLocation")}
              value={location}
              onValueChange={(value) => {
                if (value === "all" || value === "local" || value === "remote")
                  onLocationChange?.(value);
              }}
              options={[
                { value: "all", label: t("all") },
                { value: "local", label: t("local") },
                { value: "remote", label: t("remote") },
              ]}
            />
            <ResourcePageSearchField
              aria-label={t("librarySearch")}
              placeholder={t("librarySearch")}
              value={search}
              onChange={(event) => setSearch(event.target.value)}
            />
          </>
        }
      >
        <div className="flex items-center gap-3 whitespace-nowrap">
          <b>{t("libraries")}</b>
          <span className="type-caption text-muted-foreground">
            {t("libraryCount", {
              count: search ? rows.length : (total ?? rows.length),
            })}
          </span>
        </div>
      </ResourcePageToolbar>
      {state === "error" && libraries.length > 0 ? (
        <div className="mt-4 flex min-w-0 flex-wrap items-center gap-2">
          <p role="alert" className="type-body text-destructive wrap-anywhere">
            {localApiErrorMessage(error, t("knowledgeReadFailed"))}
          </p>
          {onRetry ? (
            <Button size="sm" variant="secondary" onClick={onRetry}>
              {t("retry")}
            </Button>
          ) : null}
        </div>
      ) : null}
      {state === "loading" && !libraries.length ? (
        <KnowledgeLoading />
      ) : state === "error" && !libraries.length ? (
        <QueryState
          error={error || new Error(t("knowledgeReadFailed"))}
          onRetry={() => onRetry?.()}
        />
      ) : rows.length ? (
        <ResourceCardGrid className="mt-6" aria-label={t("libraries")}>
          {rows.map((library) => (
            <KnowledgeLibraryCard
              key={JSON.stringify([
                library.ref.backend,
                library.ref.connectionId,
                library.ref.knowledgeBaseId,
              ])}
              library={library}
              open={open}
              onDelete={onDelete ? setDeleting : undefined}
              canDelete={canDelete(library)}
            />
          ))}
        </ResourceCardGrid>
      ) : (
        <Empty
          icon={BookOpen}
          title={t("emptyLibraries")}
          description={t("emptyLibrariesHelp")}
        />
      )}
      {deleting && onDelete ? (
        <Suspense fallback={null}>
          <KnowledgeLibraryDeleteDialog
            key={JSON.stringify([
              deleting.ref.backend,
              deleting.ref.connectionId,
              deleting.ref.knowledgeBaseId,
            ])}
            library={deleting}
            open={active}
            onOpenChange={(value) => {
              if (!value) setDeleting(null);
            }}
            onDelete={onDelete}
          />
        </Suspense>
      ) : null}
    </ResourcePageSurface>
  );
}

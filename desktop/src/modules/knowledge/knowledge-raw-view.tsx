import { lazy, Suspense, type ReactNode } from "react";
import { FileText, Upload } from "lucide-react";
import { Button } from "@/shared/ui/button";
import { ResourceFilterSelect } from "@/shared/ui/resource-filter-select";
import {
  ResourcePageToolbar,
  ResourcePageSearchField,
} from "@/shared/ui/resource-page";
import { KnowledgeReadFeedback } from "./knowledge-read-feedback";
import { KnowledgeDocumentsLoading } from "./knowledge-loading";
import { useCopy } from "./use-copy";
import { Empty } from "./knowledge-controls";
import {
  isOrganizableSourceState,
  type Library,
  type KnowledgeReadState,
  type Route,
  type Source,
} from "./knowledge-model";
import { useKnowledgeSourcePaginationLabels } from "./use-knowledge-source-pagination";
const KnowledgeDocumentTable = lazy(() =>
  import("./knowledge-document-table").then((module) => ({
    default: module.KnowledgeDocumentTable,
  })),
);

/** 选择按当前库保存；翻页和新上传不会隐式加入已选集合。 */
export function RawView({
  library,
  actions,
  route,
  go,
  rows,
  selection,
  setSelection,
  upload,
  busy,
  readState,
  sourceOffset,
  sourcePageSize,
  onSourcePageSizeChange,
  sourceTotal,
  sourcePagePending,
  onSourcePageChange,
  onRebuild,
}: {
  library?: Library;
  actions?: ReactNode;
  route: Route;
  go: (patch: Partial<Route>) => void;
  rows: Source[];
  selection: Source[];
  setSelection: (update: (current: Source[]) => Source[]) => void;
  upload: () => void;
  busy: boolean;
  readState: KnowledgeReadState;
  sourceOffset: number;
  sourcePageSize: number;
  onSourcePageSizeChange: (size: number) => void;
  sourceTotal: number;
  sourcePagePending: boolean;
  onSourcePageChange: (page: number) => void;
  onRebuild: (source: Source) => void;
}) {
  const t = useCopy();
  const paginationLabels = useKnowledgeSourcePaginationLabels();
  return (
    <div
      data-motion="presence"
      data-state="open"
      className="flex min-w-0 shrink-0 flex-col gap-4"
    >
      <ResourcePageToolbar
        filters={
          <>
            <ResourcePageSearchField
              aria-label={t("sourcePageSearch")}
              placeholder={t("sourcePageSearch")}
              value={route.sourceQuery}
              onChange={(event) => go({ sourceQuery: event.target.value })}
            />
            <ResourceFilterSelect
              ariaLabel={t("sourceFilter")}
              value={route.sourceState}
              onValueChange={(sourceState) => go({ sourceState })}
              options={(
                [
                  "all",
                  "pending",
                  "queued",
                  "processing",
                  "processed",
                  "unchanged",
                  "failed",
                  "cancelled",
                ] as const
              )
                .filter(
                  (value) =>
                    library?.ref.backend !== "weknora" ||
                    !["queued", "processed", "unchanged"].includes(value),
                )
                .map((value) => ({ value, label: t(value) }))}
            />
            <ResourceFilterSelect
              ariaLabel={t("sort")}
              value={route.sourceSort}
              onValueChange={(sourceSort) => go({ sourceSort })}
              options={[
                { value: "newest", label: t("newest") },
                { value: "oldest", label: t("oldest") },
              ]}
            />
            {actions}
          </>
        }
      />
      <KnowledgeReadFeedback
        state={readState}
        hasData={Boolean(library?.sources.length)}
        loading={<KnowledgeDocumentsLoading />}
      />
      {library &&
        !rows.length &&
        sourceTotal === 0 &&
        (route.sourceQuery || route.sourceState !== "all") &&
        !readState.pending &&
        !sourcePagePending &&
        !readState.error && (
          <p role="status" className="type-body text-content-secondary">
            {t("noMatchingSources")}
          </p>
        )}
      {library && !(readState.pending && !library.sources.length) && (
        <Suspense fallback={<KnowledgeDocumentsLoading />}>
          <KnowledgeDocumentTable
            library={library}
            emptyContent={
              !rows.length &&
              sourceTotal === 0 &&
              !route.sourceQuery &&
              route.sourceState === "all" &&
              !readState.error ? (
                <Empty
                  icon={FileText}
                  title={t("rawEmpty")}
                  description={t("rawEmptyHelp")}
                >
                  <Button disabled={!library || busy} onClick={upload}>
                    <Upload />
                    {t("upload")}
                  </Button>
                </Empty>
              ) : undefined
            }
            onRebuild={onRebuild}
            onDeleted={(source) =>
              setSelection((current) =>
                current.filter(
                  (entry) =>
                    entry.id !== source.id &&
                    (!source.sourceId || entry.sourceId !== source.sourceId),
                ),
              )
            }
            rows={rows}
            disabled={!library || busy || sourcePagePending}
            selection={selection.map((source) => source.id)}
            select={(source) =>
              setSelection((current) =>
                current.some((entry) => entry.id === source.id)
                  ? current.filter((entry) => entry.id !== source.id)
                  : [...current, source],
              )
            }
            selectPage={(checked) =>
              setSelection((current) =>
                checked
                  ? [
                      ...current.filter(
                        (entry) =>
                          !rows.some((source) => source.id === entry.id),
                      ),
                      ...rows.filter((source) =>
                        isOrganizableSourceState(source.state),
                      ),
                    ]
                  : current.filter(
                      (entry) => !rows.some((source) => source.id === entry.id),
                    ),
              )
            }
            pagination={{
              labels: paginationLabels,
              page: Math.floor(sourceOffset / sourcePageSize) + 1,
              pageSize: sourcePageSize,
              pageSizeOptions: [10, 30, 50],
              onPageSizeChange: onSourcePageSizeChange,
              totalItems: sourceTotal,
              pending: sourcePagePending,
              onPageChange: onSourcePageChange,
            }}
          />
        </Suspense>
      )}
    </div>
  );
}

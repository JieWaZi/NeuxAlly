import { useInfiniteQuery, useQueryClient } from "@tanstack/react-query";
import { Plus, Workflow } from "lucide-react";
import { useEffect } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate, useSearchParams } from "react-router-dom";
import { Button } from "@/shared/ui/button";
import { EmptyState } from "@/shared/ui/empty-state";
import {
  PageAction,
  PageActionLabel,
  ResourceCardGrid,
  ResourcePageHeader,
  ResourcePageLibraryLabel,
  ResourcePageSearchField,
  ResourcePageSurface,
  ResourcePageToolbar,
} from "@/shared/ui/resource-page";
import { WorkflowCard } from "../components/workflow-card";
import {
  WorkflowLoadingState,
  WorkflowQueryError,
} from "../components/query-feedback";
import { listWorkflowsQueryKey } from "../api/workflow-queries";
import { listWorkflowsInfiniteOptions } from "@/shared/api/generated/@tanstack/react-query.gen";
import { useRouteSceneActive } from "@/shared/workspace/app-scene-workspace";

export function WorkflowListPage() {
  const { t } = useTranslation("workflow");
  const navigate = useNavigate();
  const client = useQueryClient();
  const active = useRouteSceneActive();
  useEffect(() => {
    if (active)
      void client.invalidateQueries({ queryKey: listWorkflowsQueryKey() });
  }, [active, client]);
  const [params, setParams] = useSearchParams();
  const search = params.get("search") ?? "";
  const query = useInfiniteQuery({
    ...listWorkflowsInfiniteOptions({ query: { search, limit: 50 } }),
    enabled: active,
    initialPageParam: { query: { search, limit: 50 } },
    getNextPageParam: (page) =>
      page.nextBefore && page.nextBeforeId
        ? {
            query: {
              search,
              limit: 50,
              before: page.nextBefore,
              beforeId: page.nextBeforeId,
            },
          }
        : undefined,
  });
  const items = query.data?.pages.flatMap((page) => page.items ?? []) ?? [];
  const submit = () =>
    navigate(`/workflows/new?draftId=${crypto.randomUUID()}`);
  return (
    <ResourcePageSurface>
      <ResourcePageHeader
        title={t("title")}
        description={t("subtitle")}
        actions={
          <PageAction onClick={submit} aria-label={t("create")}>
            <Plus />
            <PageActionLabel>{t("create")}</PageActionLabel>
          </PageAction>
        }
      />
      <ResourcePageToolbar
        filters={
          <ResourcePageSearchField
            aria-label={t("search")}
            placeholder={t("search")}
            value={search}
            onChange={(event) =>
              setParams(
                event.target.value ? { search: event.target.value } : {},
                { replace: true },
              )
            }
          />
        }
      >
        <ResourcePageLibraryLabel
          installed={t("library")}
          count={t("count", { count: items.length })}
        />
      </ResourcePageToolbar>
      {query.isError ? (
        <WorkflowQueryError
          error={query.error}
          className="mt-6"
          retrying={query.isFetching}
          onRetry={() => void query.refetch()}
        />
      ) : null}
      {query.isPending ? (
        <WorkflowLoadingState className="mt-6" />
      ) : !query.data ? null : !items.length ? (
        <EmptyState
          className="mt-6"
          title={t("empty")}
          description={t("emptyHelp")}
          icon={Workflow}
        />
      ) : (
        <ResourceCardGrid className="mt-6">
          {items.map((definition) => (
            <WorkflowCard key={definition.id} definition={definition} />
          ))}
        </ResourceCardGrid>
      )}
      {query.hasNextPage ? (
        <Button
          variant="secondary"
          disabled={query.isFetchingNextPage}
          onClick={() => void query.fetchNextPage()}
        >
          {t("loadMore")}
        </Button>
      ) : null}
    </ResourcePageSurface>
  );
}

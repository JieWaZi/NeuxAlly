import { useInfiniteQuery, useQuery } from "@tanstack/react-query";
import { ChevronDown } from "lucide-react";
import type { ComponentProps } from "react";
import type { GithubComJieWaZiNeuxAllyServerApisKnowledgeWikiIndexGroup as IndexGroup } from "@/shared/api/generated/types.gen";
import { Button } from "@/shared/ui/button";
import { EmptyState } from "@/shared/ui/empty-state";
import { QueryState } from "@/shared/ui/query-state";
import { StaticMarkdown } from "@/shared/ui/static-markdown";
import { KnowledgeWikiContentLoading } from "./knowledge-loading";
import { KnowledgeReadFeedback } from "./knowledge-read-feedback";
import type { ResourceRef } from "./knowledge-transfer";
import { readWikiIndex, wikiIndexOptions } from "./knowledge-wiki-queries";
import { useWikiCopy } from "./use-wiki-copy";
import {
  remarkWikiIndexBreaks,
  normalizeWikiIndexIntro,
} from "./knowledge-wiki-markdown";

/** 复用上游结构化索引：导言只渲染一次，分组目录来自全库轻量分页。 */
export function KnowledgeWikiStructuredIndex({
  resource,
  active,
  onLink,
  remarkPlugins,
}: {
  resource: ResourceRef;
  active: boolean;
  onLink: (slug: string) => void;
  remarkPlugins: ComponentProps<typeof StaticMarkdown>["remarkPlugins"];
}) {
  const t = useWikiCopy();
  const indexRemarkPlugins = [...(remarkPlugins ?? []), remarkWikiIndexBreaks];
  const query = useQuery({ ...wikiIndexOptions(resource), enabled: active });
  if (query.isPending) return <KnowledgeWikiContentLoading />;
  if (!query.data)
    return (
      <QueryState error={query.error} onRetry={() => void query.refetch()} />
    );
  const groups = (query.data.groups ?? []).filter(
    (group) => (group.total ?? 0) > 0,
  );
  return (
    <div className="space-y-6">
      {query.error && (
        <KnowledgeReadFeedback
          hasData
          state={{
            pending: false,
            error: query.error,
            retry: () => {
              void query.refetch();
            },
          }}
        />
      )}
      <StaticMarkdown
        source={normalizeWikiIndexIntro(query.data.intro ?? "")}
        remarkPlugins={indexRemarkPlugins}
        onLink={onLink}
      />
      {groups.map((group) => (
        <KnowledgeWikiIndexGroup
          key={`${group.type}:${query.dataUpdatedAt}`}
          resource={resource}
          group={group}
          refreshedAt={query.dataUpdatedAt}
          onLink={onLink}
          remarkPlugins={indexRemarkPlugins}
        />
      ))}
      {!groups.length && !query.data.intro && (
        <EmptyState title={t("empty")} description={t("emptyHelp")} />
      )}
    </div>
  );
}

function KnowledgeWikiIndexGroup({
  resource,
  group,
  refreshedAt,
  onLink,
  remarkPlugins,
}: {
  resource: ResourceRef;
  group: IndexGroup;
  refreshedAt: number;
  onLink: (slug: string) => void;
  remarkPlugins: ComponentProps<typeof StaticMarkdown>["remarkPlugins"];
}) {
  const t = useWikiCopy();
  // 新首屏读取后重置该组追加页，避免把更新前后的目录窗口拼成一份假快照。
  const options = wikiIndexOptions(resource, group.type);
  const more = useInfiniteQuery({
    queryKey: [...options.queryKey, "more", refreshedAt],
    initialPageParam: group.nextCursor ?? "",
    queryFn: ({ signal, pageParam }) =>
      readWikiIndex(resource, group.type, pageParam, signal),
    getNextPageParam: (last) => last.groups?.[0]?.nextCursor || undefined,
    enabled: false,
  });
  const rows = [
    ...(group.items ?? []),
    ...(more.data?.pages.flatMap((page) => page.groups?.[0]?.items ?? []) ??
      []),
  ];
  const next = more.data ? more.hasNextPage : Boolean(group.nextCursor);
  const label =
    group.type === "summary"
      ? t("summaryType")
      : group.type === "entity"
        ? t("entity")
        : group.type === "concept"
          ? t("concept")
          : group.type === "synthesis"
            ? t("synthesis")
            : group.type === "comparison"
              ? t("comparison")
              : group.type === "topic"
                ? t("topic")
                : t("page");
  return (
    <section className="space-y-3" aria-label={label}>
      <StaticMarkdown
        source={`## ${label} (${group.total ?? 0})\n\n${appendIndexDirectoryLines(rows)}`}
        remarkPlugins={remarkPlugins}
        onLink={onLink}
      />
      {more.error && (
        <KnowledgeReadFeedback
          hasData
          state={{
            pending: false,
            error: more.error,
            retry: () => {
              void more.fetchNextPage();
            },
          }}
        />
      )}
      {next && (
        <div className="flex justify-center">
          <Button
            type="button"
            variant="text"
            disabled={more.isFetching}
            onClick={() => void more.fetchNextPage()}
          >
            <ChevronDown aria-hidden="true" />
            {t("loadMore")}
          </Button>
        </div>
      )}
    </section>
  );
}

/** 迁自WeKnora固定8776 WikiBrowser.vue appendIndexDirectoryLines，仅DTO字段改为camelCase。 */
function appendIndexDirectoryLines(
  items: NonNullable<IndexGroup["items"]>,
): string {
  let out = "";
  const emittedDirs = new Set<string>();
  for (const entry of items) {
    const path = (Array.isArray(entry.categoryPath) ? entry.categoryPath : [])
      .map((part) => String(part || "").trim())
      .filter(Boolean);
    for (let i = 0; i < path.length; i++) {
      const parts = path.slice(0, i + 1);
      const key = parts.join("/");
      if (emittedDirs.has(key)) continue;
      emittedDirs.add(key);
      out += `${"  ".repeat(i)}**${path[i]}**\n`;
    }
    const display = entry.title || entry.slug;
    const indent = "  ".repeat(path.length);
    if (entry.summary) {
      out += `${indent}[[${entry.slug}|${display}]] — ${entry.summary}\n`;
    } else {
      out += `${indent}[[${entry.slug}|${display}]]\n`;
    }
  }
  return out;
}

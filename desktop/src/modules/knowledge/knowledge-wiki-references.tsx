import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Button } from "@/shared/ui/button";
import { useRouteSceneActive } from "@/shared/workspace/app-scene-workspace";
import { KnowledgeResultDocument } from "./knowledge-result-document";
import { sourceDetailOptions } from "./knowledge-source-document";
import type { ResourceRef } from "./knowledge-transfer";
import type { WikiPage } from "./knowledge-wiki-queries";
import { useWikiCopy } from "./use-wiki-copy";

/** 与 Lite 阅读页一致，展示权威入链和来源引用；原件复用统一预览。 */
export function KnowledgeWikiReferences({
  resource,
  page,
  pages,
  onLink,
}: {
  resource: ResourceRef;
  page: WikiPage;
  pages: WikiPage[];
  onLink: (slug: string) => void;
}) {
  const t = useWikiCopy();
  const [sourceId, setSourceId] = useState("");
  const incoming = [...new Set(page.inLinks ?? [])];
  const sources = [...new Set(page.sourceRefs ?? [])];
  if (!incoming.length && !sources.length) return null;
  return (
    <>
      <footer className="mt-8 space-y-4" aria-label={t("references")}>
        {incoming.length > 0 && (
          <section className="space-y-2">
            <h3 className="type-label text-content-secondary">
              {t("linkedFrom")}
            </h3>
            <div className="flex flex-wrap gap-2">
              {incoming.map((slug) => (
                <Button key={slug} variant="text" onClick={() => onLink(slug)}>
                  {pages.find((item) => item.slug === slug)?.title || slug}
                </Button>
              ))}
            </div>
          </section>
        )}
        {sources.length > 0 && (
          <section className="space-y-2">
            <h3 className="type-label text-content-secondary">
              {t("sourceDocuments")}
            </h3>
            <div className="flex flex-wrap gap-2">
              {sources.map((reference, index) => (
                <SourceReference
                  key={reference}
                  resource={resource}
                  reference={reference}
                  label={t("sourceDocument", { number: index + 1 })}
                  onOpen={setSourceId}
                />
              ))}
            </div>
          </section>
        )}
      </footer>
      {sourceId && (
        <KnowledgeResultDocument
          key={sourceId}
          resource={resource}
          sourceId={sourceId}
          onClose={() => setSourceId("")}
        />
      )}
    </>
  );
}

function SourceReference({
  resource,
  reference,
  label,
  onOpen,
}: {
  resource: ResourceRef;
  reference: string;
  label: string;
  onOpen: (id: string) => void;
}) {
  // 官方远端 SourceRefs 协议允许 ID|标题；本地管线保存不可变来源版本 ID。
  const separator = reference.indexOf("|");
  const id = separator > 0 ? reference.slice(0, separator) : reference;
  const title = separator > 0 ? reference.slice(separator + 1) : "";
  const active = useRouteSceneActive();
  const source = useQuery({
    ...sourceDetailOptions(resource, id),
    enabled: active && Boolean(id) && !title,
  });
  return (
    <Button variant="text" onClick={() => onOpen(id)}>
      {title || source.data?.filename || label}
    </Button>
  );
}

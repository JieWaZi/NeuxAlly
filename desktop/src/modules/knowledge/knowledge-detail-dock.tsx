import { BookOpen, Files, Network } from "lucide-react";
import { DetailDockNavigation } from "@/shared/ui/detail-dock";
import { useCopy } from "./use-copy";

export function KnowledgeDetailDock({
  view,
  onChange,
}: {
  view: "raw" | "wiki" | "graph";
  onChange: (view: "raw" | "wiki" | "graph") => void;
}) {
  const t = useCopy();
  return (
    <DetailDockNavigation
      label={t("knowledgeViews")}
      items={[
        {
          id: "raw",
          label: t("documents"),
          icon: Files,
          active: view === "raw",
          onSelect: () => onChange("raw"),
        },
        {
          id: "wiki",
          label: "Wiki",
          icon: BookOpen,
          active: view === "wiki",
          onSelect: () => onChange("wiki"),
        },
        {
          id: "graph",
          label: t("graph"),
          icon: Network,
          active: view === "graph",
          onSelect: () => onChange("graph"),
        },
      ]}
    />
  );
}

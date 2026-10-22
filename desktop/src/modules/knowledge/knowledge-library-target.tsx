import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { knowledgeAdapter } from "@/shared/api/knowledge";
import {
  Select,
  SelectTrigger,
  SelectValue,
  SelectContent,
  SelectItem,
} from "@/shared/ui/select";
import { QueryState } from "@/shared/ui/query-state";
import { Field } from "./knowledge-controls";
import type { ResourceRef } from "./knowledge-transfer";
import { useCopy } from "./use-extended-copy";

/** 选择保存位置时才读取连接目录；创建目标始终进入同一库草稿。 */
export function KnowledgeLibraryTarget({
  value,
  onChange,
  disabled,
}: {
  value: ResourceRef;
  onChange: (value: ResourceRef) => void;
  disabled: boolean;
}) {
  const t = useCopy();
  const [opened, setOpened] = useState(false);
  const connections = useQuery({
    queryKey: ["knowledge", "connections", "creation-targets"],
    queryFn: () =>
      knowledgeAdapter({
        backend: "local",
        connectionId: "local",
        knowledgeBaseId: "",
      }).listConnections(),
    enabled: opened,
    staleTime: 0,
  });
  const targets = (connections.data?.data.items ?? []).filter(
    (entry) =>
      entry.enabled &&
      entry.id &&
      entry.id !== "local" &&
      (entry.backend === "custom" || entry.backend === "weknora"),
  );
  return (
    <Field label={t("storageLocation")} hint={t("creationLocationHelp")}>
      <Select
        value={value.connectionId}
        disabled={disabled}
        onOpenChange={setOpened}
        onValueChange={(id) => {
          if (id === "local")
            onChange({
              backend: "local",
              connectionId: "local",
              knowledgeBaseId: "",
            });
          else {
            const target = targets.find((entry) => entry.id === id);
            if (target?.id && target.backend)
              onChange({
                backend: target.backend,
                connectionId: target.id,
                knowledgeBaseId: "",
              });
          }
        }}
      >
        <SelectTrigger aria-label={t("storageLocation")}>
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="local">{t("local")}</SelectItem>
          {targets.map((entry) => (
            <SelectItem key={entry.id} value={entry.id || ""}>
              {entry.name || entry.id}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      {connections.error && (
        <QueryState
          error={connections.error}
          onRetry={() => void connections.refetch()}
        />
      )}
    </Field>
  );
}

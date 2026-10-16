import { Workflow } from "lucide-react";
import { chatMessageReferenceChipClass } from "./chat-reference-chip";

/** 已保存工作流的稳定身份及展示快照。 */
export type ChatWorkflowOption = {
  id: string;
  name: string;
  description: string;
};

export function ChatMessageWorkflowReferences({
  workflows,
}: {
  workflows: { id?: string; name?: string }[];
}) {
  return workflows.map((flow) => (
    <span key={flow.id} className={chatMessageReferenceChipClass}>
      <Workflow className="size-icon-sm shrink-0" />
      <span className="truncate">{flow.name}</span>
    </span>
  ));
}

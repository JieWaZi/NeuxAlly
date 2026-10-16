import { useTranslation } from "react-i18next";

import { DetailSection } from "@/shared/ui/detail-section";

type SkillInstructionsPanelProps = {
  content: string;
};

export function SkillInstructionsPanel({
  content,
}: SkillInstructionsPanelProps) {
  const { t } = useTranslation("skill");

  return (
    <DetailSection title="SKILL.md">
      <pre className="bg-card text-foreground type-body max-h-[min(720px,calc(100dvh-280px))] min-w-0 overflow-auto rounded-xl p-5 font-mono [overflow-wrap:anywhere] whitespace-pre-wrap">
        {content || t("detail.contentUnavailable")}
      </pre>
    </DetailSection>
  );
}

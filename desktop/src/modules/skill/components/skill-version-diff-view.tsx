import { generateDiffFile } from "@git-diff-view/file";
import { DiffModeEnum, DiffView } from "@git-diff-view/react";
import "@git-diff-view/react/styles/diff-view-pure.css";
import { useMemo } from "react";

/** 重型差异库只在用户查看版本时加载，弹窗结构由调用方同步挂载。 */
export function SkillVersionDiffView({
  currentContent,
  currentVersion,
  selectedContent,
  selectedVersion,
  path,
  theme,
}: {
  currentContent: string;
  currentVersion: string;
  selectedContent: string;
  selectedVersion: string;
  path: string;
  theme: "light" | "dark";
}) {
  const diffFile = useMemo(() => {
    const language = languageOf(path);
    const file = generateDiffFile(
      `${path} (v${currentVersion})`,
      currentContent,
      `${path} (v${selectedVersion})`,
      selectedContent,
      language,
      language,
    );
    file.initTheme(theme);
    file.init();
    file.buildUnifiedDiffLines();
    return file;
  }, [
    currentContent,
    currentVersion,
    selectedContent,
    selectedVersion,
    path,
    theme,
  ]);

  return (
    <div className="border-border bg-surface overflow-hidden rounded-lg border">
      <DiffView
        diffFile={diffFile}
        diffViewMode={DiffModeEnum.Unified}
        diffViewTheme={theme}
        diffViewHighlight
        diffViewWrap
        diffViewFontSize={12}
        className="skill-diff-view"
      />
    </div>
  );
}

function languageOf(path: string): string {
  if (path.endsWith(".md")) return "markdown";
  if (path.endsWith(".json")) return "json";
  if (path.endsWith(".yaml") || path.endsWith(".yml")) return "yaml";
  if (path.endsWith(".sh")) return "shell";
  return "text";
}

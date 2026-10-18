import { PluginInstalledPartners } from "./plugin-installed-partners";
import { useState, type ReactNode } from "react";
import { Cable, ExternalLink } from "lucide-react";
import { useTranslation } from "react-i18next";
import { openExternalURL } from "@/shared/desktop";
import { DetailSection } from "@/shared/ui/detail-section";
import { SurfacePatternPanel } from "@/shared/ui/surface-pattern-panel";
import { ReadOnlyField } from "@/shared/ui/read-only-field";
import { MarkdownContent } from "@/shared/ui/markdown-content";
import {
  PluginSkillDialog,
  type PluginSkillContext,
} from "./plugin-skill-dialog";
import { Button } from "@/shared/ui/button";
import { Card, CardBody } from "@/shared/ui/card";
import {
  ResourceListItem,
  ResourceItemAction,
} from "@/shared/ui/resource-list-item";
import { SkillIcon } from "@/modules/skill";
import {
  pluginUsesHostedVersion,
  pluginExplanation,
  builtinPluginGuide,
  pluginDisplayName,
} from "../model/plugin-view";
import type { PluginCatalogEntry, PluginDetail } from "../model/plugin-view";

/** 两种插件详情复用同一份包信息与真实安装关系，不复制安装状态。 */
export function PluginOverview({
  pkg,
  sourceKey,
  detail,
  scope = "global",
  installationStatus,
  skillContext,
  onUninstall,
  uninstalling,
}: {
  pkg?: PluginCatalogEntry["package"];
  sourceKey?: string;
  scope?: "global" | "partner";
  detail?: PluginDetail;
  installationStatus?: ReactNode;
  skillContext?: PluginSkillContext;
  onUninstall?: (id: string) => void;
  uninstalling?: boolean;
}) {
  const { t } = useTranslation("plugin");
  const [linkError, setLinkError] = useState(false);
  const manifest = pkg?.manifest;
  const [showSkills, setShowSkills] = useState(false);
  const [skill, setSkill] = useState<{ key: string; name: string } | null>(
    null,
  );
  const presentation = pkg?.presentation;
  const resourceSource = sourceKey ?? detail?.plugin?.sourceKey;
  const guide = builtinPluginGuide(pkg, resourceSource);
  const developer = presentation?.developerName || manifest?.author?.name;
  const info = [
    [
      t("version"),
      pluginUsesHostedVersion(pkg)
        ? t("officialHosted")
        : guide?.upstreamVersion || manifest?.version,
    ],
    [t("license"), manifest?.license],
  ];
  const urls = [
    [t("website"), presentation?.websiteURL || manifest?.homepage],
    [t("repository"), manifest?.repository],
    [t("privacy"), presentation?.privacyPolicyURL],
    [t("terms"), presentation?.termsOfServiceURL],
  ];
  return (
    <div className="@container/plugin-overview space-y-8">
      <SurfacePatternPanel pattern="wash" surface="scene" className="p-5">
        <div className="grid items-start gap-6 @[48rem]/plugin-overview:grid-cols-[minmax(0,2fr)_minmax(0,1fr)]">
          <DetailSection title={t("pluginReadme")}>
            <div className="max-w-prose">
              <MarkdownContent
                content={
                  pluginExplanation(pkg, resourceSource) ||
                  detail?.plugin?.description ||
                  t("noExplanation")
                }
              />
            </div>
          </DetailSection>
          <DetailSection title={t("information")}>
            <dl className="grid min-w-0 grid-cols-[auto_minmax(0,1fr)_auto_minmax(0,1fr)] items-center gap-4">
              {developer && (
                <ReadOnlyField
                  label={t("developer")}
                  layout="row"
                  className="contents [&>dd]:col-span-3"
                >
                  {developer}
                </ReadOnlyField>
              )}
              {info
                .filter(([, value]) => value)
                .map(([label, value]) => (
                  <ReadOnlyField
                    key={label}
                    label={label}
                    layout="row"
                    className="contents"
                  >
                    {value || t("unknown")}
                  </ReadOnlyField>
                ))}
              {urls
                .filter(([, url]) => url && /^https?:\/\//.test(url))
                .map(([label, url]) => (
                  <ReadOnlyField
                    key={label}
                    label={label}
                    layout="row"
                    className="contents"
                  >
                    <ResourceItemAction
                      label={label ?? ""}
                      variant="ghost"
                      onClick={() => {
                        setLinkError(false);
                        void openExternalURL(url ?? "").catch(() =>
                          setLinkError(true),
                        );
                      }}
                    >
                      <ExternalLink />
                    </ResourceItemAction>
                  </ReadOnlyField>
                ))}
            </dl>
            {linkError && (
              <p role="alert" className="type-body text-destructive">
                {t("failed")}
              </p>
            )}
            {!!pkg?.diagnostics?.length && (
              <div className="mt-4 space-y-2">
                <h3 className="type-label font-medium">{t("diagnostics")}</h3>
                {pkg.diagnostics.map((item) => (
                  <p
                    key={`${item.component}:${item.code}`}
                    className="type-caption text-content-secondary [overflow-wrap:anywhere]"
                  >
                    {item.component} · {item.code}
                  </p>
                ))}
              </div>
            )}
          </DetailSection>
        </div>
      </SurfacePatternPanel>
      {!!pkg?.skills?.length && (
        <DetailSection
          title={
            <>
              {t("skills")}{" "}
              <span className="text-content-caption">{pkg.skills.length}</span>
            </>
          }
        >
          <div className="space-y-3">
            <div className="grid grid-cols-1 gap-2 @[36rem]/resource-page:grid-cols-2 @[56rem]/resource-page:grid-cols-3">
              {pkg.skills.slice(0, showSkills ? undefined : 6).map((item) => (
                <ResourceListItem
                  key={item.key}
                  title={`/${item.name}`}
                  description={item.description}
                  descriptionLines={1}
                  headingLevel={3}
                  icon={<SkillIcon />}
                  label={t("viewSkill", { name: item.name })}
                  onClick={
                    skillContext
                      ? () =>
                          setSkill({
                            key: item.key ?? "",
                            name: item.name ?? "",
                          })
                      : undefined
                  }
                />
              ))}
            </div>
            {pkg.skills.length > 6 && (
              <Button
                variant="ghost"
                onClick={() => setShowSkills(!showSkills)}
              >
                {t(showSkills ? "collapseSkills" : "allSkills", {
                  count: pkg.skills.length,
                })}
              </Button>
            )}
          </div>
        </DetailSection>
      )}
      {!!pkg?.servers?.length && (
        <DetailSection
          title={
            <>
              {t("connectors")}{" "}
              <span className="text-content-caption">{pkg.servers.length}</span>
            </>
          }
        >
          <ul className="grid grid-cols-1 gap-2 @[36rem]/resource-page:grid-cols-2 @[56rem]/resource-page:grid-cols-3">
            {pkg.servers.map((server) => (
              <Card key={server.key} asChild variant="resource">
                <li>
                  <CardBody className="flex min-w-0 items-start gap-3 p-4">
                    <span className="bg-surface-subtle text-content-secondary grid size-10 shrink-0 place-items-center rounded-md">
                      <Cable className="size-icon-lg" />
                    </span>
                    <div className="min-w-0 flex-1">
                      <h3 className="type-label truncate font-medium">
                        {server.key}
                      </h3>
                      <p className="type-body text-content-secondary mt-1 truncate">
                        {server.url || server.command}
                      </p>
                    </div>
                  </CardBody>
                </li>
              </Card>
            ))}
          </ul>
        </DetailSection>
      )}
      {scope === "global" && (
        <DetailSection
          title={
            <>
              {t("installedPartners")}
              {installationStatus == null && (
                <>
                  {" "}
                  <span className="text-content-caption">
                    {detail?.installations?.length ?? 0}
                  </span>
                </>
              )}
            </>
          }
        >
          <PluginInstalledPartners
            detail={detail}
            installationStatus={installationStatus}
            onUninstall={onUninstall}
            uninstalling={uninstalling}
          />
        </DetailSection>
      )}
      {skill && skillContext && (
        <PluginSkillDialog
          sourceName={pluginDisplayName(pkg, detail?.plugin?.name)}
          context={skillContext}
          skillKey={skill.key}
          name={skill.name}
          onClose={() => setSkill(null)}
        />
      )}
    </div>
  );
}

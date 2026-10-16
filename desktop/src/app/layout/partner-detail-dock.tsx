import { BarChart3, Cable, Folder, History, Sparkles } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Link, useLocation } from "react-router-dom";

import { cn } from "@/shared/lib/cn";
import { buttonVariants } from "@/shared/ui/button";
import { Dock, DockIcon } from "@/shared/ui/dock";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/shared/ui/tooltip";

const sections = [
  { key: "overview", icon: BarChart3 },
  { key: "projects", icon: Folder },
  { key: "memory", icon: History },
  { key: "skills", icon: Sparkles },
  { key: "connectors", icon: Cable },
] as const;

export function PartnerDetailDock({ partnerId }: { partnerId: string }) {
  const { t } = useTranslation("common");
  const { pathname } = useLocation();
  const root = `/wakers/${partnerId}`;

  return (
    <nav
      aria-label={t("partnerNavigation.partnerMenu")}
      className="pointer-events-auto"
    >
      <TooltipProvider>
        <Dock direction="middle">
          {sections.map(({ icon: Icon, key }) => {
            const href = key === "overview" ? root : `${root}/${key}`;
            const active =
              pathname === href ||
              (pathname.startsWith(`${href}/`) && key !== "overview");
            return (
              <DockIcon key={key}>
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Link
                      to={href}
                      aria-label={t(`partnerNavigation.sections.${key}`)}
                      aria-current={active ? "page" : undefined}
                      className={cn(
                        buttonVariants({
                          variant: "ghost",
                          size: "dock",
                          shape: "circle",
                        }),
                        active && "text-content-primary",
                      )}
                    >
                      <Icon aria-hidden="true" strokeWidth={1.5} />
                    </Link>
                  </TooltipTrigger>
                  <TooltipContent side="top">
                    <p>{t(`partnerNavigation.sections.${key}`)}</p>
                  </TooltipContent>
                </Tooltip>
              </DockIcon>
            );
          })}
        </Dock>
      </TooltipProvider>
    </nav>
  );
}

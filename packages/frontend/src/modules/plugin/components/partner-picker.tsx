import { useQuery } from "@tanstack/react-query";
import { useState, useEffect } from "react";
import { useTranslation } from "react-i18next";
import { listWakersOptions, getWakerOptions } from "../api/plugin-queries";
import { withLocalErrorSurface } from "@/shared/api/local-error-surface";
import { Checkbox } from "@/shared/ui/checkbox";
import { Input } from "@/shared/ui/input";
import { Button } from "@/shared/ui/button";
import { QueryState } from "@/shared/ui/query-state";

export function PartnerPicker({
  selected,
  onChange,
  installed = [],
  onlyPartner,
  onNames,
}: {
  selected: string[];
  onChange: (ids: string[]) => void;
  installed?: string[];
  onlyPartner?: string;
  onNames?: (names: Record<string, string>) => void;
}) {
  const { t } = useTranslation("plugin");
  const [search, setSearch] = useState("");
  const [page, setPage] = useState(1);
  const library = useQuery({
    ...listWakersOptions(
      withLocalErrorSurface({
        query: { pageNum: page, pageSize: 50, query: search },
      }),
    ),
    enabled: !onlyPartner,
  });
  const owner = useQuery({
    ...getWakerOptions(
      withLocalErrorSurface({ path: { wakerId: onlyPartner ?? "pending" } }),
    ),
    enabled: Boolean(onlyPartner),
  });
  const query = onlyPartner ? owner : library;
  const partners = onlyPartner
    ? owner.data
      ? [owner.data]
      : []
    : (library.data?.resources ?? []).filter((item) => item.id);
  useEffect(() => {
    const items = onlyPartner
      ? owner.data
        ? [owner.data]
        : []
      : (library.data?.resources ?? []);
    onNames?.(
      Object.fromEntries(
        items
          .filter((item) => item.id)
          .map((item) => [item.id ?? "", item.name ?? ""]),
      ),
    );
  }, [onlyPartner, owner.data, library.data, onNames]);
  return (
    <div className="space-y-3">
      {!onlyPartner && (
        <Input
          aria-label={t("partners")}
          value={search}
          placeholder={t("search")}
          onChange={(event) => {
            setSearch(event.target.value);
            setPage(1);
          }}
        />
      )}
      {query.isPending || query.isError ? (
        <QueryState
          pending={query.isPending}
          error={query.error}
          onRetry={() => void query.refetch()}
        />
      ) : partners.length === 0 ? (
        <p className="text-muted-foreground type-body">{t("noPartners")}</p>
      ) : (
        <div className="space-y-1">
          {partners.map((partner) => {
            const id = partner.id;
            if (!id) return null;
            const existing = installed.includes(id);
            return (
              <label
                key={id}
                className="hover:bg-surface-subtle min-h-control-md rounded-control flex cursor-pointer items-center gap-3 px-2 py-2"
              >
                <Checkbox
                  checked={existing || selected.includes(id)}
                  disabled={existing}
                  onCheckedChange={(checked) =>
                    onChange(
                      checked
                        ? [...selected, id]
                        : selected.filter((value) => value !== id),
                    )
                  }
                />
                <span className="type-body min-w-0 flex-1 truncate">
                  {partner.name}
                </span>
                {existing && (
                  <span className="type-caption text-muted-foreground">
                    {t("installed")}
                  </span>
                )}
              </label>
            );
          })}
        </div>
      )}
      {!onlyPartner && (library.data?.totalSize ?? 0) > 50 && (
        <div className="flex items-center justify-between gap-3">
          <Button
            variant="ghost"
            disabled={page === 1}
            onClick={() => setPage(page - 1)}
          >
            {t("previous")}
          </Button>
          <span className="type-caption">
            {t("page", { page, count: library.data?.totalSize })}
          </span>
          <Button
            variant="ghost"
            disabled={page * 50 >= (library.data?.totalSize ?? 0)}
            onClick={() => setPage(page + 1)}
          >
            {t("next")}
          </Button>
        </div>
      )}
    </div>
  );
}

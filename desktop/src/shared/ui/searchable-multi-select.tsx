import { useId, useState } from "react";

import { consumeOverlayWheelScroll } from "@/shared/lib/overlay-scroll-wheel";
import { Button } from "@/shared/ui/button";
import { Checkbox } from "@/shared/ui/checkbox";
import { FieldControlTrigger } from "@/shared/ui/field-control-trigger";
import type { FieldControlTriggerProps } from "@/shared/ui/field-control-trigger";
import { Popover, PopoverContent, PopoverTrigger } from "@/shared/ui/popover";
import { SearchField } from "@/shared/ui/search-field";

export type SearchableMultiSelectOption = {
  value: string;
  label: string;
};

type SearchableMultiSelectProps = {
  options: readonly SearchableMultiSelectOption[];
  value: readonly string[];
  onValueChange: (value: string[]) => void;
  triggerLabel: string;
  searchLabel: string;
  emptyLabel: string;
  clearSearchLabel: string;
  clearSelectionLabel?: string;
  size?: FieldControlTriggerProps["size"];
};

export function SearchableMultiSelect({
  options,
  value,
  onValueChange,
  triggerLabel,
  searchLabel,
  emptyLabel,
  clearSearchLabel,
  clearSelectionLabel,
  size = "sm",
}: SearchableMultiSelectProps) {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const id = useId();
  const selected = new Set(value);
  const normalizedQuery = query.trim().toLocaleLowerCase();
  const visibleOptions = normalizedQuery
    ? options.filter((option) =>
        `${option.label} ${option.value}`
          .toLocaleLowerCase()
          .includes(normalizedQuery),
      )
    : options;

  return (
    <div className="w-full min-w-0">
      <Popover
        open={open}
        onOpenChange={(next) => {
          setOpen(next);
          if (!next) setQuery("");
        }}
      >
        <PopoverTrigger asChild>
          <FieldControlTrigger
            size={size}
            className="w-full min-w-0"
            aria-label={triggerLabel}
          >
            <span className="truncate">{triggerLabel}</span>
          </FieldControlTrigger>
        </PopoverTrigger>
        <PopoverContent
          align="start"
          width="trigger"
          className="flex flex-col p-2"
        >
          <div className="flex shrink-0 items-center gap-2">
            <SearchField
              width="full"
              containerClassName="min-w-0 flex-1"
              aria-label={searchLabel}
              placeholder={searchLabel}
              clearLabel={clearSearchLabel}
              value={query}
              onChange={(event) => setQuery(event.target.value)}
            />
            {clearSelectionLabel && value.length ? (
              <Button
                type="button"
                size="sm"
                variant="ghost"
                className="shrink-0"
                onClick={() => onValueChange([])}
              >
                {clearSelectionLabel}
              </Button>
            ) : null}
          </div>
          <div
            className="mt-2 max-h-56 min-h-0 overflow-y-auto overscroll-contain"
            role="group"
            aria-label={triggerLabel}
            onWheel={consumeOverlayWheelScroll}
          >
            {visibleOptions.length ? (
              visibleOptions.map((option, index) => (
                <div
                  key={option.value}
                  className="hover:bg-action-quiet-hover min-h-row-compact flex items-center gap-2 rounded-sm px-2"
                >
                  <Checkbox
                    id={`${id}-${index}`}
                    size="sm"
                    checked={selected.has(option.value)}
                    onCheckedChange={(checked) =>
                      onValueChange(
                        checked
                          ? [...value, option.value]
                          : value.filter((item) => item !== option.value),
                      )
                    }
                  />
                  <label
                    htmlFor={`${id}-${index}`}
                    className="type-ui min-w-0 flex-1 cursor-pointer truncate py-1"
                    title={option.value}
                  >
                    {option.label}
                  </label>
                </div>
              ))
            ) : (
              <p className="type-ui text-content-secondary px-2 py-3">
                {emptyLabel}
              </p>
            )}
          </div>
        </PopoverContent>
      </Popover>
    </div>
  );
}

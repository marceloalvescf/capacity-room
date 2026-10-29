import { PlusIcon, XIcon } from "@phosphor-icons/react";
import { useId, useState } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Field } from "@/components/ui/field";
import { IconButton } from "@/components/ui/icon-button";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { cn } from "@/lib/cn";

interface NamespacePickerProps {
  label: string;
  // The namespaces the source shows, sorted; null when the list is not known yet.
  names: readonly string[] | null;
  // What the popover says in place of the list while the names are not known.
  unknownText: string;
  // The legend is read by screen readers alone, for a field another legend already names.
  hideLabel?: boolean;
  selected: readonly string[];
  onChange: (selected: string[]) => void;
  error?: string | undefined;
}

// The chosen namespaces as chips in one field, and a popover that lists the namespaces of the source with a filter. A
// chosen name the source no longer shows stays chosen and is listed first, marked, so it never leaves unseen.
export function NamespacePicker({ label, names, unknownText, hideLabel = false, selected, onChange, error }: NamespacePickerProps) {
  const { t } = useTranslation();
  const [filter, setFilter] = useState("");
  const id = useId();
  const legendId = `${id}-legend`;
  const errorId = `${id}-error`;
  const known = new Set(names);
  const missing = names === null ? [] : selected.filter((name) => !known.has(name)).sort();
  const absent = (name: string) => names !== null && !known.has(name);
  const text = filter.trim();
  const shown = names === null ? [] : [...missing, ...names].filter((name) => name.includes(text));
  const choose = (name: string, on: boolean) => {
    onChange(on ? [...selected, name].sort() : selected.filter((other) => other !== name));
  };

  return (
    <fieldset
      tabIndex={-1}
      className="focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent"
      aria-invalid={error === undefined ? undefined : true}
      aria-describedby={error === undefined ? undefined : errorId}
    >
      <legend id={legendId} className={hideLabel ? "sr-only" : "mb-1 text-sm font-medium"}>
        {label}
      </legend>
      <div
        className={cn(
          "flex min-h-9 flex-wrap items-center gap-1.5 rounded-md border bg-surface px-1.5 py-1",
          error === undefined ? "border-input-border" : "border-critical",
        )}
      >
        {selected.map((name) => (
          <span
            key={name}
            className={cn(
              "inline-flex items-center gap-1 rounded-sm border bg-hover py-px pl-2 font-mono text-xs",
              absent(name) ? "border-dashed border-input-border" : "border-border",
            )}
          >
            {name}
            {absent(name) ? <span className="font-sans text-muted-fg">{t("namespaces.notFound")}</span> : null}
            <IconButton
              label={t("namespaces.remove", { name })}
              className="min-h-6 w-6 border-transparent bg-transparent"
              onClick={() => {
                choose(name, false);
              }}
            >
              <XIcon aria-hidden="true" className="size-3" />
            </IconButton>
          </span>
        ))}
        <Popover
          onOpenChange={(open) => {
            if (!open) {
              setFilter("");
            }
          }}
        >
          <PopoverTrigger asChild>
            <button
              type="button"
              aria-labelledby={`${legendId} ${id}-add`}
              className="inline-flex min-h-7 grow items-center gap-1 rounded-sm px-1.5 text-sm text-muted-fg not-disabled:hover:text-fg focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent"
            >
              <PlusIcon aria-hidden="true" className="size-4" />
              <span id={`${id}-add`}>{t("namespaces.add")}</span>
            </button>
          </PopoverTrigger>
          <PopoverContent side="bottom" align="start" collisionPadding={24} aria-labelledby={legendId} className="flex w-80 flex-col gap-2 p-2">
            {names === null ? (
              <p className="py-2 text-sm text-muted-fg">{unknownText}</p>
            ) : (
              <>
                <Field
                  label={t("namespaces.filter")}
                  type="search"
                  value={filter}
                  onChange={(event) => {
                    setFilter(event.target.value);
                  }}
                />
                <div role="group" aria-labelledby={legendId} className="flex max-h-56 flex-col overflow-y-auto px-1 font-mono">
                  {shown.length === 0 ? (
                    <p className="py-2 font-sans text-sm text-muted-fg">{t("namespaces.noMatch")}</p>
                  ) : (
                    shown.map((name) => (
                      <Checkbox
                        key={name}
                        label={name}
                        hint={absent(name) ? t("namespaces.notFound") : undefined}
                        checked={selected.includes(name)}
                        onChange={(event) => {
                          choose(name, event.target.checked);
                        }}
                      />
                    ))
                  )}
                </div>
              </>
            )}
            <div className="flex items-center justify-between gap-2 border-t border-border pt-2 text-xs text-muted-fg">
              {names === null ? (
                <span />
              ) : (
                <span>{t("namespaces.count", { found: names.length, selected: selected.length })}</span>
              )}
              <Button
                className="min-h-7 px-2 text-xs"
                disabled={selected.length === 0}
                onClick={() => {
                  onChange([]);
                }}
              >
                {t("namespaces.clear")}
              </Button>
            </div>
          </PopoverContent>
        </Popover>
      </div>
      {error === undefined ? null : (
        <p id={errorId} className="mt-1 text-sm text-critical">
          {error}
        </p>
      )}
    </fieldset>
  );
}

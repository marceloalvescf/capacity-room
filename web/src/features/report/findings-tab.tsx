import { CaretRightIcon } from "@phosphor-icons/react";
import { useId, useLayoutEffect, useMemo, useRef, useState, type Ref } from "react";
import { useTranslation } from "react-i18next";
import { viewScopeTypeValues, viewSeverityValues } from "@/api/schema.gen";
import { StatusMark, StatusShape } from "@/components/status-mark";
import { Button } from "@/components/ui/button";
import { Select } from "@/components/ui/select";
import { scrollBehavior } from "@/components/reveal";
import { cn } from "@/lib/cn";
import { useAddress } from "@/lib/address";
import type { ViewFinding } from "./api";
import { ConfidenceText, isUnknownPool, scopeName } from "./pieces";
import { proofOf, provedNames } from "./proof";
import { FindingsChart } from "./findings-chart";
import { FindingProof } from "./proof-bar";
import { useView } from "./report-page";
import { SearchField, useSearchText } from "@/components/search-field";

type Severity = (typeof viewSeverityValues)[number];
type ScopeType = (typeof viewScopeTypeValues)[number];

// The order of the groups, the most serious first.
const severityOrder: readonly Severity[] = ["critical", "attention", "healthy"];

// The severities the address holds: absent means all, an empty value means none, and a list of names none of which is
// known means all.
function readSeverities(param: string | null): ReadonlySet<Severity> {
  if (param === null) {
    return new Set(viewSeverityValues);
  }
  if (param === "") {
    return new Set();
  }
  const named = param.split(",");
  const known = severityOrder.filter((severity) => named.includes(severity));
  return new Set(known.length === 0 ? viewSeverityValues : known);
}

function readIndex(param: string | null): number | null {
  return param !== null && /^\d+$/.test(param) ? Number(param) : null;
}

interface Entry {
  finding: ViewFinding;
  index: number;
}

function ListItem({
  entry,
  selected,
  onSelect,
  buttonRef,
}: {
  entry: Entry;
  selected: boolean;
  onSelect: () => void;
  buttonRef?: Ref<HTMLButtonElement> | undefined;
}) {
  const { t } = useTranslation();
  const name = scopeName(entry.finding, t("report.node.unknownPool"));
  return (
    <li className="border-b border-border last:border-0">
      <button
        ref={buttonRef}
        type="button"
        aria-pressed={selected}
        onClick={onSelect}
        className={cn(
          "flex w-full flex-col gap-0.5 px-3 py-2 text-left text-sm focus-visible:outline-2 focus-visible:outline-offset-[-2px] focus-visible:outline-accent",
          selected ? "bg-info-surface text-fg" : "hover:underline",
        )}
      >
        <span className="flex flex-wrap items-center gap-x-3">
          <StatusShape status={entry.finding.severity} />
          <span className="sr-only">{t(`status.${entry.finding.severity}`)}</span>
          <span className="text-muted-fg">{t(`scope.${entry.finding.scope_type}`)}</span>
          {name === null ? null : (
            <span className={cn("min-w-0 truncate", !isUnknownPool(entry.finding) && "font-mono")}>{name}</span>
          )}
        </span>
        <span className="line-clamp-2">{entry.finding.sentence}</span>
      </button>
    </li>
  );
}

function Detail({ finding }: { finding: ViewFinding }) {
  const { t } = useTranslation();
  const name = scopeName(finding, t("report.node.unknownPool"));
  const proof = proofOf(finding);
  const proved = proof === null ? [] : provedNames(proof);
  const listed = finding.evidence.filter((item) => !proved.includes(item.name));
  return (
    <article className="flex flex-col gap-5 rounded-lg border border-border bg-surface p-5 text-surface-fg lg:sticky lg:top-4 lg:max-h-[calc(100dvh-2rem)] lg:overflow-y-auto">
      <header className="flex flex-col gap-1">
        <p className="flex flex-wrap items-center gap-x-3 text-sm">
          <StatusMark status={finding.severity} />
          <span className="text-muted-fg">
            {finding.scope_kind === ""
              ? t(`scope.${finding.scope_type}`)
              : `${t(`scope.${finding.scope_type}`)} · ${finding.scope_kind}`}
          </span>
          <span className="text-muted-fg">
            <ConfidenceText confidence={finding.confidence} />
          </span>
        </p>
        {name === null ? null : (
          <h3 className={cn("text-lg font-semibold break-all", !isUnknownPool(finding) && "font-mono")}>{name}</h3>
        )}
      </header>
      <section className="flex flex-col gap-1">
        <h4 className="label">{t("report.findings.observed")}</h4>
        <p className="text-base">{finding.sentence}</p>
      </section>
      {finding.action === "" ? null : (
        <section className="flex flex-col gap-1">
          <h4 className="label">{t("report.findings.action")}</h4>
          <p className="text-sm">{finding.action}</p>
        </section>
      )}
      {finding.evidence.length === 0 ? null : (
        <section className="flex flex-col gap-1">
          <h4 className="label">{t("report.findings.evidence")}</h4>
          <FindingProof finding={finding} className="mt-1" />
          {listed.length === 0 ? null : (
            <dl className="grid grid-cols-[auto_1fr] gap-x-6 gap-y-1 text-sm">
              {listed.map((item) => (
                <div key={item.name} className="contents">
                  <dt className="text-muted-fg">{t(`evidence.${item.name}`)}</dt>
                  <dd className="font-mono">{item.text}</dd>
                </div>
              ))}
            </dl>
          )}
        </section>
      )}
    </article>
  );
}

export function FindingsTab() {
  const { t } = useTranslation();
  const { findings } = useView();
  const address = useAddress();
  const { params, write } = address;
  const search = useSearchText(address);
  const groupId = useId();
  const query = search.text;
  const severities = readSeverities(params.get("severity"));
  const present = useMemo(
    () => viewScopeTypeValues.filter((type) => findings.some((finding) => finding.scope_type === type)),
    [findings],
  );
  const scopeParam = params.get("scope");
  const scope = present.find((type): type is ScopeType => type === scopeParam) ?? null;

  const unknownPool = t("report.node.unknownPool");
  const needle = query.toLowerCase();
  const shown: Entry[] = findings
    .map((finding, index) => ({ finding, index }))
    .filter(
      ({ finding }) =>
        severities.has(finding.severity) &&
        (scope === null || finding.scope_type === scope) &&
        `${scopeName(finding, unknownPool) ?? ""} ${finding.sentence} ${finding.action}`.toLowerCase().includes(needle),
    );

  const wanted = readIndex(params.get("finding"));
  const selected = shown.find((entry) => entry.index === wanted);

  // The reader's own folds are kept until the selected finding or a filter changes. A group is open when the selected
  // finding is in it, or when a search or a filter narrows the list to what it still shows.
  const foldKey = `${String(wanted ?? "")}|${query}|${params.get("severity") ?? ""}|${scope ?? ""}`;
  const [kept, setKept] = useState<{ key: string; chosen: ReadonlyMap<Severity, boolean> }>({ key: foldKey, chosen: new Map() });
  const chosen = kept.key === foldKey ? kept.chosen : new Map<Severity, boolean>();
  const narrowed = query !== "" || severities.size < severityOrder.length || scope !== null;
  const listId = useId();
  function isOpen(severity: Severity): boolean {
    return chosen.get(severity) ?? (selected?.finding.severity === severity || narrowed);
  }

  // A finding named in the address when the tab opens is brought into view once.
  const selectedItem = useRef<HTMLButtonElement>(null);
  const [deepLinked] = useState(wanted !== null);
  useLayoutEffect(() => {
    if (deepLinked) {
      selectedItem.current?.scrollIntoView({ block: "nearest", behavior: scrollBehavior() });
    }
  }, [deepLinked]);

  function toggle(severity: Severity) {
    const kept = severityOrder.filter((name) => severities.has(name) !== (name === severity));
    write((next) => {
      if (kept.length === severityOrder.length) {
        next.delete("severity");
      } else {
        next.set("severity", kept.join(","));
      }
    });
  }

  if (findings.length === 0) {
    return <p className="text-sm text-muted-fg">{t("report.findings.none")}</p>;
  }

  return (
    <div className="flex flex-col gap-4">
      <FindingsChart linkSegments />
      <div className="flex flex-wrap items-end gap-4">
        <SearchField label={t("report.findings.search")} text={search.text} onChange={search.change} />
        <div className="flex flex-col gap-1">
          <span id={groupId} className="text-sm font-medium">
            {t("report.findings.severity")}
          </span>
          <div role="group" aria-labelledby={groupId} className="flex flex-wrap gap-2">
            {severityOrder.map((severity) => {
              const pressed = severities.has(severity);
              return (
                <button
                  key={severity}
                  type="button"
                  aria-pressed={pressed}
                  onClick={() => {
                    toggle(severity);
                  }}
                  className={cn(
                    "inline-flex min-h-9 items-center gap-2 rounded-md border px-3 text-sm focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent",
                    "active:translate-y-px motion-safe:transition-transform motion-safe:duration-100 motion-safe:ease-out",
                    pressed
                      ? "border-fg bg-surface text-surface-fg hover:bg-hover"
                      : "border-border bg-transparent text-muted-fg hover:bg-hover hover:text-fg",
                  )}
                >
                  <StatusMark status={severity} plain={!pressed} />
                  <span className="font-mono">{findings.filter((finding) => finding.severity === severity).length}</span>
                </button>
              );
            })}
          </div>
        </div>
        <Select
          label={t("report.findings.scope")}
          value={scope ?? ""}
          onChange={(event) => {
            write((next) => {
              if (event.target.value === "") {
                next.delete("scope");
              } else {
                next.set("scope", event.target.value);
              }
            });
          }}
        >
          <option value="">{t("report.findings.allScopes")}</option>
          {present.map((type) => (
            <option key={type} value={type}>
              {t(`scope.${type}`)}
            </option>
          ))}
        </Select>
      </div>
      {shown.length === 0 ? (
        <div className="flex flex-col items-start gap-3">
          <p className="text-sm text-muted-fg">{t("report.findings.noMatch")}</p>
          <Button
            onClick={() => {
              search.change("");
              write((next) => {
                next.delete("severity");
                next.delete("scope");
              });
            }}
          >
            {t("report.findings.showAll")}
          </Button>
        </div>
      ) : (
        <div className="grid items-start gap-4 lg:grid-cols-[minmax(0,26rem)_minmax(0,1fr)]">
          <div className="flex flex-col gap-3">
            {severityOrder.map((severity) => {
              const group = shown.filter((entry) => entry.finding.severity === severity);
              if (group.length === 0) {
                return null;
              }
              const open = isOpen(severity);
              return (
                <section key={severity} className="flex flex-col gap-1">
                  <h2 className="text-base font-semibold">
                    <button
                      type="button"
                      aria-expanded={open}
                      aria-controls={open ? `${listId}-${severity}` : undefined}
                      onClick={() => {
                        setKept({ key: foldKey, chosen: new Map(chosen).set(severity, !open) });
                      }}
                      className="flex w-full items-center justify-between gap-3 rounded-lg border border-border bg-surface px-3 py-3 text-left text-surface-fg hover:bg-hover focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent"
                    >
                      <StatusMark status={severity} />
                      <span className="flex items-center gap-3">
                        <span className="font-mono text-muted-fg">{group.length}</span>
                        <CaretRightIcon
                          aria-hidden="true"
                          className={cn(
                            "size-4 text-accent motion-safe:transition-transform motion-safe:duration-150",
                            open && "rotate-90",
                          )}
                        />
                      </span>
                    </button>
                  </h2>
                  {open ? (
                    <ul
                      id={`${listId}-${severity}`}
                      className="max-h-[26rem] overflow-y-auto rounded-lg border border-border bg-surface text-surface-fg"
                    >
                      {group.map((entry) => (
                        <ListItem
                          key={entry.index}
                          entry={entry}
                          selected={entry === selected}
                          buttonRef={entry === selected ? selectedItem : undefined}
                          onSelect={() => {
                            write((next) => {
                              next.set("finding", String(entry.index));
                            });
                          }}
                        />
                      ))}
                    </ul>
                  ) : null}
                </section>
              );
            })}
          </div>
          {selected === undefined ? (
            <p className="rounded-lg border border-border bg-surface p-5 text-sm text-muted-fg lg:sticky lg:top-4">
              {t("report.findings.choose")}
            </p>
          ) : (
            <Detail finding={selected.finding} />
          )}
        </div>
      )}
    </div>
  );
}

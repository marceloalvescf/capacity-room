import { MagnifyingGlassIcon } from "@phosphor-icons/react";
import { useId, useMemo } from "react";
import { useTranslation } from "react-i18next";
import { useSearchParams } from "react-router";
import { viewScopeTypeValues, viewSeverityValues } from "@/api/schema.gen";
import { StatusMark } from "@/components/status-mark";
import { Field } from "@/components/ui/field";
import { Select } from "@/components/ui/select";
import { cn } from "@/lib/cn";
import type { ViewFinding } from "./api";
import { useView } from "./report-page";

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

// The scope name as the reader sees it: none for the cluster, a word for a node pool the report could not name.
function scopeName(finding: ViewFinding, unknownPool: string): string | null {
  if (finding.scope_type === "cluster") {
    return null;
  }
  return finding.scope_type === "nodepool" && finding.scope_name === "unknown" ? unknownPool : finding.scope_name;
}

interface Entry {
  finding: ViewFinding;
  index: number;
}

function ListItem({ entry, selected, onSelect }: { entry: Entry; selected: boolean; onSelect: () => void }) {
  const { t } = useTranslation();
  const name = scopeName(entry.finding, t("report.node.unknownPool"));
  return (
    <li className="border-b border-border last:border-0">
      <button
        type="button"
        aria-pressed={selected}
        onClick={onSelect}
        className={cn(
          "flex w-full flex-col gap-0.5 px-3 py-2 text-left text-sm focus-visible:outline-2 focus-visible:outline-offset-[-2px] focus-visible:outline-accent",
          selected ? "bg-info-surface text-fg" : "hover:underline",
        )}
      >
        <span className="flex flex-wrap items-center gap-x-3">
          <StatusMark status={entry.finding.severity} />
          <span className="text-muted-fg">{t(`scope.${entry.finding.scope_type}`)}</span>
          {name === null ? null : <span className="min-w-0 truncate font-mono">{name}</span>}
        </span>
        <span className="truncate">{entry.finding.sentence}</span>
      </button>
    </li>
  );
}

function Detail({ finding }: { finding: ViewFinding }) {
  const { t } = useTranslation();
  const name = scopeName(finding, t("report.node.unknownPool"));
  return (
    <article className="flex flex-col gap-4 rounded-md border border-border bg-surface p-4 text-surface-fg">
      <header className="flex flex-col gap-1">
        <p className="flex flex-wrap items-center gap-x-3 text-sm">
          <StatusMark status={finding.severity} />
          <span className="text-muted-fg">{t(`scope.${finding.scope_type}`)}</span>
          <span className="text-muted-fg">{t("report.confidence", { value: t(`confidence.${finding.confidence}`) })}</span>
        </p>
        {name === null ? null : <h3 className="font-mono text-lg font-semibold">{name}</h3>}
      </header>
      <section className="flex flex-col gap-1">
        <h4 className="text-sm font-semibold">{t("report.findings.observed")}</h4>
        <p className="text-sm">{finding.sentence}</p>
      </section>
      <section className="flex flex-col gap-1">
        <h4 className="text-sm font-semibold">{t("report.findings.action")}</h4>
        <p className="text-sm">{finding.action}</p>
      </section>
      {finding.evidence.length === 0 ? null : (
        <section className="flex flex-col gap-1">
          <h4 className="text-sm font-semibold">{t("report.findings.evidence")}</h4>
          <dl className="grid grid-cols-[auto_1fr] gap-x-6 gap-y-1 text-sm">
            {finding.evidence.map((item) => (
              <div key={item.name} className="contents">
                <dt className="text-muted-fg">{t(`evidence.${item.name}`)}</dt>
                <dd className="font-mono">{item.text}</dd>
              </div>
            ))}
          </dl>
        </section>
      )}
    </article>
  );
}

export function FindingsTab() {
  const { t } = useTranslation();
  const { findings } = useView();
  const [params, setParams] = useSearchParams();
  const groupId = useId();
  const query = params.get("q") ?? "";
  const severities = readSeverities(params.get("severity"));
  const present = useMemo(
    () => viewScopeTypeValues.filter((type) => findings.some((finding) => finding.scope_type === type)),
    [findings],
  );
  const scopeParam = params.get("scope");
  const scope = present.find((type): type is ScopeType => type === scopeParam) ?? null;

  const needle = query.toLowerCase();
  const shown: Entry[] = findings
      .map((finding, index) => ({ finding, index }))
      .filter(
        ({ finding }) =>
          severities.has(finding.severity) &&
          (scope === null || finding.scope_type === scope) &&
          `${finding.scope_name} ${finding.sentence} ${finding.action}`.toLowerCase().includes(needle),
      );;

  const wanted = readIndex(params.get("finding"));
  const selected = shown.find((entry) => entry.index === wanted) ?? shown[0];

  function write(change: (next: URLSearchParams) => void) {
    const next = new URLSearchParams(params);
    change(next);
    setParams(next, { replace: true });
  }

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
      <div className="flex flex-wrap items-end gap-4">
        <div className="relative">
          <Field
            label={t("report.findings.search")}
            type="search"
            value={query}
            className="w-72 max-w-full pl-9"
            onChange={(event) => {
              write((next) => {
                if (event.target.value === "") {
                  next.delete("q");
                } else {
                  next.set("q", event.target.value);
                }
              });
            }}
          />
          <MagnifyingGlassIcon aria-hidden="true" className="pointer-events-none absolute bottom-2.5 left-3 size-4 text-muted-fg" />
        </div>
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
                    "inline-flex min-h-9 items-center gap-2 rounded-full border px-3 text-sm focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent",
                    pressed ? "border-accent bg-info-surface text-fg" : "border-input-border bg-surface text-muted-fg",
                  )}
                >
                  <StatusMark status={severity} />
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
      {selected === undefined ? (
        <p className="text-sm text-muted-fg">{t("report.findings.noMatch")}</p>
      ) : (
        <div className="grid items-start gap-4 lg:grid-cols-[minmax(0,26rem)_minmax(0,1fr)]">
          <div className="flex flex-col gap-3">
            {severityOrder.map((severity) => {
              const group = shown.filter((entry) => entry.finding.severity === severity);
              if (group.length === 0) {
                return null;
              }
              return (
                <section key={severity} className="flex flex-col gap-1">
                  <h2 className="flex items-center justify-between text-sm font-semibold">
                    <StatusMark status={severity} />
                    <span className="font-mono text-muted-fg">{group.length}</span>
                  </h2>
                  <ul className="rounded-md border border-border bg-surface text-surface-fg">
                    {group.map((entry) => (
                      <ListItem
                        key={entry.index}
                        entry={entry}
                        selected={entry === selected}
                        onSelect={() => {
                          write((next) => {
                            next.set("finding", String(entry.index));
                          });
                        }}
                      />
                    ))}
                  </ul>
                </section>
              );
            })}
          </div>
          <Detail finding={selected.finding} />
        </div>
      )}
    </div>
  );
}

import { useId } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import { StatusShape } from "@/components/status-mark";
import { cn } from "@/lib/cn";
import { formatAge } from "@/lib/format";
import { chipOf, type ChipLine, type HomeCluster } from "./clusters";

const stepKeys = {
  queued: "home.chip.step.queued",
  collecting: "home.chip.step.collecting",
  rendering: "home.chip.step.rendering",
} as const;

// The muted line of a chip: the age of the latest report, the step of a request in flight, or why there is none.
function useLineText(line: ChipLine, now: number): string {
  const { t, i18n } = useTranslation();
  switch (line.kind) {
    case "age":
      return formatAge(line.at, now, i18n.language);
    case "step":
      return t(stepKeys[line.step]);
    case "failed":
      return t("reportStatus.failed");
    case "expired":
      return t("reports.expired");
    case "none":
      return t("home.chip.none");
  }
}

// The chosen chip takes the look of a pressed severity toggle; the others sit on the ground.
const chip =
  "relative flex items-center gap-2.5 rounded-lg border px-3.5 py-2.5 text-sm focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent hover:bg-hover";

function Chip({ cluster, chosen, unseen, now }: { cluster: HomeCluster; chosen: boolean; unseen: boolean; now: number }) {
  const { t } = useTranslation();
  const { status, line } = chipOf(cluster);
  const lineText = useLineText(line, now);
  const named = cluster.latest?.cluster_name ?? null;
  return (
    <Link
      to={{ search: new URLSearchParams({ cluster: cluster.profile_id }).toString() }}
      replace
      aria-current={chosen ? "true" : undefined}
      className={cn(chip, chosen ? "border-fg bg-surface text-surface-fg" : "border-border bg-transparent text-fg")}
    >
      <StatusShape status={status} />
      <span className="sr-only">{t(`status.${status}`)}</span>
      <span className={cn(named !== null && "font-mono")}>{named ?? cluster.display_name}</span>
      <span className="text-xs text-muted-fg">{lineText}</span>
      {unseen ? (
        <>
          <span aria-hidden="true" className="absolute top-1.5 right-1.5 size-1.5 rounded-full bg-accent" />
          <span className="sr-only">{t("home.chip.unseen")}</span>
        </>
      ) : null}
    </Link>
  );
}

// One chip for each cluster, in the order given, the chosen one marked as current.
export function ClusterStrip({
  clusters,
  chosen,
  unseen,
  now,
}: {
  clusters: HomeCluster[];
  chosen: string;
  unseen: ReadonlySet<string>;
  now: number;
}) {
  const { t } = useTranslation();
  const headingId = useId();
  return (
    <section className="flex flex-col gap-2">
      <h2 id={headingId} className="label">
        {t("home.strip.heading")}
      </h2>
      <ul aria-labelledby={headingId} className="flex flex-wrap gap-2">
        {clusters.map((cluster) => (
          <li key={cluster.profile_id}>
            <Chip cluster={cluster} chosen={cluster.profile_id === chosen} unseen={unseen.has(cluster.profile_id)} now={now} />
          </li>
        ))}
      </ul>
    </section>
  );
}

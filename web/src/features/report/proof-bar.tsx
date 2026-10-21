import { Fragment } from "react";
import { useTranslation } from "react-i18next";
import { Separator } from "@/components/separator";
import type { Status } from "@/components/status-mark";
import { cn } from "@/lib/cn";
import type { ViewFinding } from "./api";
import { proofOf } from "./proof";
import { RangeBar } from "./range-bar";

const width = 1000;
const height = 16;
const top = 4;
const depth = 8;

const fill: Partial<Record<Status, string>> = {
  critical: "fill-critical",
  attention: "fill-attention",
  healthy: "fill-healthy",
};

// Above this many units a division per unit would be thinner than a hairline, so none is drawn.
const unitLimit = 40;

// A value is drawn up to this many times its reference; past that the bar breaks, so the reference keeps a
// readable place on the scale. The printed numbers carry the size of the overrun.
const ceiling = 4;

// A value drawn against its reference on one track. Up to the reference the fill takes the full height of the
// track; past it, the overrun runs at half height, and the tick of the reference stands where the reference falls
// once the value reaches it. A value beyond the ceiling ends in a break. The fill takes the colour of the status
// the view gives, ink when it gives none. A count may divide the track per unit and mark its floor with a dashed
// tick. The numbers are printed beside it, so the drawing is hidden from assistive technology. The fill grows once
// from the left, under motion-safe only; otherwise it is drawn at once.
export function ProofBar({
  value,
  reference,
  status,
  units = false,
  floor = null,
  className,
}: {
  value: number;
  reference: number;
  status: Status | null;
  units?: boolean;
  floor?: number | null;
  className?: string;
}) {
  const domain = Math.min(Math.max(value, reference), reference * ceiling);
  if (!(domain > 0)) {
    return null;
  }
  const x = (at: number) => (width * Math.min(Math.max(at, 0), domain)) / domain;
  const colour = (status === null ? undefined : fill[status]) ?? "fill-neatline";
  const within = x(Math.min(value, reference));
  const past = Math.max(x(value) - x(reference), 0);
  const divisions =
    units && Number.isInteger(domain) && domain <= unitLimit ? Array.from({ length: domain - 1 }, (_, n) => n + 1) : [];
  const tick = (at: number) => ({ x1: x(at), x2: x(at), y1: 0, y2: height, vectorEffect: "non-scaling-stroke" as const });
  const slash = (at: number) => ({ x1: at, x2: at + 16, y1: top + depth + 3, y2: top - 3, vectorEffect: "non-scaling-stroke" as const });
  return (
    <svg
      viewBox={`0 0 ${String(width)} ${String(height)}`}
      preserveAspectRatio="none"
      aria-hidden="true"
      className={cn("block h-4 w-full overflow-visible", className)}
    >
      <rect x={0} y={top} width={width} height={depth} className="fill-track" />
      <g className="origin-left [transform-box:view-box] motion-safe:animate-proof">
        <rect data-fill="" x={0} y={top} width={within} height={depth} className={colour} />
        {past > 0 ? (
          <rect
            data-overrun=""
            x={x(reference)}
            y={top + depth / 4}
            width={past}
            height={depth / 2}
            className={colour}
          />
        ) : null}
      </g>
      {divisions.map((n) => (
        <line key={n} data-unit="" {...tick(n)} y1={top} y2={top + depth} strokeWidth="2" className="stroke-surface" />
      ))}
      {floor !== null && floor > 0 && floor < domain ? (
        <line data-floor="" {...tick(floor)} strokeWidth="1.5" strokeDasharray="2 2" className="stroke-neatline" />
      ) : null}
      {value >= reference ? <line data-reference="" {...tick(reference)} strokeWidth="2" className="stroke-neatline" /> : null}
      {value > reference * ceiling ? (
        <g data-break="" strokeWidth="3" className="stroke-surface">
          <line {...slash(width - 52)} />
          <line {...slash(width - 34)} />
        </g>
      ) : null}
    </svg>
  );
}

// The proof of a finding: its leading evidence large, the rest beside it, and the bar or the range that draws
// them. A finding whose evidence holds nothing to draw shows nothing here.
export function FindingProof({ finding, className }: { finding: ViewFinding; className?: string }) {
  const { t } = useTranslation();
  const proof = proofOf(finding);
  if (proof === null) {
    return null;
  }
  const lead = proof.kind === "range" ? proof.high : proof.lead;
  const rest =
    proof.kind === "range" ? [proof.low, ...(proof.spread === null ? [] : [proof.spread])] : proof.rest;
  return (
    <div className={cn("flex flex-col gap-2", className)}>
      <p className="flex flex-wrap items-baseline text-sm text-muted-fg">
        <span className="inline-flex items-baseline gap-2">
          {t(`evidence.${lead.name}`)}
          <span className="font-mono text-lg font-semibold text-fg">{lead.text}</span>
        </span>
        {rest.map((item) => (
          <Fragment key={item.name}>
            <Separator />
            <span className="inline-flex items-baseline gap-1.5">
              {t(`evidence.${item.name}`)}
              <span className="font-mono text-fg">{item.text}</span>
            </span>
          </Fragment>
        ))}
      </p>
      {proof.kind === "range" ? (
        <RangeBar low={proof.low} high={proof.high} status={finding.severity} />
      ) : (
        <ProofBar value={proof.value} reference={proof.reference} status={finding.severity} units={proof.count} />
      )}
    </div>
  );
}

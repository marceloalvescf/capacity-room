import { useId } from "react";

const width = 360;
const height = 48;
const left = 1;
const span = width - 2;
const top = 6;
const depth = 18;

// At most ten labelled soundings, so no two labels meet; the maximum always carries one.
function soundings(max: number): number[] {
  const step = max <= 12 ? 1 : Math.ceil(max / 10);
  const values: number[] = [];
  for (let value = 0; value <= max - step; value += step) {
    values.push(value);
  }
  return [...values, max];
}

const tints = ["fill-depth-1", "fill-depth-2", "fill-depth-3"] as const;

// The severity the view gives the cluster: it alone decides whether a full scale is drawn as a hazard.
export type GaugeSeverity = "critical" | "attention" | null;

const hazardStroke = { critical: "stroke-critical", attention: "stroke-attention" } as const;

// Above this many nodes a band per node would be thinner than a hairline, so the level is drawn whole.
const bandLimit = 60;

// The nodes of the cluster as a depth scale: one band per node up to the maximum, the bands in use tinted
// deeper toward the maximum, and the minimum as a dashed contour. Once no room is left, the last band and the
// line that closes the scale take the severity of the cluster finding: hatched and drawn in its colour when it
// is critical or attention, plain depth and a neat-line otherwise. Counts are whole nodes: the maximum and the
// level are rounded, and the level is held between none and the maximum. The numbers are in the headline above
// it, so the drawing is hidden from assistive technology. Under reduced motion, or with no animation at all,
// every part is drawn at once.
export function DepthGauge({
  current,
  max: rawMax,
  min: rawMin,
  severity,
}: {
  current: number;
  max: number;
  min: number | null;
  severity: GaugeSeverity;
}) {
  const pattern = `hazard-${useId().replace(/[^a-zA-Z0-9]/g, "")}`;
  const max = Number.isFinite(rawMax) ? Math.round(rawMax) : 0;
  if (max <= 0) {
    return null;
  }
  const level = Number.isFinite(current) ? Math.min(Math.max(Math.round(current), 0), max) : 0;
  const min = rawMin === null || !Number.isFinite(rawMin) ? null : rawMin;
  const x = (value: number) => left + (span * Math.min(Math.max(value, 0), max)) / max;
  const full = level >= max;
  const hatched = full && severity !== null;
  // The tint drawn up to here; a hatched last band is drawn on its own.
  const tinted = hatched ? max - 1 : level;
  const perNode = max <= bandLimit;
  const bands = perNode ? Array.from({ length: max }, (_, n) => n) : [];
  const tint = (n: number) => tints[Math.min(2, Math.floor((3 * n) / max))];
  // Above the limit, the same three tints over the same thirds of the scale, each one rectangle.
  const thirds = perNode
    ? []
    : tints.flatMap((className, third) => {
        const from = (third * max) / 3;
        const to = Math.min(((third + 1) * max) / 3, tinted);
        return to > from ? [{ className, from, to }] : [];
      });
  // A class would override the fill attribute, so the hatched band takes the pattern alone.
  const band = (n: number, className?: string) => (
    <rect
      key={n}
      data-band={n}
      x={x(n)}
      y={top}
      width={x(n + 1) - x(n)}
      height={depth}
      data-hazard={className === undefined ? "" : undefined}
      fill={className === undefined ? `url(#${pattern})` : undefined}
      className={className}
    />
  );
  return (
    <svg viewBox={`0 0 ${String(width)} ${String(height)}`} aria-hidden="true" className="mt-2 block h-auto w-full max-w-[360px] overflow-visible">
      {hatched ? (
        <defs>
          <pattern id={pattern} width="6" height="6" patternUnits="userSpaceOnUse" patternTransform="rotate(45)">
            <rect width="6" height="6" className="fill-depth-3" />
            <line x1="0" y1="0" x2="0" y2="6" strokeWidth="2.2" className={hazardStroke[severity]} />
          </pattern>
        </defs>
      ) : null}
      {/* The level rises once, from the left edge, to the nodes in use. */}
      <g className="origin-left [transform-box:view-box] motion-safe:animate-sounding">
        {bands.filter((n) => n < tinted).map((n) => band(n, tint(n)))}
        {thirds.map(({ className, from, to }) => (
          <rect key={className} data-level="" x={x(from)} y={top} width={x(to) - x(from)} height={depth} className={className} />
        ))}
        {full ? null : (
          <line x1={x(level)} y1={top - 4} x2={x(level)} y2={top + depth + 4} strokeWidth="2" className="stroke-accent" />
        )}
      </g>
      {/* Once the level is in, the last band shows hatched. */}
      {hatched ? <g className="motion-safe:animate-hazard-in">{band(max - 1)}</g> : null}
      {bands.filter((n) => n >= level).map((n) => band(n, "fill-transparent"))}
      {max <= 40
        ? bands.slice(1).map((n) => (
            <line key={n} x1={x(n)} y1={top} x2={x(n)} y2={top + depth} strokeWidth="1" className="stroke-bg" />
          ))
        : null}
      {min !== null && min > 0 && min < max ? (
        <line
          data-contour=""
          x1={x(min)}
          y1={top}
          x2={x(min)}
          y2={top + depth}
          strokeWidth="1"
          strokeDasharray="2 2"
          className="stroke-neatline"
        />
      ) : null}
      <rect x={left} y={top} width={span} height={depth} fill="none" strokeWidth="1" className="stroke-neatline" />
      {full ? (
        <line
          data-closing=""
          x1={x(max)}
          y1={top - 4}
          x2={x(max)}
          y2={top + depth + 4}
          strokeWidth="2"
          className={`${hatched ? hazardStroke[severity] : "stroke-neatline"} motion-safe:animate-hazard-in`}
        />
      ) : null}
      {soundings(max).map((value) => (
        <g key={value}>
          <line x1={x(value)} y1={top + depth} x2={x(value)} y2={top + depth + 5} strokeWidth="1" className="stroke-neatline" />
          <text
            x={x(value)}
            y={top + depth + 18}
            textAnchor={value === 0 ? "start" : value === max ? "end" : "middle"}
            className="fill-muted-fg font-mono text-sounding italic"
          >
            {value}
          </text>
        </g>
      ))}
    </svg>
  );
}

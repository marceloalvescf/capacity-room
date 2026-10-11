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

// The nodes of the cluster as a depth scale: one band per node up to the maximum, the bands in use tinted
// deeper toward the maximum, and the minimum as a dashed contour. Once no room is left, the last band and the
// line that closes the scale take the severity of the cluster finding: hatched and drawn in its colour when it
// is critical or attention, plain depth and a neat-line otherwise. The numbers are in the headline above it,
// so the drawing is hidden from assistive technology. Under reduced motion, or with no animation at all, every
// part is drawn at once.
export function DepthGauge({
  current,
  max,
  min,
  severity,
}: {
  current: number;
  max: number;
  min: number | null;
  severity: GaugeSeverity;
}) {
  const pattern = `hazard-${useId().replace(/[^a-zA-Z0-9]/g, "")}`;
  const x = (value: number) => left + (span * Math.min(Math.max(value, 0), max)) / max;
  const full = current >= max;
  const hatched = full && severity !== null;
  const bands = Array.from({ length: max }, (_, n) => n);
  const filled = bands.filter((n) => n < current && !(hatched && n === max - 1));
  const tint = (n: number) => tints[Math.min(2, Math.floor((3 * n) / max))];
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
        {filled.map((n) => band(n, tint(n)))}
        {full ? null : (
          <line x1={x(current)} y1={top - 4} x2={x(current)} y2={top + depth + 4} strokeWidth="2" className="stroke-accent" />
        )}
      </g>
      {/* Once the level is in, the last band shows hatched. */}
      {hatched ? <g className="motion-safe:animate-hazard-in">{band(max - 1)}</g> : null}
      {bands.filter((n) => n >= current).map((n) => band(n, "fill-transparent"))}
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

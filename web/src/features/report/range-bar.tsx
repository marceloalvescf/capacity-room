import { useTranslation } from "react-i18next";
import type { Status } from "@/components/status-mark";
import type { ViewFinding } from "./api";
import type { Peak } from "./proof";

type Evidence = ViewFinding["evidence"][number];

const width = 600;
const height = 92;
const trackY = 38;
const trackHeight = 8;
const radius = 7;
const mid = trackY + trackHeight / 2;
const labelWidth = 190;
const scale = [0, 25, 50, 75, 100];

const fill: Partial<Record<Status, string>> = {
  critical: "fill-critical",
  attention: "fill-attention",
  healthy: "fill-healthy",
};
const stroke: Partial<Record<Status, string>> = {
  critical: "stroke-critical",
  attention: "stroke-attention",
  healthy: "stroke-healthy",
};

// The peaks of the nodes of a pool drawn as a range on a track of 0 to 100 percent: an open dot at the lowest, a
// filled dot at the highest, the spread between them, each named by the words and the text of its evidence. The
// dots and the span take the colour of the status the view gives, ink when it gives none. The numbers are also
// printed beside it, so the drawing is hidden from assistive technology.
export function RangeBar({
  low,
  high,
  spread,
  status,
}: {
  low: Peak;
  high: Peak;
  spread: Evidence | null;
  status: Status | null;
}) {
  const { t } = useTranslation();
  const x = (percent: number) => (width * Math.min(Math.max(percent, 0), 100)) / 100;
  const colour = (status === null ? undefined : fill[status]) ?? "fill-neatline";
  const outline = (status === null ? undefined : stroke[status]) ?? "stroke-neatline";
  const lowX = x(low.raw);
  const highX = x(high.raw);
  const crowded = highX - lowX < labelWidth;
  const label = (item: Evidence) => `${t(`evidence.${item.name}`)} ${item.text}`;
  return (
    <svg viewBox={`0 0 ${String(width)} ${String(height)}`} aria-hidden="true" className="block h-auto w-full overflow-visible">
      <g className="fill-current font-mono text-[13px] text-fg">
        <text data-label="low" x={lowX} y={trackY - 14} textAnchor={crowded ? "end" : "middle"}>
          {label(low)}
        </text>
        <text data-label="high" x={highX} y={trackY - 14} textAnchor={crowded ? "start" : "middle"}>
          {label(high)}
        </text>
        {spread === null ? null : (
          <text data-label="spread" x={(lowX + highX) / 2} y={trackY + 26} textAnchor="middle">
            {label(spread)}
          </text>
        )}
      </g>
      <rect data-track="" x={0} y={trackY} width={width} height={trackHeight} rx={trackHeight / 2} className="fill-track" />
      <rect
        data-span=""
        x={lowX}
        y={trackY}
        width={Math.max(highX - lowX, 0)}
        height={trackHeight}
        className={`${colour} opacity-35`}
      />
      <circle data-low="" cx={lowX} cy={mid} r={radius} strokeWidth="3" className={`fill-surface ${outline}`} />
      <circle data-high="" cx={highX} cy={mid} r={radius} className={colour} />
      <g className="fill-current font-mono text-[12px] text-muted-fg">
        {scale.map((percent) => (
          <text
            key={percent}
            x={x(percent)}
            y={height - 2}
            textAnchor={percent === 0 ? "start" : percent === 100 ? "end" : "middle"}
          >
            {`${String(percent)}%`}
          </text>
        ))}
      </g>
      <g className="stroke-neatline" strokeWidth="1">
        {scale.map((percent) => (
          <line key={percent} x1={x(percent)} x2={x(percent)} y1={trackY + 14} y2={trackY + 20} />
        ))}
      </g>
    </svg>
  );
}

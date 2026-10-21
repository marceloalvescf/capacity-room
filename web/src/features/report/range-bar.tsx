import { useTranslation } from "react-i18next";
import type { Status } from "@/components/status-mark";
import type { ViewFinding } from "./api";
import type { Peak } from "./proof";

type Evidence = ViewFinding["evidence"][number];

const trackY = 38;
const trackHeight = 8;
const radius = 7;
const mid = trackY + trackHeight / 2;
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

type Anchor = "start" | "middle" | "end";

const percent = (at: number) => `${String(Math.min(Math.max(at, 0), 100))}%`;

// A label hangs toward the centre from its dot, so it stays inside the drawing at either end of the track.
const anchor = (at: number): Anchor => (at <= 50 ? "start" : "end");

// The peaks of the nodes of a pool drawn as a range on a track of 0 to 100 percent: an open dot at the lowest, a
// filled dot at the highest, the span between them, each named by the words and the text of its evidence: the lowest above the track, the highest below it. Marks
// and labels are placed by percent of the width, so the dots stay round and the text keeps its size however wide
// the drawing is. The dots and the span take the colour of the status the view gives, ink when it gives none. The
// numbers are also printed beside it, so the drawing is hidden from assistive technology.
export function RangeBar({
  low,
  high,
  status,
}: {
  low: Peak;
  high: Peak;
  status: Status | null;
}) {
  const { t } = useTranslation();
  const colour = (status === null ? undefined : fill[status]) ?? "fill-neatline";
  const outline = (status === null ? undefined : stroke[status]) ?? "stroke-neatline";
  const label = (item: Evidence) => `${t(`evidence.${item.name}`)} ${item.text}`;
  return (
    <div className="px-3">
      <svg width="100%" height="108" aria-hidden="true" className="block h-[108px] w-full overflow-visible">
        <g className="fill-current font-mono text-[13px] text-fg">
          <text data-label="low" x={percent(low.raw)} y={trackY - 14} textAnchor={anchor(low.raw)}>
            {label(low)}
          </text>
          <text data-label="high" x={percent(high.raw)} y={trackY + 36} textAnchor={anchor(high.raw)}>
            {label(high)}
          </text>
        </g>
        <rect data-track="" x="0" y={trackY} width="100%" height={trackHeight} rx={trackHeight / 2} className="fill-track" />
        <rect
          data-span=""
          x={percent(low.raw)}
          y={trackY}
          width={percent(Math.max(high.raw - low.raw, 0))}
          height={trackHeight}
          className={`${colour} opacity-35`}
        />
        <circle data-low="" cx={percent(low.raw)} cy={mid} r={radius} strokeWidth="3" className={`fill-surface ${outline}`} />
        <circle data-high="" cx={percent(high.raw)} cy={mid} r={radius} className={colour} />
        <g className="stroke-neatline" strokeWidth="1">
          {scale.map((at) => (
            <line key={at} x1={percent(at)} x2={percent(at)} y1={trackY + 14} y2={trackY + 20} />
          ))}
        </g>
        <g className="fill-current font-mono text-[12px] text-muted-fg">
          {scale.map((at) => (
            <text key={at} x={percent(at)} y={104} textAnchor={at === 0 ? "start" : at === 100 ? "end" : "middle"}>
              {`${String(at)}%`}
            </text>
          ))}
        </g>
      </svg>
    </div>
  );
}

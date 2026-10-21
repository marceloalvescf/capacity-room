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

// A label centred on a dot this close to an end of the track would spill outside the drawing, and two labels
// whose dots are this close would overlap.
const edge = 15;
const crowd = 30;

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

// The side a label hangs from its dot: inward at an end of the track, apart from a close neighbour, else centred.
function anchors(low: number, high: number): [Anchor, Anchor] {
  const close = high - low < crowd;
  const first: Anchor = low < edge ? "start" : close ? "end" : "middle";
  const second: Anchor = high > 100 - edge ? "end" : close ? "start" : "middle";
  return [first, second];
}

// The peaks of the nodes of a pool drawn as a range on a track of 0 to 100 percent: an open dot at the lowest, a
// filled dot at the highest, the spread between them, each named by the words and the text of its evidence. Marks
// and labels are placed by percent of the width, so the dots stay round and the text keeps its size however wide
// the drawing is. The dots and the span take the colour of the status the view gives, ink when it gives none. The
// numbers are also printed beside it, so the drawing is hidden from assistive technology.
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
  const colour = (status === null ? undefined : fill[status]) ?? "fill-neatline";
  const outline = (status === null ? undefined : stroke[status]) ?? "stroke-neatline";
  const label = (item: Evidence) => `${t(`evidence.${item.name}`)} ${item.text}`;
  const [lowAnchor, highAnchor] = anchors(low.raw, high.raw);
  return (
    <div className="px-2">
      <svg width="100%" height="92" aria-hidden="true" className="block h-[92px] w-full overflow-visible">
        <g className="fill-current font-mono text-[13px] text-fg">
          <text data-label="low" x={percent(low.raw)} y={trackY - 14} textAnchor={lowAnchor}>
            {label(low)}
          </text>
          <text data-label="high" x={percent(high.raw)} y={trackY - 14} textAnchor={highAnchor}>
            {label(high)}
          </text>
          {spread === null ? null : (
            <text data-label="spread" x={percent((low.raw + high.raw) / 2)} y={trackY + 28} textAnchor="middle">
              {label(spread)}
            </text>
          )}
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
            <text key={at} x={percent(at)} y={88} textAnchor={at === 0 ? "start" : at === 100 ? "end" : "middle"}>
              {`${String(at)}%`}
            </text>
          ))}
        </g>
      </svg>
    </div>
  );
}

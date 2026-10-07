import type { ReactNode } from "react";
import { cn } from "@/lib/cn";

type Tone = "error" | "notice";

const tones: Record<Tone, string> = {
  error: "bg-critical-surface text-critical-surface-fg border-critical-border",
  notice: "bg-info-surface text-info-surface-fg border-border",
};

export function Alert({ tone, children }: { tone: Tone; children: ReactNode }) {
  return (
    <div role={tone === "error" ? "alert" : "status"} className={cn("rounded-md border px-3 py-2 text-sm", tones[tone])}>
      {children}
    </div>
  );
}

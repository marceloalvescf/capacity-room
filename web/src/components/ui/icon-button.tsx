import { Tooltip } from "radix-ui";
import type { ComponentProps } from "react";
import { cn } from "@/lib/cn";
import { Button } from "./button";

export function IconButton({ label, className, ...rest }: ComponentProps<typeof Button> & { label: string }) {
  return (
    <Tooltip.Provider>
      <Tooltip.Root>
        <Tooltip.Trigger asChild>
          <Button aria-label={label} className={cn("w-9 px-0", className)} {...rest} />
        </Tooltip.Trigger>
        <Tooltip.Portal>
          <Tooltip.Content sideOffset={4} className="z-50 rounded-md bg-fg px-2 py-1 text-xs text-bg">
            {label}
          </Tooltip.Content>
        </Tooltip.Portal>
      </Tooltip.Root>
    </Tooltip.Provider>
  );
}

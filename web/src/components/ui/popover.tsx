import { Popover as RadixPopover } from "radix-ui";
import type { ComponentProps } from "react";
import { cn } from "@/lib/cn";

export const Popover = RadixPopover.Root;
export const PopoverTrigger = RadixPopover.Trigger;

export function PopoverContent({ className, ...rest }: ComponentProps<typeof RadixPopover.Content>) {
  return (
    <RadixPopover.Portal>
      <RadixPopover.Content
        sideOffset={4}
        className={cn("rounded-md border border-border bg-surface p-3 text-surface-fg shadow-md", className)}
        {...rest}
      />
    </RadixPopover.Portal>
  );
}

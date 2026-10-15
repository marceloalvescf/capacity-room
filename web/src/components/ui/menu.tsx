import { CheckIcon } from "@phosphor-icons/react";
import { DropdownMenu } from "radix-ui";
import type { ComponentProps } from "react";
import { cn } from "@/lib/cn";

export const Menu = DropdownMenu.Root;
export const MenuTrigger = DropdownMenu.Trigger;
export const MenuRadioGroup = DropdownMenu.RadioGroup;

export function MenuContent({ className, ...rest }: ComponentProps<typeof DropdownMenu.Content>) {
  return (
    <DropdownMenu.Portal>
      <DropdownMenu.Content
        align="end"
        sideOffset={4}
        className={cn("layer-motion z-50 min-w-40 rounded-lg border border-border bg-surface p-1 text-surface-fg shadow-float", className)}
        {...rest}
      />
    </DropdownMenu.Portal>
  );
}

export function MenuRadioItem({ className, children, ...rest }: ComponentProps<typeof DropdownMenu.RadioItem>) {
  return (
    <DropdownMenu.RadioItem
      className={cn(
        "flex min-h-9 cursor-pointer items-center gap-2 rounded-md px-2 text-sm outline-none",
        // The highlighted item takes the tint. Radix focuses it from the pointer and from the keyboard alike, and the
        // browser counts both as visible focus, so the ring is taken off the item under the pointer: only the item
        // reached by the keyboard keeps it, inset.
        "data-[highlighted]:bg-sidebar-active data-[highlighted]:text-sidebar-active-fg",
        "focus-visible:outline-solid focus-visible:outline-2 focus-visible:-outline-offset-2! focus-visible:outline-accent",
        "hover:outline-none!",
        className,
      )}
      {...rest}
    >
      <span className="inline-flex size-4 shrink-0 items-center justify-center">
        <DropdownMenu.ItemIndicator>
          <CheckIcon aria-hidden="true" weight="bold" className="size-4" />
        </DropdownMenu.ItemIndicator>
      </span>
      {children}
    </DropdownMenu.RadioItem>
  );
}

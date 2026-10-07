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
        className={cn("min-w-40 rounded-md border border-border bg-surface p-1 text-surface-fg shadow-md", className)}
        {...rest}
      />
    </DropdownMenu.Portal>
  );
}

export function MenuRadioItem({ className, children, ...rest }: ComponentProps<typeof DropdownMenu.RadioItem>) {
  return (
    <DropdownMenu.RadioItem
      className={cn(
        "flex min-h-9 cursor-pointer items-center gap-2 rounded px-2 text-sm outline-none",
        "data-[highlighted]:outline-solid data-[highlighted]:outline-2 data-[highlighted]:-outline-offset-2 data-[highlighted]:outline-accent",
        "data-[highlighted]:bg-sidebar-active data-[highlighted]:text-sidebar-active-fg",
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

import { Tooltip } from "radix-ui";
import { useCallback, useState, type ComponentProps } from "react";
import { cn } from "@/lib/cn";
import { Button } from "./button";

// A tooltip is drawn inside the landmark that holds its button, so that it
// belongs to a region of the page; a button outside every landmark (inside a
// layer portalled to the body) keeps the body.
const landmarks = "main, header, nav, aside, footer";

export function IconButton({ label, className, ...rest }: ComponentProps<typeof Button> & { label: string }) {
  const [container, setContainer] = useState<HTMLElement | undefined>(undefined);
  const place = useCallback((node: HTMLElement | null) => {
    setContainer(node?.closest<HTMLElement>(landmarks) ?? undefined);
  }, []);
  return (
    <Tooltip.Root>
      <Tooltip.Trigger asChild ref={place}>
        <Button aria-label={label} className={cn("w-9 px-0", className)} {...rest} />
      </Tooltip.Trigger>
      <Tooltip.Portal container={container}>
        <Tooltip.Content sideOffset={4} className="layer-motion z-50 rounded-md bg-fg px-2 py-1 text-xs text-bg">
          {label}
        </Tooltip.Content>
      </Tooltip.Portal>
    </Tooltip.Root>
  );
}

import type { ReactNode } from "react";
import { Dialog as Layer } from "radix-ui";

interface DialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  children: ReactNode;
  onCloseAutoFocus?: (event: Event) => void;
}

// A modal layer for a form; Escape and the overlay close it.
export function Dialog({ open, onOpenChange, title, children, onCloseAutoFocus }: DialogProps) {
  return (
    <Layer.Root open={open} onOpenChange={onOpenChange}>
      <Layer.Portal>
        <Layer.Overlay className="fixed inset-0 z-50 bg-fg/40" />
        <Layer.Content
          aria-describedby={undefined}
          onCloseAutoFocus={onCloseAutoFocus}
          className="fixed top-1/2 left-1/2 z-50 flex max-h-[calc(100vh-2rem)] w-[min(36rem,calc(100vw-2rem))] -translate-x-1/2 -translate-y-1/2 flex-col gap-4 overflow-y-auto rounded-md border border-border bg-surface p-6 text-surface-fg shadow-md"
        >
          <Layer.Title className="text-lg font-semibold">{title}</Layer.Title>
          {children}
        </Layer.Content>
      </Layer.Portal>
    </Layer.Root>
  );
}

import { AlertDialog } from "radix-ui";
import { Button } from "./button";

interface ConfirmDialogProps {
  open: boolean;
  title: string;
  body: string;
  confirm: string;
  cancel: string;
  busy?: boolean;
  // The caller closes the dialog; a confirm does not.
  onConfirm: () => void;
  onCloseAutoFocus?: (event: Event) => void;
  onOpenChange: (open: boolean) => void;
}

export function ConfirmDialog({
  open,
  title,
  body,
  confirm,
  cancel,
  busy = false,
  onConfirm,
  onCloseAutoFocus,
  onOpenChange,
}: ConfirmDialogProps) {
  return (
    <AlertDialog.Root open={open} onOpenChange={onOpenChange}>
      <AlertDialog.Portal>
        <AlertDialog.Overlay className="fixed inset-0 bg-fg/40" />
        <AlertDialog.Content
          onCloseAutoFocus={onCloseAutoFocus}
          className="fixed top-1/2 left-1/2 flex w-[min(28rem,calc(100vw-2rem))] -translate-x-1/2 -translate-y-1/2 flex-col gap-4 rounded-md border border-border bg-surface p-6 text-surface-fg shadow-md"
        >
          <AlertDialog.Title className="text-lg font-semibold">{title}</AlertDialog.Title>
          <AlertDialog.Description>{body}</AlertDialog.Description>
          <div className="flex justify-end gap-2">
            <AlertDialog.Cancel asChild>
              <Button>{cancel}</Button>
            </AlertDialog.Cancel>
            <AlertDialog.Action asChild>
              <Button
                variant="danger"
                disabled={busy}
                onClick={(event) => {
                  event.preventDefault();
                  onConfirm();
                }}
              >
                {confirm}
              </Button>
            </AlertDialog.Action>
          </div>
        </AlertDialog.Content>
      </AlertDialog.Portal>
    </AlertDialog.Root>
  );
}

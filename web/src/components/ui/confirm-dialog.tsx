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
    <AlertDialog.Root
      open={open}
      onOpenChange={(next) => {
        // A delete at work cannot be dismissed.
        if (!busy) {
          onOpenChange(next);
        }
      }}
    >
      <AlertDialog.Portal>
        <AlertDialog.Overlay className="scrim-motion fixed inset-0 z-50 bg-scrim" />
        <AlertDialog.Content
          onCloseAutoFocus={onCloseAutoFocus}
          className="dialog-motion fixed top-1/2 z-50 left-1/2 flex w-[min(28rem,calc(100vw-2rem))] -translate-x-1/2 -translate-y-1/2 flex-col gap-4 rounded-md border border-neatline bg-surface p-6 text-surface-fg"
        >
          <AlertDialog.Title className="text-lg font-semibold">{title}</AlertDialog.Title>
          <AlertDialog.Description>{body}</AlertDialog.Description>
          <div className="flex justify-end gap-2">
            <AlertDialog.Cancel asChild>
              <Button disabled={busy}>{cancel}</Button>
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

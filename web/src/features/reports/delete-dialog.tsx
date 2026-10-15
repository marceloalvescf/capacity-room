import { useState } from "react";
import { useTranslation } from "react-i18next";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";

export type DeleteTarget = { kind: "one"; id: string; profile: string; time: string; username: string } | { kind: "many"; ids: string[] };

interface DeleteDialogProps {
  target: DeleteTarget | null;
  busy: boolean;
  onConfirm: () => void;
  onCancel: () => void;
  onCloseAutoFocus: (event: Event) => void;
}

export function DeleteDialog({ target, busy, onConfirm, onCancel, onCloseAutoFocus }: DeleteDialogProps) {
  const { t } = useTranslation();
  // The last target stays after the dialog closes, so its title and body do not go blank while it fades.
  const [last, setLast] = useState(target);
  if (target !== null && target !== last) {
    setLast(target);
  }
  const shown = target ?? last;
  let title = "";
  let body = "";
  if (shown?.kind === "one") {
    title = t("delete.oneTitle", { profile: shown.profile });
    body = t("delete.oneBody", { time: shown.time, username: shown.username });
  } else if (shown?.kind === "many") {
    title = t("delete.manyTitle", { count: shown.ids.length });
    body = t("delete.manyBody");
  }
  return (
    <ConfirmDialog
      open={target !== null}
      title={title}
      body={body}
      confirm={t("delete.confirm")}
      cancel={t("delete.cancel")}
      busy={busy}
      onConfirm={onConfirm}
      onCloseAutoFocus={onCloseAutoFocus}
      onOpenChange={(open) => {
        if (!open) {
          onCancel();
        }
      }}
    />
  );
}

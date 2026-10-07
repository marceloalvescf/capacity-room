import { useTranslation } from "react-i18next";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";

export type DeleteTarget = { kind: "one"; id: string } | { kind: "many"; ids: string[] };

interface DeleteDialogProps {
  target: DeleteTarget | null;
  busy: boolean;
  onConfirm: () => void;
  onCancel: () => void;
  onCloseAutoFocus: (event: Event) => void;
}

export function DeleteDialog({ target, busy, onConfirm, onCancel, onCloseAutoFocus }: DeleteDialogProps) {
  const { t } = useTranslation();
  let title = "";
  let body = "";
  if (target?.kind === "one") {
    title = t("delete.oneTitle");
    body = t("delete.oneBody");
  } else if (target?.kind === "many") {
    title = t("delete.manyTitle", { count: target.ids.length });
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

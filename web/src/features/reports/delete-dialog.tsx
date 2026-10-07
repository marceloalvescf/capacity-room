import { useTranslation } from "react-i18next";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { formatTime } from "@/lib/format";
import type { ReportSummary } from "./api";

export type DeleteTarget = { kind: "one"; report: ReportSummary; profile: string } | { kind: "many"; count: number };

interface DeleteDialogProps {
  target: DeleteTarget | null;
  onConfirm: () => void;
  onCancel: () => void;
}

export function DeleteDialog({ target, onConfirm, onCancel }: DeleteDialogProps) {
  const { t } = useTranslation();
  let title = "";
  let body = "";
  if (target?.kind === "one") {
    title = t("delete.oneTitle");
    body = t("delete.oneBody", {
      profile: target.profile,
      time: formatTime(target.report.created_at),
      username: target.report.owner_username,
    });
  } else if (target?.kind === "many") {
    title = t("delete.manyTitle", { count: target.count });
    body = t("delete.manyBody");
  }
  return (
    <ConfirmDialog
      open={target !== null}
      title={title}
      body={body}
      confirm={t("delete.confirm")}
      cancel={t("delete.cancel")}
      onConfirm={onConfirm}
      onOpenChange={(open) => {
        if (!open) {
          onCancel();
        }
      }}
    />
  );
}

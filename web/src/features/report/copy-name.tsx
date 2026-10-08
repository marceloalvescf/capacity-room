import { CopyIcon } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { useAnnounce } from "@/app/live-region";
import { IconButton } from "@/components/ui/icon-button";

// A name in monospace with a control that copies it to the clipboard and announces the outcome.
export function CopyName({ name }: { name: string }) {
  const { t } = useTranslation();
  const announce = useAnnounce();
  return (
    <span className="inline-flex items-center gap-2">
      <span className="font-mono">{name}</span>
      <IconButton
        label={t("report.copy", { name })}
        variant="secondary"
        className="h-7 min-h-7 w-7 border-transparent bg-transparent"
        onClick={() => {
          navigator.clipboard.writeText(name).then(
            () => {
              announce(t("report.copied", { name }));
            },
            () => {
              announce(t("report.copyFailed"));
            },
          );
        }}
      >
        <CopyIcon aria-hidden="true" className="size-4" />
      </IconButton>
    </span>
  );
}

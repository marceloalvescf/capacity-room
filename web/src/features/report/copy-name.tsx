import { CopyIcon } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { useAnnounce } from "@/app/live-region";
import { IconButton } from "@/components/ui/icon-button";

// Writes a text to the clipboard and announces the outcome under a name. A missing clipboard (a page that is not a secure
// context) fails like a refused write.
export function useCopy(): (text: string, name: string) => Promise<void> {
  const { t } = useTranslation();
  const announce = useAnnounce();
  return async (text, name) => {
    try {
      await navigator.clipboard.writeText(text);
      announce(t("report.copied", { name }));
    } catch {
      announce(t("report.copyFailed"));
    }
  };
}

// A name in monospace with a control that copies it to the clipboard and announces the outcome.
export function CopyName({ name }: { name: string }) {
  const { t } = useTranslation();
  const copy = useCopy();
  return (
    <span className="inline-flex items-center gap-2">
      <span className="font-mono [overflow-wrap:anywhere]">{name}</span>
      <IconButton
        label={t("report.copy", { name })}
        variant="secondary"
        className="h-7 min-h-7 w-7 border-transparent bg-transparent"
        onClick={() => {
          void copy(name, name);
        }}
      >
        <CopyIcon aria-hidden="true" className="size-4" />
      </IconButton>
    </span>
  );
}

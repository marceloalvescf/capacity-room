import { CaretDownIcon } from "@phosphor-icons/react";
import { useId, useState } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Field } from "@/components/ui/field";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { isLanguage } from "@/i18n/language";
import { quickRanges, parsePeriod, periodText } from "./period";

interface PeriodPickerProps {
  value: string;
  // The periods the profile permits; empty means any.
  permitted: readonly string[];
  error?: string | undefined;
  onChange: (period: string) => void;
}

const choice =
  "min-h-9 px-3 text-left text-sm text-surface-fg hover:underline aria-pressed:bg-accent aria-pressed:text-accent-fg " +
  "focus-visible:outline-2 focus-visible:outline-offset-[-2px] focus-visible:outline-accent";

function Custom({ initial, onApply }: { initial: string; onApply: (period: string) => void }) {
  const { t } = useTranslation();
  const [text, setText] = useState(initial);
  const parsed = parsePeriod(text);
  return (
    <form
      noValidate
      className="flex flex-col gap-2 p-3"
      onSubmit={(event) => {
        // The portal's events reach the report form through React; they stop here.
        event.preventDefault();
        event.stopPropagation();
        if (parsed !== null) {
          onApply(parsed);
        }
      }}
    >
      <h3 className="text-xs font-semibold text-muted-fg uppercase">{t("period.custom.title")}</h3>
      <Field
        label={t("period.custom.label")}
        className="font-mono"
        value={text}
        hint={t("period.custom.hint")}
        error={parsed === null ? t("period.invalid") : undefined}
        onChange={(event) => {
          setText(event.target.value);
        }}
      />
      <Button type="submit" variant="primary" className="self-start" disabled={parsed === null}>
        {t("period.apply")}
      </Button>
    </form>
  );
}

export function PeriodPicker({ value, permitted, error, onChange }: PeriodPickerProps) {
  const { t, i18n } = useTranslation();
  const [open, setOpen] = useState(false);
  const labelId = useId();
  const buttonId = useId();
  const errorId = useId();
  const language = isLanguage(i18n.language) ? i18n.language : "en";
  const restricted = permitted.length > 0;
  const ranges = restricted ? permitted : quickRanges;

  const pick = (period: string) => {
    onChange(period);
    setOpen(false);
  };

  return (
    <div className="flex flex-col gap-1">
      <span id={labelId} className="text-sm font-medium">
        {t("generate.period")}
      </span>
      <Popover open={open} onOpenChange={setOpen}>
        <PopoverTrigger asChild>
          <button
            id={buttonId}
            type="button"
            aria-labelledby={`${labelId} ${buttonId}`}
            aria-describedby={error === undefined ? undefined : errorId}
            className="flex min-h-9 items-center justify-between gap-2 rounded-md border border-input-border bg-surface px-3 text-sm text-surface-fg focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent"
          >
            {periodText(t, language, value)}
            <CaretDownIcon aria-hidden="true" className="size-4" />
          </button>
        </PopoverTrigger>
        <PopoverContent align="start" className="w-[min(32rem,calc(100vw-2rem))] p-0">
          <div className={restricted ? "flex flex-col" : "grid sm:grid-cols-[1fr_12rem]"}>
            {restricted ? (
              <p className="p-3 text-sm text-muted-fg">{t("period.restricted")}</p>
            ) : (
              <Custom initial={value} onApply={pick} />
            )}
            <div className="flex flex-col py-2 sm:border-l sm:border-border">
              {ranges.map((range) => (
                <button
                  key={range}
                  type="button"
                  aria-pressed={range === value}
                  className={choice}
                  onClick={() => {
                    pick(range);
                  }}
                >
                  {periodText(t, language, range)}
                </button>
              ))}
            </div>
          </div>
        </PopoverContent>
      </Popover>
      {error === undefined ? null : (
        <p id={errorId} className="text-sm text-critical">
          {error}
        </p>
      )}
    </div>
  );
}

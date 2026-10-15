import { CalendarBlankIcon, CaretDownIcon } from "@phosphor-icons/react";
import { DayPicker, type ClassNames } from "@daypicker/react";
import { enUS, ptBR } from "@daypicker/react/locale";
import { useId, useRef, useState, type Ref } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Field } from "@/components/ui/field";
import { IconButton } from "@/components/ui/icon-button";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Segmented } from "@/components/ui/segmented";
import { isLanguage, type Language } from "@/i18n/language";
import { cn } from "@/lib/cn";
import {
  calendarRanges,
  checkRange,
  parsePeriod,
  parseWallTime,
  periodText,
  quickRanges,
  resolveCalendar,
  type PeriodValue,
  type RangeProblem,
} from "./period";

interface PeriodPickerProps {
  value: PeriodValue;
  // The report's time zone, which reads every wall time of a range.
  zone: string;
  // The periods the profile permits; empty means any.
  permitted: readonly string[];
  error?: string | undefined;
  triggerRef?: Ref<HTMLButtonElement>;
  onChange: (value: PeriodValue) => void;
}

type Mode = "length" | "range";
type Side = "from" | "to";

const choice =
  "min-h-9 px-3 text-left text-sm text-surface-fg not-aria-pressed:hover:bg-hover aria-pressed:bg-action aria-pressed:text-action-fg " +
  "focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent";

const calendarClasses: Partial<ClassNames> = {
  months: "relative",
  month: "flex flex-col gap-1",
  month_caption: "flex h-8 items-center justify-center",
  caption_label: "text-sm font-semibold",
  nav: "absolute inset-x-0 top-0 flex items-center justify-between",
  button_previous:
    "inline-flex size-8 items-center justify-center rounded-md text-surface-fg hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent",
  button_next:
    "inline-flex size-8 items-center justify-center rounded-md text-surface-fg hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent",
  chevron: "size-4 fill-current",
  month_grid: "w-full border-collapse",
  weekday: "p-0.5 text-center text-xs font-normal text-muted-fg",
  week: "[&>td]:py-px",
  day: "p-0 text-center",
  day_button:
    "h-8 w-full rounded-md font-mono text-sm focus-visible:relative focus-visible:z-10 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent",
  outside: "text-muted-fg",
  selected: "rounded-md bg-action text-action-fg data-[outside=true]:text-action-fg",
};

const problemKeys: Record<RangeProblem, string> = {
  invalid: "period.range.invalid",
  order: "period.range.order",
  future: "period.range.future",
};

const language = (code: string): Language => (isLanguage(code) ? code : "en");

function dayOf(wall: string): Date | null {
  const parsed = parseWallTime(wall);
  if (parsed === null) {
    return null;
  }
  return new Date(Number(parsed.slice(0, 4)), Number(parsed.slice(5, 7)) - 1, Number(parsed.slice(8, 10)));
}

function twoDigits(n: number): string {
  return String(n).padStart(2, "0");
}

// The wall text with the date of a picked day and the time it had.
function withDay(wall: string, day: Date): string {
  const time = parseWallTime(wall)?.slice(11) ?? "00:00";
  return `${String(day.getFullYear()).padStart(4, "0")}-${twoDigits(day.getMonth() + 1)}-${twoDigits(day.getDate())} ${time}`;
}

function Length({ initial, onEdit, onApply }: { initial: string; onEdit: () => void; onApply: (period: string) => void }) {
  const { t } = useTranslation();
  const [text, setText] = useState(initial);
  const parsed = parsePeriod(text);
  return (
    <form
      noValidate
      className="flex flex-col gap-2"
      onSubmit={(event) => {
        // The portal's events reach the report form through React; they stop here.
        event.preventDefault();
        event.stopPropagation();
        if (parsed !== null) {
          onApply(parsed);
        }
      }}
    >
      <Field
        label={t("period.custom.label")}
        className="font-mono"
        value={text}
        hint={t("period.custom.hint")}
        error={parsed === null ? t("period.invalid") : undefined}
        onChange={(event) => {
          setText(event.target.value);
          onEdit();
        }}
      />
      <Button type="submit" variant="primary" className="self-start" disabled={parsed === null}>
        {t("period.apply")}
      </Button>
    </form>
  );
}

interface WallFieldProps {
  label: string;
  pickLabel: string;
  value: string;
  problem: string | undefined;
  hintId: string | undefined;
  open: boolean;
  buttonRef: Ref<HTMLButtonElement>;
  inputRef: Ref<HTMLInputElement>;
  onChange: (text: string) => void;
  onToggle: () => void;
}

function WallField({ label, pickLabel, value, problem, hintId, open, buttonRef, inputRef, onChange, onToggle }: WallFieldProps) {
  const id = useId();
  const errorId = `${id}-error`;
  const described = [hintId, problem === undefined ? undefined : errorId].filter((part) => part !== undefined).join(" ");
  return (
    <div className="flex flex-col gap-1">
      <label htmlFor={id} className="text-sm font-medium">
        {label}
      </label>
      <div className="flex gap-2">
        <input
          id={id}
          ref={inputRef}
          value={value}
          aria-invalid={problem === undefined ? undefined : true}
          aria-describedby={described === "" ? undefined : described}
          className="min-h-9 min-w-0 flex-1 rounded-md border border-input-border bg-surface px-3 font-mono text-sm text-surface-fg focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent"
          onChange={(event) => {
            onChange(event.target.value);
          }}
        />
        <IconButton ref={buttonRef} label={pickLabel} aria-expanded={open} onClick={onToggle}>
          <CalendarBlankIcon aria-hidden="true" className="size-[18px]" />
        </IconButton>
      </div>
      {problem === undefined ? null : (
        <p id={errorId} className="text-sm text-critical">
          {problem}
        </p>
      )}
    </div>
  );
}

interface RangeProps {
  initial: { start: string; end: string };
  zone: string;
  calendar: Side | null;
  buttons: Record<Side, React.RefObject<HTMLButtonElement | null>>;
  onCalendar: (side: Side | null) => void;
  onEdit: () => void;
  onApply: (start: string, end: string) => void;
}

function Range({ initial, zone, calendar, buttons, onCalendar, onEdit, onApply }: RangeProps) {
  const { t, i18n } = useTranslation();
  const hintId = useId();
  const [text, setText] = useState<Record<Side, string>>({ from: initial.start, to: initial.end });
  const inputs = { from: useRef<HTMLInputElement>(null), to: useRef<HTMLInputElement>(null) };
  const [problems, setProblems] = useState<{ start?: RangeProblem; end?: RangeProblem }>({});
  const from = dayOf(text.from);
  const to = dayOf(text.to);
  const ordered = from !== null && to !== null && from.getTime() <= to.getTime();
  const between = (day: Date): boolean =>
    ordered && day.getTime() > from.getTime() && day.getTime() < to.getTime();
  const other = (side: Side): Date | null => (side === "from" ? to : from);

  const edit = (side: Side, next: string) => {
    setText((current) => ({ ...current, [side]: next }));
    setProblems({});
    onEdit();
  };

  const calendarOf = (side: Side) => {
    const own = side === "from" ? from : to;
    const far = other(side);
    return (
      <DayPicker
        mode="single"
        required
        showOutsideDays
        weekStartsOn={1}
        locale={language(i18n.language) === "pt-BR" ? ptBR : enUS}
        labels={{
          labelPrevious: () => t("period.range.previous"),
          labelNext: () => t("period.range.next"),
          labelNav: () => t("period.range.months"),
        }}
        classNames={calendarClasses}
        selected={own ?? undefined}
        defaultMonth={own ?? far ?? new Date()}
        modifiers={{
          between,
          otherEnd: far === null ? [] : [far],
        }}
        modifiersClassNames={{
          between:
            "rounded-none bg-info-surface text-info-surface-fg data-[outside=true]:bg-transparent data-[outside=true]:text-muted-fg",
          otherEnd: "rounded-md bg-action text-action-fg data-[outside=true]:text-action-fg",
        }}
        onSelect={(day) => {
          edit(side, withDay(text[side], day));
          onCalendar(null);
          buttons[side].current?.focus();
        }}
      />
    );
  };

  const problemText = (problem: RangeProblem | undefined): string | undefined =>
    problem === undefined ? undefined : t(problemKeys[problem]);

  const side = (which: Side, problem: RangeProblem | undefined, withHint: boolean) => (
    <>
      <WallField
        label={t(which === "from" ? "period.range.from" : "period.range.to")}
        pickLabel={t(which === "from" ? "period.range.pickFrom" : "period.range.pickTo")}
        value={text[which]}
        problem={problemText(problem)}
        hintId={withHint ? hintId : undefined}
        open={calendar === which}
        buttonRef={buttons[which]}
        inputRef={inputs[which]}
        onChange={(next) => {
          edit(which, next);
        }}
        onToggle={() => {
          onCalendar(calendar === which ? null : which);
        }}
      />
      {calendar === which ? <div className="rounded-lg border border-border p-2">{calendarOf(which)}</div> : null}
    </>
  );

  return (
    <form
      noValidate
      className="flex flex-col gap-3"
      onSubmit={(event) => {
        event.preventDefault();
        event.stopPropagation();
        const found = checkRange(text.from, text.to, zone, new Date());
        if (found.start !== undefined || found.end !== undefined) {
          setProblems(found);
          inputs[found.start === undefined ? "to" : "from"].current?.focus();
          return;
        }
        onApply(parseWallTime(text.from) as string, parseWallTime(text.to) as string);
      }}
    >
      {side("from", problems.start, false)}
      {side("to", problems.end, true)}
      <p id={hintId} className="text-xs text-muted-fg">
        {t("period.range.hint", { zone })}
      </p>
      <Button type="submit" variant="primary" className="self-start">
        {t("period.apply")}
      </Button>
    </form>
  );
}

const modeOf = (value: PeriodValue): Mode => (value.kind === "length" ? "length" : "range");

function initialRange(value: PeriodValue, zone: string): { start: string; end: string } {
  if (value.kind === "range") {
    return { start: value.start, end: value.end };
  }
  return resolveCalendar(value.kind === "calendar" ? value.name : "yesterday", zone, new Date());
}

export function PeriodPicker({ value, zone, permitted, error, triggerRef, onChange }: PeriodPickerProps) {
  const { t, i18n } = useTranslation();
  const [open, setOpen] = useState(false);
  const [mode, setMode] = useState<Mode>("length");
  const [calendar, setCalendar] = useState<Side | null>(null);
  const [edited, setEdited] = useState(false);
  const [range, setRange] = useState({ start: "", end: "" });
  const buttons = { from: useRef<HTMLButtonElement>(null), to: useRef<HTMLButtonElement>(null) };
  const labelId = useId();
  const buttonId = useId();
  const errorId = useId();
  const lang = language(i18n.language);
  const restricted = permitted.length > 0;
  const unedited = mode === modeOf(value) && !edited;
  const lengths = restricted ? permitted : quickRanges;

  const pick = (next: PeriodValue) => {
    onChange(next);
    setOpen(false);
  };

  const openChanged = (next: boolean) => {
    if (next) {
      setMode(modeOf(value));
      setEdited(false);
      setRange(initialRange(value, zone));
      setCalendar(null);
    }
    setOpen(next);
  };

  const markEdited = () => {
    setEdited(true);
  };

  const switchMode = (next: Mode) => {
    setCalendar(null);
    setMode(next);
  };

  return (
    <div className="flex flex-col gap-1">
      <span id={labelId} className="text-sm font-medium">
        {t("generate.period")}
      </span>
      <Popover open={open} onOpenChange={openChanged}>
        <PopoverTrigger asChild>
          <button
            id={buttonId}
            ref={triggerRef}
            type="button"
            aria-labelledby={`${labelId} ${buttonId}`}
            data-invalid={error === undefined ? undefined : true}
            aria-describedby={error === undefined ? undefined : errorId}
            className="flex min-h-9 items-center justify-between gap-2 rounded-md border border-input-border bg-surface px-3 text-sm text-surface-fg focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent"
          >
            {periodText(t, lang, value)}
            <CaretDownIcon aria-hidden="true" className="size-4" />
          </button>
        </PopoverTrigger>
        <PopoverContent
          side="bottom"
          align="start"
          aria-labelledby={labelId}
          collisionPadding={24}
          className={cn(
            "max-h-(--radix-popover-content-available-height) overflow-y-auto p-0",
            restricted ? "w-[min(36rem,calc(100vw-2rem))]" : "w-[min(46rem,calc(100vw-2rem))]",
          )}
          onEscapeKeyDown={(event) => {
            if (calendar !== null) {
              event.preventDefault();
              setCalendar(null);
              buttons[calendar].current?.focus();
            }
          }}
        >
          <div className={restricted ? "flex flex-col" : "grid sm:grid-cols-[1fr_24rem]"}>
            {restricted ? (
              <p className="p-3 text-sm text-muted-fg">{t("period.restricted")}</p>
            ) : (
              <div className="flex flex-col gap-3 p-3">
                <Segmented
                  legend={t("generate.period")}
                  hideLegend
                  name="period-mode"
                  value={mode}
                  options={[
                    { value: "length", label: t("period.mode.length") },
                    { value: "range", label: t("period.mode.range") },
                  ]}
                  onChange={switchMode}
                />
                {mode === "length" ? (
                  <Length
                    initial={value.kind === "length" ? value.period : "24h"}
                    onEdit={markEdited}
                    onApply={(period) => {
                      pick({ kind: "length", period });
                    }}
                  />
                ) : (
                  <Range
                    initial={range}
                    zone={zone}
                    calendar={calendar}
                    buttons={buttons}
                    onCalendar={setCalendar}
                    onEdit={markEdited}
                    onApply={(start, end) => {
                      pick({ kind: "range", start, end });
                    }}
                  />
                )}
              </div>
            )}
            <div className="px-1 py-2 sm:border-l sm:border-border">
              <div className={restricted ? "flex flex-col" : "grid grid-cols-2 gap-x-1"}>
                <div className="flex flex-col">
                  {lengths.map((length) => (
                    <button
                      key={length}
                      type="button"
                      aria-pressed={unedited && value.kind === "length" && value.period === length}
                      className={choice}
                      onClick={() => {
                        pick({ kind: "length", period: length });
                      }}
                    >
                      {periodText(t, lang, { kind: "length", period: length })}
                    </button>
                  ))}
                </div>
                {restricted ? null : (
                  <div className="flex flex-col">
                    {calendarRanges.map((name) => (
                      <button
                        key={name}
                        type="button"
                        aria-pressed={unedited && value.kind === "calendar" && value.name === name}
                        className={choice}
                        onClick={() => {
                          pick({ kind: "calendar", name });
                        }}
                      >
                        {periodText(t, lang, { kind: "calendar", name })}
                      </button>
                    ))}
                  </div>
                )}
              </div>
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

import { useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { ApiFailure } from "@/api/client";
import { errorText, toError } from "@/api/errors";
import { languageValues } from "@/api/schema.gen";
import { StatusMark } from "@/components/status-mark";
import { Alert } from "@/components/ui/alert";
import { Button, buttonStyles } from "@/components/ui/button";
import { isLanguage, type Language } from "@/i18n/language";
import { reportsKey, useRequestPDF, type Report } from "./api";
import { pdfMark } from "./marks";

// The PDF of the interface language comes first, the other below it.
function ordered(current: string): Language[] {
  const first = isLanguage(current) ? current : "en";
  return [first, ...languageValues.filter((language) => language !== first)];
}

export function PdfBlock({ report, canRequest }: { report: Report; canRequest: boolean }) {
  const { t, i18n } = useTranslation();
  const queryClient = useQueryClient();
  const request = useRequestPDF(report.id);
  const [problem, setProblem] = useState<string | null>(null);

  async function generate(language: Language) {
    setProblem(null);
    try {
      await request.mutateAsync(language);
    } catch (thrown) {
      if (thrown instanceof ApiFailure && thrown.code === "PDF_ALREADY_READY") {
        await queryClient.invalidateQueries({ queryKey: [...reportsKey, report.id], exact: true });
        return;
      }
      setProblem(errorText(t, toError(thrown)));
    }
  }

  return (
    <section className="flex flex-col gap-3">
      <h2 className="text-base font-semibold">{t("progress.pdf")}</h2>
      {problem === null ? null : <Alert tone="error">{problem}</Alert>}
      <div className="grid grid-cols-[auto_1fr_auto] items-center gap-x-3 divide-y divide-border">
        {ordered(i18n.language).map((language) => {
          const state = report.pdfs.find((candidate) => candidate.language === language);
          const label = `pdf-${language}`;
          const generateButton = (name: string) =>
            canRequest ? (
              <Button
                disabled={request.isPending}
                onClick={() => {
                  void generate(language);
                }}
              >
                {name}
              </Button>
            ) : null;
          return (
            <div
              key={language}
              role="group"
              aria-labelledby={label}
              className="col-span-3 grid grid-cols-subgrid items-center py-2 text-sm first:pt-0 last:pb-0"
            >
              <span id={label} className="font-medium">
                {t(`language.${language}`)}
              </span>
              {state === undefined ? (
                <>
                  <span className="text-muted-fg">{t("reports.notGenerated")}</span>
                  <span>{generateButton(t("progress.generatePdf"))}</span>
                </>
              ) : (
                <>
                  <span className="flex flex-wrap items-center gap-x-3">
                    <StatusMark status={pdfMark(state.status)} label={t(`pdfStatus.${state.status}`)} />
                    {state.status === "failed" ? (
                      <span>{state.failure_code === null ? t("error.general") : t(`pdfFailure.${state.failure_code}`)}</span>
                    ) : null}
                  </span>
                  <span>
                    {state.status === "ready" ? (
                      <a href={`/api/v1/reports/${report.id}/pdf?lang=${language}`} className={buttonStyles("primary")}>
                        {t("progress.download")}
                      </a>
                    ) : null}
                    {state.status === "failed" ? generateButton(t("progress.retryPdf")) : null}
                  </span>
                </>
              )}
            </div>
          );
        })}
      </div>
    </section>
  );
}

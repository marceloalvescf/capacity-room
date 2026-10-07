import { zodResolver } from "@hookform/resolvers/zod";
import { useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { useForm } from "react-hook-form";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router";
import { z } from "zod";
import { useApi } from "@/api/context";
import { ApiFailure } from "@/api/client";
import { errorText, toError } from "@/api/errors";
import { languageValues } from "@/api/schema.gen";
import { useScreenTitle } from "@/app/title";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Field } from "@/components/ui/field";
import { isLanguage } from "@/i18n/language";
import { usePrincipal } from "@/session/session";
import { useChangeLanguage } from "@/session/use-change-language";

const newPassword = z
  .string()
  .min(1, "validation.required")
  .min(12, "validation.passwordShort")
  .max(1024, "validation.passwordLong");

const schema = z
  .object({
    current: z.string().min(1, "validation.required").max(1024, "validation.passwordLong"),
    next: newPassword,
    repeat: z.string().min(1, "validation.required"),
  })
  .refine((values) => values.next === values.repeat, {
    path: ["repeat"],
    message: "validation.passwordsDiffer",
  });

type Values = z.infer<typeof schema>;

const version = import.meta.env.VITE_KCR_VERSION;

export function AccountPage() {
  const { t } = useTranslation();
  const api = useApi();
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const principal = usePrincipal();
  const changeLanguage = useChangeLanguage();
  const [failure, setFailure] = useState<string | null>(null);
  const {
    register,
    handleSubmit,
    setError,
    formState: { errors, isSubmitting },
  } = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: { current: "", next: "", repeat: "" },
  });
  useScreenTitle(t("account.title"));

  if (principal === null) {
    return null;
  }

  async function submit(values: Values) {
    setFailure(null);
    try {
      await api.PATCH("/account", { body: { current_password: values.current, password: values.next } });
    } catch (thrown) {
      const error = toError(thrown);
      if (error instanceof ApiFailure && error.code === "INVALID_CREDENTIALS") {
        setError("current", { message: "error.INVALID_CREDENTIALS" });
      } else {
        setFailure(errorText(t, error));
      }
      return;
    }
    queryClient.clear();
    void navigate("/login?changed=1");
  }

  const fieldError = (message: string | undefined) => (message === undefined ? undefined : t(message));

  return (
    <div className="flex max-w-xl flex-col gap-8">
      <div className="flex flex-col gap-2">
        <h1 className="text-xl font-semibold">{t("account.title")}</h1>
        <p>{t("account.signedInAs", { username: principal.username, role: t(`role.${principal.role}`) })}</p>
      </div>

      <fieldset className="flex flex-col gap-2">
        <legend className="mb-2 text-lg font-semibold">{t("account.language.title")}</legend>
        {languageValues.map((language) => (
          <label key={language} className="flex min-h-9 items-center gap-2 text-sm">
            <input
              type="radio"
              name="language"
              value={language}
              checked={principal.language === language}
              onChange={(event) => {
                if (isLanguage(event.target.value)) {
                  void changeLanguage(event.target.value);
                }
              }}
            />
            {t(`language.${language}`, { lng: language })}
          </label>
        ))}
      </fieldset>

      <section className="flex flex-col gap-4">
        <h2 className="text-lg font-semibold">{t("account.password.title")}</h2>
        {failure === null ? null : <Alert tone="error">{failure}</Alert>}
        <form
          noValidate
          className="flex flex-col gap-4"
          onSubmit={(event) => {
            void handleSubmit(submit)(event);
          }}
        >
          <Field
            label={t("account.password.current")}
            type="password"
            autoComplete="current-password"
            error={fieldError(errors.current?.message)}
            {...register("current")}
          />
          <Field
            label={t("account.password.new")}
            type="password"
            autoComplete="new-password"
            hint={t("account.password.hint")}
            error={fieldError(errors.next?.message)}
            {...register("next")}
          />
          <Field
            label={t("account.password.repeat")}
            type="password"
            autoComplete="new-password"
            error={fieldError(errors.repeat?.message)}
            {...register("repeat")}
          />
          <Button type="submit" variant="primary" disabled={isSubmitting}>
            {isSubmitting ? t("account.password.submitting") : t("account.password.submit")}
          </Button>
        </form>
      </section>

      <p className="font-mono text-sm text-muted-fg">{t("app.version", { version })}</p>
    </div>
  );
}

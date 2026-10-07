import { zodResolver } from "@hookform/resolvers/zod";
import { useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";
import { useForm } from "react-hook-form";
import { useTranslation } from "react-i18next";
import { Navigate, useSearchParams } from "react-router";
import { z } from "zod";
import { useApi } from "@/api/context";
import { errorText, toError } from "@/api/errors";
import { Brand } from "@/app/brand";
import { LanguageMenu } from "@/app/language-menu";
import { useScreenTitle } from "@/app/title";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Field } from "@/components/ui/field";
import { isSignedIn, safeNext, sessionKey, useSession } from "@/session/session";

export const schema = z.object({
  username: z.string().min(1, "validation.required"),
  password: z.string().min(1, "validation.required").max(1024, "validation.passwordLong"),
});

type Values = z.infer<typeof schema>;

export function LoginPage() {
  const { t } = useTranslation();
  const api = useApi();
  const queryClient = useQueryClient();
  const [params] = useSearchParams();
  const { data } = useSession();
  const [failure, setFailure] = useState<string | null>(null);
  const failureRef = useRef<HTMLDivElement>(null);
  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<Values>({ resolver: zodResolver(schema), defaultValues: { username: "", password: "" } });
  useScreenTitle(t("login.title"));

  useEffect(() => {
    if (failure !== null) {
      failureRef.current?.focus();
    }
  }, [failure]);

  if (data !== undefined && isSignedIn(data)) {
    return <Navigate to={safeNext(params.get("next"))} replace />;
  }

  async function submit(values: Values) {
    setFailure(null);
    try {
      const { data: session } = await api.POST("/auth/login", { body: values });
      if (session !== undefined) {
        queryClient.setQueryData(sessionKey, session);
      }
    } catch (thrown) {
      setFailure(errorText(t, toError(thrown)));
    }
  }

  const fieldError = (message: string | undefined) => (message === undefined ? undefined : t(message));

  return (
    <div className="min-h-screen bg-bg text-fg">
      <header className="flex items-center justify-between gap-4 border-b border-border px-6 py-3">
        <Brand />
        <LanguageMenu />
      </header>
      <main id="main" tabIndex={-1} className="flex justify-center p-6 outline-none">
        <div className="flex w-full max-w-sm flex-col gap-4 rounded-md border border-border bg-surface p-6 text-surface-fg">
          <h1 className="text-xl font-semibold">{t("login.title")}</h1>
          {params.get("changed") === "1" ? <Alert tone="notice">{t("login.passwordChanged")}</Alert> : null}
          {params.get("ended") === "1" ? <Alert tone="notice">{t("login.sessionEnded")}</Alert> : null}
          {failure === null ? null : (
            <Alert tone="error" ref={failureRef}>
              {failure}
            </Alert>
          )}
          <form
            noValidate
            className="flex flex-col gap-4"
            onSubmit={(event) => {
              void handleSubmit(submit)(event);
            }}
          >
            <Field
              label={t("login.username")}
              autoComplete="username"
              error={fieldError(errors.username?.message)}
              {...register("username")}
            />
            <Field
              label={t("login.password")}
              type="password"
              autoComplete="current-password"
              error={fieldError(errors.password?.message)}
              {...register("password")}
            />
            <Button type="submit" variant="primary" disabled={isSubmitting}>
              {isSubmitting ? t("login.submitting") : t("login.submit")}
            </Button>
          </form>
        </div>
      </main>
    </div>
  );
}

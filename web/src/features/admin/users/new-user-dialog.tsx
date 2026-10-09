import { zodResolver } from "@hookform/resolvers/zod";
import { useState } from "react";
import { useForm } from "react-hook-form";
import { useTranslation } from "react-i18next";
import { z } from "zod";
import { ApiFailure } from "@/api/client";
import { errorText, toError } from "@/api/errors";
import { languageValues, roleValues } from "@/api/schema.gen";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Dialog } from "@/components/ui/dialog";
import { Field } from "@/components/ui/field";
import { Select } from "@/components/ui/select";
import { newPassword } from "@/features/account/account-page";
import { usePrincipal } from "@/session/session";
import { useCreateUser, type User } from "./api";

const schema = z.object({
  username: z.string().min(1, "validation.required"),
  password: newPassword,
  role: z.enum(["administrator", "operator", "reader"]),
  language: z.enum(["pt-BR", "en"]),
});
type Values = z.infer<typeof schema>;

function NewUserForm({ onCancel, onCreated }: { onCancel: () => void; onCreated: (user: User) => void }) {
  const { t } = useTranslation();
  const principal = usePrincipal();
  const create = useCreateUser();
  const [failure, setFailure] = useState<string | null>(null);
  const {
    register,
    handleSubmit,
    setError,
    formState: { errors },
  } = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: { username: "", password: "", role: "reader", language: principal?.language ?? "en" },
  });

  async function submit(values: Values) {
    setFailure(null);
    try {
      onCreated(await create.mutateAsync(values));
    } catch (thrown) {
      const error = toError(thrown);
      const code = error instanceof ApiFailure ? error.code : null;
      if (code === "USERNAME_TAKEN" || code === "INVALID_USERNAME") {
        setError("username", { message: `error.${code}` });
      } else if (code === "PASSWORD_TOO_SHORT" || code === "PASSWORD_TOO_LONG") {
        setError("password", { message: `error.${code}` });
      } else {
        setFailure(errorText(t, error));
      }
    }
  }

  const fieldError = (message: string | undefined) => (message === undefined ? undefined : t(message));

  return (
    <form
      noValidate
      className="flex flex-col gap-4"
      onSubmit={(event) => {
        void handleSubmit(submit)(event);
      }}
    >
      {failure === null ? null : <Alert tone="error">{failure}</Alert>}
      <Field
        label={t("admin.users.username")}
        autoComplete="off"
        className="font-mono"
        error={fieldError(errors.username?.message)}
        {...register("username")}
      />
      <Field
        label={t("admin.users.passwordLabel")}
        type="password"
        autoComplete="new-password"
        hint={t("admin.users.passwordHint")}
        error={fieldError(errors.password?.message)}
        {...register("password")}
      />
      <Select label={t("admin.users.roleLabel")} {...register("role")}>
        {roleValues.map((value) => (
          <option key={value} value={value}>
            {t(`admin.users.role.${value}`)}
          </option>
        ))}
      </Select>
      <Select label={t("admin.users.languageLabel")} {...register("language")}>
        {languageValues.map((value) => (
          <option key={value} value={value}>
            {t(`language.${value}`, { lng: value })}
          </option>
        ))}
      </Select>
      <div className="flex justify-end gap-2">
        <Button onClick={onCancel}>{t("admin.users.cancel")}</Button>
        <Button type="submit" variant="primary" disabled={create.isPending}>
          {create.isPending ? t("admin.users.creating") : t("admin.users.create")}
        </Button>
      </div>
    </form>
  );
}

export function NewUserDialog({
  open,
  onOpenChange,
  onCreated,
  onCloseAutoFocus,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onCloseAutoFocus: (event: Event) => void;
  onCreated: (user: User) => void;
}) {
  const { t } = useTranslation();
  return (
    <Dialog open={open} onOpenChange={onOpenChange} title={t("admin.users.new")} onCloseAutoFocus={onCloseAutoFocus}>
      <NewUserForm
        onCancel={() => {
          onOpenChange(false);
        }}
        onCreated={onCreated}
      />
    </Dialog>
  );
}

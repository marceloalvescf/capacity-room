import { zodResolver } from "@hookform/resolvers/zod";
import { useRef, useState, type ReactNode } from "react";
import { useForm } from "react-hook-form";
import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import { z } from "zod";
import { errorText, toError } from "@/api/errors";
import { languageValues, roleValues } from "@/api/schema.gen";
import { useAnnounce } from "@/app/live-region";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { Field } from "@/components/ui/field";
import { Select } from "@/components/ui/select";
import { formatDate } from "@/lib/format";
import { newPassword } from "@/lib/password";
import { usePrincipal } from "@/session/session";
import { useChangeUser, useDeactivateUser, type User } from "./api";

function Part({ children }: { children: ReactNode }) {
  return <section className="flex flex-col gap-3 border-t border-border pt-4">{children}</section>;
}

function RolePart({ user }: { user: User }) {
  const { t } = useTranslation();
  const announce = useAnnounce();
  const change = useChangeUser(user.id);
  const [role, setRole] = useState(user.role);
  const [language, setLanguage] = useState(user.language);
  const [seen, setSeen] = useState({ role: user.role, language: user.language });
  const [notice, setNotice] = useState<string | null>(null);
  const [failure, setFailure] = useState<string | null>(null);
  if (seen.role !== user.role || seen.language !== user.language) {
    // The user changed elsewhere: the selects follow it and a pending edit goes.
    setSeen({ role: user.role, language: user.language });
    setRole(user.role);
    setLanguage(user.language);
  }
  const roleChanged = role !== user.role;
  const languageChanged = language !== user.language;

  function clear() {
    setNotice(null);
    setFailure(null);
  }

  async function save() {
    setNotice(null);
    setFailure(null);
    try {
      await change.mutateAsync({
        ...(roleChanged ? { role } : {}),
        ...(languageChanged ? { language } : {}),
      });
      const saved = t(roleChanged ? "admin.users.savedSignedOut" : "admin.users.saved");
      setNotice(saved);
      announce(saved);
    } catch (thrown) {
      setFailure(errorText(t, toError(thrown)));
    }
  }

  return (
    <Part>
      <div className="grid grid-cols-[repeat(auto-fit,minmax(12rem,1fr))] gap-4">
        <Select
          label={t("admin.users.roleLabel")}
          value={role}
          onChange={(event) => {
            clear();
            setRole(roleValues.find((value) => value === event.target.value) ?? user.role);
          }}
        >
          {roleValues.map((value) => (
            <option key={value} value={value}>
              {t(`admin.users.role.${value}`)}
            </option>
          ))}
        </Select>
        <Select
          label={t("admin.users.languageLabel")}
          value={language}
          onChange={(event) => {
            clear();
            setLanguage(languageValues.find((value) => value === event.target.value) ?? user.language);
          }}
        >
          {languageValues.map((value) => (
            <option key={value} value={value}>
              {t(`language.${value}`, { lng: value })}
            </option>
          ))}
        </Select>
      </div>
      <p className="text-sm text-muted-fg">{t("admin.users.roleHint")}</p>
      {failure === null ? null : <Alert tone="error">{failure}</Alert>}
      {notice === null ? null : <Alert tone="notice">{notice}</Alert>}
      <div>
        <Button
          variant="primary"
          disabled={change.isPending || (!roleChanged && !languageChanged)}
          onClick={() => {
            void save();
          }}
        >
          {change.isPending ? t("admin.users.saving") : t("admin.users.save")}
        </Button>
      </div>
    </Part>
  );
}

const passwordSchema = z.object({ password: newPassword });
type PasswordValues = z.infer<typeof passwordSchema>;

function PasswordPart({ user }: { user: User }) {
  const { t } = useTranslation();
  const announce = useAnnounce();
  const change = useChangeUser(user.id);
  const [notice, setNotice] = useState<string | null>(null);
  const [failure, setFailure] = useState<string | null>(null);
  const {
    register,
    handleSubmit,
    reset,
    formState: { errors },
  } = useForm<PasswordValues>({ resolver: zodResolver(passwordSchema), defaultValues: { password: "" } });

  async function submit(values: PasswordValues) {
    setNotice(null);
    setFailure(null);
    try {
      await change.mutateAsync({ password: values.password });
      reset();
      const done = t("admin.users.password.done");
      setNotice(done);
      announce(done);
    } catch (thrown) {
      setFailure(errorText(t, toError(thrown)));
    }
  }

  const problem = errors.password?.message;
  return (
    <Part>
      <form
        noValidate
        className="flex flex-col gap-3"
        onSubmit={(event) => {
          void handleSubmit(submit)(event);
        }}
      >
        <Field
          label={t("admin.users.password.label")}
          type="password"
          autoComplete="new-password"
          hint={t("admin.users.password.hint")}
          error={problem === undefined ? undefined : t(problem)}
          {...register("password", {
            onChange: () => {
              setNotice(null);
              setFailure(null);
            },
          })}
        />
        {failure === null ? null : <Alert tone="error">{failure}</Alert>}
        {notice === null ? null : <Alert tone="notice">{notice}</Alert>}
        <div>
          <Button type="submit" disabled={change.isPending}>
            {t("admin.users.password.submit")}
          </Button>
        </div>
      </form>
    </Part>
  );
}

function StatePart({ user }: { user: User }) {
  const { t } = useTranslation();
  const announce = useAnnounce();
  const change = useChangeUser(user.id);
  const deactivate = useDeactivateUser(user.id);
  const [asking, setAsking] = useState(false);
  const deactivateButton = useRef<HTMLButtonElement>(null);
  const reactivateButton = useRef<HTMLButtonElement>(null);
  const [failure, setFailure] = useState<string | null>(null);

  async function confirmDeactivate() {
    setFailure(null);
    try {
      await deactivate.mutateAsync();
      announce(t("admin.users.deactivated", { name: user.username }));
    } catch (thrown) {
      setFailure(errorText(t, toError(thrown)));
    }
    setAsking(false);
  }

  async function reactivate() {
    setFailure(null);
    try {
      await change.mutateAsync({ active: true });
      announce(t("admin.users.reactivated", { name: user.username }));
    } catch (thrown) {
      setFailure(errorText(t, toError(thrown)));
    }
  }

  return (
    <Part>
      {failure === null ? null : <Alert tone="error">{failure}</Alert>}
      {user.active ? (
        <>
          <p className="text-sm text-muted-fg">{t("admin.users.deactivateHint")}</p>
          <div>
            <Button
              ref={deactivateButton}
              variant="danger"
              onClick={() => {
                setAsking(true);
              }}
            >
              {t("admin.users.deactivate")}
            </Button>
          </div>
        </>
      ) : (
        <div>
          <Button
            ref={reactivateButton}
            disabled={change.isPending}
            onClick={() => {
              void reactivate();
            }}
          >
            {t("admin.users.reactivate")}
          </Button>
        </div>
      )}
      <ConfirmDialog
        open={asking}
        title={t("admin.users.deactivateTitle", { name: user.username })}
        body={t("admin.users.deactivateBody", { name: user.username })}
        confirm={t("admin.users.deactivateConfirm")}
        cancel={t("admin.users.cancel")}
        busy={deactivate.isPending}
        onConfirm={() => {
          void confirmDeactivate();
        }}
        onCloseAutoFocus={(event) => {
          event.preventDefault();
          (reactivateButton.current ?? deactivateButton.current)?.focus();
        }}
        onOpenChange={setAsking}
      />
    </Part>
  );
}

// The opened row of a user: the date, and for another user the parts that change them.
export function UserRow({ user }: { user: User }) {
  const { t } = useTranslation();
  const principal = usePrincipal();
  const own = principal?.id === user.id;
  return (
    <div className="flex max-w-2xl flex-col gap-4">
      <p className="text-sm">{t("admin.users.created", { date: formatDate(user.created_at) })}</p>
      {own ? (
        <>
          <p className="text-sm">{t("admin.users.own")}</p>
          <p>
            <Link to="/account" className="text-accent underline">
              {t("admin.users.ownLink")}
            </Link>
          </p>
        </>
      ) : (
        <>
          <RolePart user={user} />
          <PasswordPart user={user} />
          <StatePart user={user} />
        </>
      )}
    </div>
  );
}

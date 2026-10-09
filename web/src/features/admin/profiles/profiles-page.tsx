import { useCallback, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import { errorText, toError } from "@/api/errors";
import { useAnnounce } from "@/app/live-region";
import { useScreenTitle } from "@/app/title";
import { DataTable, factGrid, type Column } from "@/components/data-table";
import { Alert } from "@/components/ui/alert";
import { Button, buttonStyles } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { useAddress } from "@/lib/address";
import { formatTime } from "@/lib/format";
import { useAdminProfiles, useDeleteProfile, useTestProfile, type Profile } from "./api";
import { summaries, valuesOf } from "./form";
import { outcomeText, TestResultLine, type TestOutcome } from "./test-result";

const rowKey = (profile: Profile) => profile.id;
const rowName = (profile: Profile) => profile.id;

function Fact({ label, children, wide = false }: { label: string; children: React.ReactNode; wide?: boolean }) {
  return (
    <div className={wide ? "col-span-full min-w-0" : "min-w-0"}>
      <dt className="text-sm text-muted-fg">{label}</dt>
      <dd>{children}</dd>
    </div>
  );
}

function FileName({ name }: { name: string | undefined }) {
  const { t } = useTranslation();
  return name === undefined || name === "" ? (
    <span className="text-muted-fg">{t("admin.profiles.none")}</span>
  ) : (
    <span className="font-mono">{name}</span>
  );
}

// The opened row of a profile: its facts, a test of the saved revision, and the ways to edit or delete it.
function ProfileRow({
  profile,
  outcome,
  onOutcome,
  onDelete,
}: {
  profile: Profile;
  outcome: TestOutcome | null;
  onOutcome: (id: string, outcome: TestOutcome | null) => void;
  onDelete: (profile: Profile, opener: HTMLElement) => void;
}) {
  const { t } = useTranslation();
  const announce = useAnnounce();
  const test = useTestProfile(profile.id);
  const { choices } = summaries(valuesOf(profile), t);

  async function runTest() {
    let found: TestOutcome;
    try {
      found = { kind: "result", result: await test.mutateAsync() };
    } catch (thrown) {
      found = { kind: "failed", error: toError(thrown) };
    }
    onOutcome(profile.id, found);
    announce(outcomeText(t, found));
  }

  return (
    <div className="flex flex-col gap-4">
      <dl className={factGrid}>
        <Fact label={t("admin.profiles.fact.endpoint")} wide>
          <span className="font-mono">{profile.endpoint_url}</span>
        </Fact>
        <Fact label={t("admin.profiles.fact.type")}>{t(`endpointType.${profile.endpoint_type}`)}</Fact>
        <Fact label={t("admin.profiles.fact.credential")}>
          <FileName name={profile.credential_file} />
        </Fact>
        <Fact label={t("admin.profiles.fact.ca")}>
          <FileName name={profile.ca_bundle_file} />
        </Fact>
        <Fact label={t("admin.profiles.fact.revision")}>
          {t("admin.profiles.revisionValue", { revision: profile.revision, time: formatTime(profile.created_at) })}
        </Fact>
        <Fact label={t("admin.profiles.fact.namespaces")} wide>
          <span className="font-mono">{profile.allowed_namespaces.join(", ")}</span>
        </Fact>
        <Fact label={t("admin.profiles.fact.choices")} wide>
          <span className="font-mono">{choices}</span>
        </Fact>
      </dl>
      <div className="flex flex-wrap items-center gap-3">
        <Button
          disabled={test.isPending}
          onClick={() => {
            void runTest();
          }}
        >
          {test.isPending ? t("admin.profiles.testing") : t("admin.profiles.test")}
        </Button>
        <TestResultLine outcome={outcome} />
        <div className="ml-auto flex items-center gap-2">
          <Link to={`/profiles/${profile.id}/edit`} className={buttonStyles("primary")}>
            {t("admin.profiles.edit")}
          </Link>
          <Button
            variant="danger"
            onClick={(event) => {
              onDelete(profile, event.currentTarget);
            }}
          >
            {t("admin.profiles.delete")}
          </Button>
        </div>
      </div>
    </div>
  );
}

export function ProfilesPage() {
  const { t } = useTranslation();
  const profiles = useAdminProfiles();
  const [outcomes, setOutcomes] = useState<ReadonlyMap<string, TestOutcome>>(new Map());
  const [failure, setFailure] = useState<Error | null>(null);
  const [target, setTarget] = useState<Profile | null>(null);
  // The name stays after the dialog closes, so its title does not go blank while it fades.
  const [askedName, setAskedName] = useState("");
  const remove = useDeleteProfile();
  const announce = useAnnounce();
  const address = useAddress();
  const heading = useRef<HTMLHeadingElement>(null);
  const opener = useRef<HTMLElement | null>(null);
  const focusHeading = useRef(false);
  useScreenTitle(t("admin.profiles.title"));

  const onOutcome = useCallback((id: string, outcome: TestOutcome | null) => {
    setOutcomes((earlier) => {
      const next = new Map(earlier);
      if (outcome === null) {
        next.delete(id);
      } else {
        next.set(id, outcome);
      }
      return next;
    });
  }, []);

  async function confirmDelete() {
    if (target === null) {
      return;
    }
    setFailure(null);
    try {
      await remove.mutateAsync(target.id);
      onOutcome(target.id, null);
      address.write((next) => {
        next.delete("open");
      });
      announce(t("admin.profiles.deleted", { name: target.display_name }));
      focusHeading.current = true;
    } catch (thrown) {
      setFailure(toError(thrown));
    }
    setTarget(null);
  }

  const columns = useMemo<Column<Profile>[]>(
    () => [
      {
        id: "profile",
        header: t("admin.profiles.column.profile"),
        cell: (profile) => (
          <>
            <div>{profile.display_name}</div>
            <div className="font-mono text-xs text-muted-fg">{profile.id}</div>
          </>
        ),
      },
      {
        id: "cluster",
        header: t("admin.profiles.column.cluster"),
        cell: (profile) => <span className="font-mono">{profile.cluster_name}</span>,
      },
      {
        id: "state",
        header: t("admin.profiles.column.state"),
        cell: (profile) =>
          profile.active ? (
            t("admin.state.active")
          ) : (
            <span className="text-muted-fg">{t("admin.state.inactive")}</span>
          ),
      },
    ],
    [t],
  );

  let content = null;
  if (profiles.data !== undefined) {
    content = (
      <DataTable
        label={t("admin.profiles.title")}
        rows={profiles.data}
        rowKey={rowKey}
        rowName={rowName}
        columns={columns}
        renderOpened={(profile) => (
          <ProfileRow
            profile={profile}
            outcome={outcomes.get(profile.id) ?? null}
            onOutcome={onOutcome}
            onDelete={(profile, button) => {
              opener.current = button;
              setAskedName(profile.display_name);
              setTarget(profile);
            }}
          />
        )}
        empty={t("admin.profiles.empty")}
        defaultOrder="view"
      />
    );
  } else if (profiles.isError) {
    content = <Alert tone="error">{t("admin.profiles.loadFailed")}</Alert>;
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h1 ref={heading} tabIndex={-1} className="text-xl font-semibold outline-none">
          {t("admin.profiles.title")}
        </h1>
        <Link to="/profiles/new" className={buttonStyles("primary")}>
          {t("admin.profiles.new")}
        </Link>
      </div>
      {failure === null ? null : <Alert tone="error">{errorText(t, failure)}</Alert>}
      {content}
      <ConfirmDialog
        open={target !== null}
        title={t("admin.profiles.deleteTitle", { name: askedName })}
        body={t("admin.profiles.deleteBody")}
        confirm={t("admin.profiles.deleteConfirm")}
        cancel={t("admin.profiles.cancel")}
        busy={remove.isPending}
        onConfirm={() => {
          void confirmDelete();
        }}
        onCloseAutoFocus={(event) => {
          event.preventDefault();
          const back = opener.current;
          if (!focusHeading.current && back?.isConnected === true) {
            back.focus();
          } else {
            heading.current?.focus();
          }
          focusHeading.current = false;
          opener.current = null;
        }}
        onOpenChange={(open) => {
          if (!open) {
            setTarget(null);
          }
        }}
      />
    </div>
  );
}
